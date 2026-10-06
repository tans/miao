package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/tans/miao/internal/pocketbase"
)

// These types are the trusted boundary between HTTP/tools/workers and storage.
// Dynamic business fields stay maps; identity and authorization do not.
type appRole string

func (r appRole) canWrite() bool {
	return r == "owner" || r == "publisher" || r == "manager" || r == "editor"
}
func (r appRole) canManage() bool  { return r == "owner" || r == "publisher" || r == "manager" }
func (r appRole) canPublish() bool { return r == "owner" || r == "publisher" }

type appAccess struct {
	Role  appRole
	Batch bool
}
type executionActor struct {
	UserID, TenantID, AppID, Source string
}

func (id identity) actor(appID, source string) executionActor {
	return executionActor{stringValue(id.User["id"]), stringValue(id.Tenant["id"]), appID, source}
}
func businessError(status int, message string) error {
	return &pocketbase.Error{Status: status, Message: message}
}
func isMissing(err error) bool {
	var pe *pocketbase.Error
	return errors.As(err, &pe) && pe.Status == http.StatusNotFound
}

func applicationAccess(ctx context.Context, pb *pocketbase.Client, app map[string]any, id identity) (appAccess, error) {
	if app["tenant_id"] != id.Tenant["id"] {
		return appAccess{}, nil
	}
	if id.Membership["role"] == "owner" && id.Tenant["owner_id"] == id.User["id"] {
		return appAccess{Role: "owner", Batch: true}, nil
	}
	permission, err := pb.Find(ctx, "app_members", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "user_id = "+pbFilterString(stringValue(id.User["id"]))))
	if isMissing(err) {
		if !boolValue(app["restricted"]) {
			return appAccess{Role: "editor"}, nil
		}
		return appAccess{}, nil
	}
	if err != nil {
		return appAccess{}, err
	}
	role := appRole(stringValue(permission["role"]))
	return appAccess{Role: role, Batch: role.canWrite() && boolValue(permission["can_batch"])}, nil
}

// Re-read the actor at the write boundary. A task or an earlier browser request
// must not retain permission after membership/role/account changes.
func (s *Server) authorizeWrite(ctx context.Context, pb *pocketbase.Client, actor executionActor, batch bool) (map[string]any, error) {
	user, err := pb.Get(ctx, "users", actor.UserID)
	if err != nil {
		return nil, err
	}
	tenant, err := pb.Get(ctx, "tenants", actor.TenantID)
	if err != nil {
		return nil, err
	}
	membership, err := pb.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "user_id = "+pbFilterString(actor.UserID)))
	if isMissing(err) {
		return nil, businessError(403, "你已不再是此工作区成员")
	}
	if err != nil {
		return nil, err
	}
	app, err := pb.Get(ctx, "apps", actor.AppID)
	if err != nil {
		return nil, err
	}
	access, err := applicationAccess(ctx, pb, app, identity{User: user, Tenant: tenant, Membership: membership})
	if err != nil {
		return nil, err
	}
	if boolValue(user["disabled"]) || s.RequireVerification && !boolValue(user["verified"]) || boolValue(app["archived"]) || !access.Role.canWrite() || batch && !access.Batch || actor.Source == "background" && !access.Role.canPublish() {
		return nil, businessError(403, "没有此应用的业务修改权限")
	}
	return app, nil
}

type recordWrite struct {
	Actor                                              executionActor
	Table, RecordID, ExpectedUpdated, PublishedVersion string
	Data                                               map[string]any
	Files                                              []pocketbase.Upload
	Batch                                              bool
	// Background callers pass the authorized fields for this specific action.
	AllowedFields  []string
	FieldsSnapshot []map[string]any
}

// saveBusinessRecord is shared by forms, Agent tools, import/batch, runtime
// actions, attachments, restoration and workers. commit saves a local receipt
// in the same transaction as the record, audit and task events.
func (s *Server) saveBusinessRecord(ctx context.Context, cmd recordWrite, commit func(*pocketbase.Client, map[string]any) error, guards ...func(*pocketbase.Client) error) (map[string]any, error) {
	var before, saved, table map[string]any
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		app, err := s.authorizeWrite(ctx, tx, cmd.Actor, cmd.Batch)
		if err != nil {
			return err
		}
		if cmd.PublishedVersion != "" && stringValue(app["published_version_id"]) != cmd.PublishedVersion {
			return businessError(409, "正式界面已变化，请刷新后操作")
		}
		for _, guard := range guards {
			if err := guard(tx); err != nil {
				return err
			}
		}
		table, err = tx.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(cmd.Actor.TenantID), "app_id = "+pbFilterString(cmd.Actor.AppID), "slug = "+pbFilterString(cmd.Table)))
		if err != nil {
			return err
		}
		fields := asSliceMap(table["fields"])
		if cmd.FieldsSnapshot != nil && !equalJSON(fields, cmd.FieldsSnapshot) {
			return businessError(409, "表结构已变化，请重新预览")
		}
		if cmd.Actor.Source == "background" {
			if cmd.RecordID == "" || len(cmd.Files) > 0 {
				return businessError(403, "后台任务只允许修改获授权的普通字段")
			}
			for name := range cmd.Data {
				field := findField(fields, name)
				if !containsString(cmd.AllowedFields, name) || field == nil || field["type"] == "file" || field["type"] == "relation" {
					return businessError(403, "字段超出此次动作授权")
				}
			}
		}
		for _, file := range cmd.Files {
			field := findField(fields, file.Name)
			if field == nil || field["type"] != "file" {
				return businessError(400, "附件字段无效")
			}
		}
		if msg := validateDataWithFiles(cmd.Data, fields, cmd.RecordID != "", uploadedFileFields(cmd.Files)); msg != "" {
			return businessError(400, msg)
		}
		if msg := validateRelations(ctx, tx, cmd.Data, fields, cmd.Actor.AppID, cmd.Actor.TenantID); msg != "" {
			return businessError(400, msg)
		}
		if msg := validateMemberReferences(ctx, tx, cmd.Data, fields, app, cmd.Actor.TenantID); msg != "" {
			return businessError(400, msg)
		}
		if cmd.RecordID != "" {
			before, err = tx.Get(ctx, stringValue(table["pb_collection"]), cmd.RecordID)
			if err != nil {
				return err
			}
			if before["tenant_id"] != cmd.Actor.TenantID || before["app_id"] != cmd.Actor.AppID {
				return businessError(404, "记录不存在")
			}
			if cmd.ExpectedUpdated == "" || stringValue(before["updated"]) != cmd.ExpectedUpdated {
				return businessError(409, "记录已变化，请重新读取后操作")
			}
		}
		values := mergeData(nil, cmd.Data)
		if cmd.RecordID == "" {
			values["tenant_id"], values["app_id"] = cmd.Actor.TenantID, cmd.Actor.AppID
		}
		saved, err = tx.UploadBusiness(ctx, stringValue(table["pb_collection"]), cmd.RecordID, values, cmd.Files, cmd.ExpectedUpdated, cmd.Actor.UserID, cmd.Actor.Source)
		if err != nil {
			return err
		}
		if commit != nil {
			return commit(tx, saved)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if cmd.Actor.Source != "background" {
		event := "updated"
		if cmd.RecordID == "" {
			event = "created"
		}
		s.processRecordAutomation(ctx, cmd.Actor.TenantID, cmd.Actor.AppID, cmd.Table, event, before, saved)
	}
	return saved, nil
}

func (s *Server) writeBusinessError(w http.ResponseWriter, err error) {
	var pe *pocketbase.Error
	if errors.As(err, &pe) {
		writeError(w, pe.Status, pe.Message)
		return
	}
	s.Logger.Error("business operation failed", "error", err)
	writeError(w, 503, "操作暂时未能保存，请稍后检查并重试")
}

type memberGrant struct {
	UserID   string  `json:"user_id"`
	Role     appRole `json:"role"`
	CanBatch bool    `json:"can_batch"`
}
type appAccessRequest struct {
	Restricted  *bool         `json:"restricted"`
	Permissions []memberGrant `json:"permissions"`
}

func (s *Server) replaceAppAccess(ctx context.Context, actor executionActor, input appAccessRequest) error {
	return s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		app, err := s.authorizeWrite(ctx, tx, actor, false)
		if err != nil {
			return err
		}
		tenant, err := tx.Get(ctx, "tenants", actor.TenantID)
		if err != nil {
			return err
		}
		if stringValue(tenant["owner_id"]) != actor.UserID {
			return businessError(403, "只有工作区所有者可以管理访问权限")
		}
		members, err := tx.ListAll(ctx, "tenant_members", "tenant_id = "+pbFilterString(actor.TenantID), "")
		if err != nil {
			return err
		}
		valid := map[string]bool{}
		for _, m := range members {
			if m["role"] != "owner" {
				valid[stringValue(m["user_id"])] = true
			}
		}
		seen := map[string]bool{}
		for _, grant := range input.Permissions {
			if !valid[grant.UserID] || seen[grant.UserID] || grant.Role != "viewer" && grant.Role != "editor" && grant.Role != "manager" && grant.Role != "publisher" {
				return businessError(400, "权限成员或角色无效或重复")
			}
			seen[grant.UserID] = true
		}
		current, err := tx.ListAll(ctx, "app_members", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "app_id = "+pbFilterString(actor.AppID)), "")
		if err != nil {
			return err
		}
		for _, p := range current {
			if err := tx.Delete(ctx, "app_members", stringValue(p["id"])); err != nil {
				return err
			}
		}
		for _, grant := range input.Permissions {
			if _, err := tx.Create(ctx, "app_members", map[string]any{"tenant_id": actor.TenantID, "app_id": actor.AppID, "user_id": grant.UserID, "role": string(grant.Role), "can_batch": grant.CanBatch && grant.Role.canWrite()}); err != nil {
				return err
			}
		}
		_, err = tx.Update(ctx, "apps", stringValue(app["id"]), map[string]any{"restricted": *input.Restricted})
		return err
	})
}

func (s *Server) deleteBusinessRecord(ctx context.Context, actor executionActor, slug, recordID string) error {
	return s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		if _, err := s.authorizeWrite(ctx, tx, actor, false); err != nil {
			return err
		}
		table, err := tx.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "app_id = "+pbFilterString(actor.AppID), "slug = "+pbFilterString(slug)))
		if err != nil {
			return err
		}
		row, err := tx.Get(ctx, stringValue(table["pb_collection"]), recordID)
		if err != nil {
			return err
		}
		if row["tenant_id"] != actor.TenantID || row["app_id"] != actor.AppID {
			return businessError(404, "记录不存在")
		}
		return tx.Delete(ctx, stringValue(table["pb_collection"]), recordID)
	})
}
