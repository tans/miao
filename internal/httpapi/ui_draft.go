package httpapi

import (
	uidoc "github.com/tans/miao/internal/ui"
	"context"
	"fmt"
	"strings"

	"github.com/tans/miao/internal/harness"
	"github.com/tans/miao/internal/pocketbase"
)

// Agent-driven UI draft creation and publication. These are called from the
// harness flow rather than HTTP handlers; the runtime action endpoint and the
// version/preview handlers stay in versions.go.

func (s *Server) createUIDraft(ctx context.Context, actor executionActor, input map[string]any, stepID string) (map[string]any, error) {
	var created map[string]any
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		id, app, _, err := s.buildActor(ctx, tx, actor)
		if err != nil {
			return err
		}
		// A durable receipt wins over a changed baseline when recovering a write.
		if stepID != "" {
			prior, err := tx.Find(ctx, "app_versions", "harness_step_id = "+pbFilterString(stepID))
			if err == nil {
				if prior["tenant_id"] != actor.TenantID || prior["app_id"] != actor.AppID || prior["created_by"] != actor.UserID {
					return businessError(403, "草稿回执不属于当前运行")
				}
				created = prior
				return nil
			}
			if !isMissing(err) {
				return err
			}
		}
		tables, err := tx.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "app_id = "+pbFilterString(actor.AppID)), "created")
		if err != nil {
			return err
		}
		definition, msg := uidoc.ValidateDefinition(input["definition"], tables)
		if msg != "" {
			return businessError(400, msg)
		}
		if msg = validateUIActionReferences(ctx, tx, app, definition, false); msg != "" {
			return businessError(400, msg)
		}
		latest, _, _, err := tx.List(ctx, "app_versions", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "app_id = "+pbFilterString(actor.AppID)), "-version", 1, 1)
		if err != nil {
			return err
		}
		latestID, number := "", 1
		if len(latest) > 0 {
			latestID = stringValue(latest[0]["id"])
			number = intValue(latest[0]["version"]) + 1
		}
		if expected, exists := input["expected_latest_version_id"]; exists {
			if expected != nil {
				if _, valid := expected.(string); !valid {
					return businessError(400, "版本基线必须为字符串或 null")
				}
			}
			if stringValue(expected) != latestID {
				return businessError(409, "已有新界面草稿，请重新载入后审阅；本次编辑和原草稿均未覆盖")
			}
		}
		basedID := stringValue(input["based_on_version_id"])
		if basedID != "" {
			base, err := tx.Get(ctx, "app_versions", basedID)
			if err != nil || base["tenant_id"] != actor.TenantID || base["app_id"] != actor.AppID {
				return businessError(404, "修订来源版本不存在")
			}
			published := stringValue(base["published_at"]) != ""
			current := basedID == stringValue(app["published_version_id"])
			if published && !current {
				return businessError(409, "正式版本已变化，请重新载入；原草稿已保留")
			}
		}
		if expected, exists := input["expected_published_version_id"]; exists && stringValue(expected) != stringValue(app["published_version_id"]) {
			return businessError(409, "正式界面已变化，请重新载入后审阅")
		}
		source := cloneAnyMap(asMap(input["source"]))
		if source == nil {
			source = map[string]any{}
		}
		source["submitted_by"] = map[string]any{"id": id.User["id"], "name": id.User["name"], "email": id.User["email"]}
		created, err = tx.Create(ctx, "app_versions", map[string]any{"tenant_id": actor.TenantID, "app_id": actor.AppID, "version": number, "summary": clip(strings.TrimSpace(stringValue(input["summary"])), 1000), "based_on_version_id": basedID, "created_by": id.User["id"], "definition": definition, "source": source, "harness_step_id": stepID})
		return err
	})
	return created, err
}

// publishHarnessDraft publishes the reviewed draft referenced by input and
// optionally applies the anonymous publication profile in one transaction.
// The dialogue chain's ui.publish candidates are the only caller since the
// manual publish entry was removed with the visual editor.
func (s *Server) publishHarnessDraft(ctx context.Context, actor executionActor, input map[string]any) (map[string]any, error) {
	versionID := stringValue(input["version_id"])
	if versionID == "" || actor.AppID == "" || actor.TenantID == "" {
		return nil, harness.ErrCapability
	}
	expected := ""
	if raw, exists := input["expected_published_version_id"]; exists && raw != nil {
		text, ok := raw.(string)
		if !ok {
			return nil, businessError(400, "发布基线必须为字符串或 null")
		}
		expected = text
	}
	unlock := lockAppVersion(actor.AppID)
	defer unlock()
	app, err := s.PB.Get(ctx, "apps", actor.AppID)
	if err != nil || app["tenant_id"] != actor.TenantID {
		return nil, businessError(404, "应用不存在或你没有访问权限")
	}
	if boolValue(app["archived"]) {
		return nil, businessError(409, "应用已归档，无法发布")
	}
	currentID := stringValue(app["published_version_id"])
	if expected != currentID {
		return nil, businessError(409, "正式界面已变化，请刷新后重新发布")
	}
	version, err := s.PB.Get(ctx, "app_versions", versionID)
	if err != nil || version["tenant_id"] != actor.TenantID || version["app_id"] != actor.AppID {
		return nil, businessError(404, "草稿版本不存在")
	}
	if currentID == versionID {
		runtime, err := s.runtimeForVersion(ctx, app, actor.TenantID, version, map[string]string{}, 0)
		if err != nil {
			return nil, err
		}
		runtime["version"] = uidoc.Public(version, versionID)
		return runtime, nil
	}
	if stringValue(version["published_at"]) != "" {
		return nil, businessError(409, "此版本已经发布或已被替代；如需回退，请基于当前版本创建新草稿")
	}
	if currentID != "" {
		current, e := s.PB.Get(ctx, "app_versions", currentID)
		if e != nil {
			return nil, e
		}
		if intValue(version["version"]) <= intValue(current["version"]) {
			return nil, businessError(409, "不能发布早于或等于当前版本的草稿；如需回退，请基于当前版本创建新草稿")
		}
	}
	tables, err := s.appTables(ctx, app, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if _, msg := uidoc.ValidateDefinition(version["definition"], tables); msg != "" {
		return nil, businessError(400, "草稿无法发布："+msg)
	}
	if msg := s.validateBusinessActionReferences(ctx, app, asMap(version["definition"]), true); msg != "" {
		return nil, businessError(400, "草稿无法发布："+msg)
	}
	previewApp := map[string]any{}
	for k, v := range app {
		previewApp[k] = v
	}
	previewApp["published_version_id"] = versionID
	runtime, err := s.runtimeForVersion(ctx, previewApp, actor.TenantID, version, map[string]string{}, 0)
	if err != nil || runtime["status"] != "published" {
		return nil, businessError(409, "草稿运行检查失败，当前发布版未更改")
	}
	publication := asMap(input["publication"])
	profile := map[string]any{}
	if publication["enabled"] == true {
		slug := strings.TrimSpace(strings.ToLower(stringValue(publication["slug"])))
		if len(slug) < 3 || len(slug) > 64 || !publicationSlugPattern.MatchString(slug) {
			return nil, businessError(400, "公开链接标识需为 3–64 位小写字母、数字或连字符")
		}
		pages, message := normalizePublicPages(version, publication["pages"], tables)
		if message != "" {
			return nil, businessError(400, message)
		}
		if existing, findErr := s.PB.Find(ctx, "apps", "public_slug = "+pbFilterString(slug)); findErr == nil && existing["id"] != actor.AppID {
			return nil, businessError(409, "公开链接标识已被使用")
		}
		profile = map[string]any{"enabled": true, "slug": slug, "pages": pages}
	}
	err = s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		fresh, err := tx.Get(ctx, "apps", actor.AppID)
		if err != nil {
			return err
		}
		if stringValue(fresh["published_version_id"]) != currentID {
			return businessError(409, "正式界面已变化，请刷新后发布")
		}
		if _, err := s.authorizeWrite(ctx, tx, actor, false); err != nil {
			return err
		}
		tenant, err := tx.Get(ctx, "tenants", actor.TenantID)
		if err != nil {
			return err
		}
		user, err := tx.Get(ctx, "users", actor.UserID)
		if err != nil {
			return err
		}
		member, err := tx.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "user_id = "+pbFilterString(actor.UserID)))
		if err != nil {
			return err
		}
		access, err := applicationAccess(ctx, tx, fresh, identity{User: user, Tenant: tenant, Membership: member})
		if err != nil {
			return err
		}
		if !access.Role.canPublish() {
			return businessError(403, "没有发布权限")
		}
		txTables, err := tx.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "app_id = "+pbFilterString(actor.AppID)), "created")
		if err != nil {
			return err
		}
		if _, msg := uidoc.ValidateDefinition(version["definition"], txTables); msg != "" {
			return businessError(409, msg)
		}
		if msg := validateUIActionReferences(ctx, tx, fresh, asMap(version["definition"]), true); msg != "" {
			return businessError(409, msg)
		}
		if _, err := tx.Update(ctx, "app_versions", versionID, map[string]any{"published_at": nowISO()}); err != nil {
			return err
		}
		updates := map[string]any{"published_version_id": versionID}
		if publication["enabled"] == true {
			updates["public_slug"] = stringValue(profile["slug"])
			updates["public_publication"] = profile
		}
		_, err = tx.Update(ctx, "apps", actor.AppID, updates)
		return err
	})
	if err != nil {
		return nil, err
	}
	runtime["version"] = uidoc.Public(version, versionID)
	receipt := map[string]any{"status": "published", "app_id": actor.AppID, "version": versionID, "version_number": intValue(version["version"]), "published": true, "publication": "private", "runtime": runtime, "message": fmt.Sprintf("正式界面已发布 v%d。", intValue(version["version"]))}
	if publication["enabled"] == true {
		receipt["publication"] = "public"
		receipt["public_slug"] = stringValue(profile["slug"])
		receipt["message"] = fmt.Sprintf("正式界面已发布 v%d，匿名公开地址 /s/%s。", intValue(version["version"]), stringValue(profile["slug"]))
	}
	return receipt, nil
}
