package httpapi

import (
	"context"
	"net/http"
	"net/url"
)

// App access control and record file serving: the member/access endpoints of
// the application domain, split from the app/collection/record handlers.

func (s *Server) listAppMemberChoices(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	tenant, err := s.PB.Get(ctx, "tenants", stringValue(who(r).Tenant["id"]))
	if err != nil {
		writeError(w, 503, "成员暂不可用")
		return
	}
	members, err := s.PB.ListAll(ctx, "tenant_members", "tenant_id = "+pbFilterString(stringValue(tenant["id"])), "created")
	if err != nil {
		writeError(w, 503, "成员暂不可用")
		return
	}
	items, err := s.appMemberChoices(ctx, app, tenant, members)
	if err != nil {
		writeError(w, 503, "成员暂不可用")
		return
	}
	writeJSON(w, 200, map[string]any{"members": items})
}

func (s *Server) appMemberChoices(ctx context.Context, app, tenant map[string]any, members []map[string]any) ([]map[string]any, error) {
	items := []map[string]any{}
	for _, member := range members {
		user, err := s.PB.Get(ctx, "users", stringValue(member["user_id"]))
		if err != nil || boolValue(user["disabled"]) {
			continue
		}
		identity := identity{User: user, Tenant: tenant, Membership: member}
		if s.appPermission(ctx, app, identity) == "" {
			continue
		}
		label := defaultString(stringValue(user["name"]), stringValue(user["email"]))
		items = append(items, map[string]any{"id": user["id"], "name": label, "label": label})
	}
	return items, nil
}

func (s *Server) getAppAccess(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireOwner(w, id) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, err := s.PB.Get(ctx, "apps", pathID(r, "id"))
	if err != nil || app["tenant_id"] != id.Tenant["id"] {
		writeError(w, 404, "应用不存在")
		return
	}
	members, err := s.PB.ListAll(ctx, "tenant_members", "tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "")
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	permissions, err := s.PB.ListAll(ctx, "app_members", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "")
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	assigned := map[string]map[string]any{}
	for _, item := range permissions {
		assigned[stringValue(item["user_id"])] = item
	}
	out := []map[string]any{}
	for _, membership := range members {
		if membership["role"] == "owner" {
			continue
		}
		user, e := s.PB.Get(ctx, "users", stringValue(membership["user_id"]))
		if e != nil {
			continue
		}
		p := assigned[stringValue(user["id"])]
		item := map[string]any{"id": user["id"], "email": user["email"], "name": user["name"], "workspace_role": membership["role"], "app_role": "", "can_batch": false}
		if p != nil {
			item["app_role"], item["can_batch"] = p["role"], p["can_batch"]
		}
		out = append(out, item)
	}
	writeJSON(w, 200, map[string]any{"restricted": boolValue(app["restricted"]), "members": out})
}
func (s *Server) updateAppAccess(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireOwner(w, id) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, err := s.PB.Get(ctx, "apps", pathID(r, "id"))
	if err != nil || app["tenant_id"] != id.Tenant["id"] {
		writeError(w, 404, "应用不存在")
		return
	}
	var input appAccessRequest
	if err := readJSON(r, &input); err != nil || input.Restricted == nil || input.Permissions == nil || len(input.Permissions) > 500 {
		writeError(w, 400, "访问权限设置无效")
		return
	}
	err = s.replaceAppAccess(ctx, id.actor(stringValue(app["id"]), "interactive"), input)
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) recordFile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, table, ok := s.recordsContext(ctx, r)
	if !ok {
		writeError(w, 404, "附件不存在")
		return
	}
	field := findField(asSliceMap(table["fields"]), pathID(r, "fieldName"))
	if field == nil || field["type"] != "file" {
		writeError(w, 404, "附件不存在")
		return
	}
	row, err := s.PB.Get(ctx, stringValue(table["pb_collection"]), pathID(r, "recordId"))
	id := who(r)
	if err != nil || row["app_id"] != app["id"] || row["tenant_id"] != id.Tenant["id"] || stringValue(row[stringValue(field["name"])]) == "" {
		writeError(w, 404, "附件不存在")
		return
	}
	data, contentType, filename, err := s.PB.ProtectedFile(ctx, stringValue(table["pb_collection"]), stringValue(row["id"]), stringValue(row[stringValue(field["name"])]))
	if err != nil {
		writeError(w, 502, "附件暂时无法读取")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline; filename*=UTF-8''"+urlQueryEscape(filename))
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
func urlQueryEscape(value string) string { return url.PathEscape(value) }
