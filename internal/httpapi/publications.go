package httpapi

import (
	"context"
	"fmt"
	"html"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var publicationSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var publicRateMu sync.Mutex
var publicRateBuckets = map[string]publicRateBucket{}

type publicRateBucket struct {
	window time.Time
	count  int
}

func allowPublicRequest(w http.ResponseWriter, r *http.Request) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if parsed := net.ParseIP(ip); parsed != nil && parsed.IsLoopback() {
		if proxyIP := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); proxyIP != nil {
			ip = proxyIP.String()
		}
	}
	now := time.Now()
	publicRateMu.Lock()
	bucket := publicRateBuckets[ip]
	if now.Sub(bucket.window) >= time.Minute {
		bucket = publicRateBucket{window: now}
	}
	bucket.count++
	publicRateBuckets[ip] = bucket
	if len(publicRateBuckets) > 4096 {
		for key, candidate := range publicRateBuckets {
			if now.Sub(candidate.window) >= 2*time.Minute {
				delete(publicRateBuckets, key)
			}
		}
	}
	publicRateMu.Unlock()
	if bucket.count > 120 {
		w.Header().Set("Retry-After", "60")
		w.Header().Set("Cache-Control", "no-store")
		writeError(w, http.StatusTooManyRequests, "公开页面请求过于频繁，请稍后重试")
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	return true
}

func (s *Server) servePublicSite(w http.ResponseWriter, r *http.Request, assets fs.FS) {
	content, err := fs.ReadFile(assets, "site.html")
	if err != nil {
		http.Error(w, "site unavailable", http.StatusNotFound)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	slug := ""
	if len(parts) == 2 && parts[0] == "s" {
		slug = parts[1]
	}
	title, description, pageID := "公开页面", "公开应用页面", r.URL.Query().Get("page")
	if slug != "" {
		ctx, cancel := contextTimeout(r)
		defer cancel()
		app, version, publication, lookupErr := s.publicApplication(ctx, slug)
		if lookupErr == nil {
			tables, tableErr := s.appTables(ctx, app, stringValue(app["tenant_id"]))
			pages, validationMessage := normalizePublicPages(version, anySlice(publication["pages"]), tables)
			if tableErr == nil && validationMessage == "" {
				grant := publicationPage(map[string]any{"pages": pages}, pageID)
				if grant == nil && len(pages) > 0 {
					grant = asMap(pages[0])
					pageID = stringValue(grant["id"])
				}
				if grant != nil {
					title = defaultString(stringValue(grant["title"]), stringValue(app["name"]))
					description = defaultString(stringValue(app["description"]), title+" · MIAO")
				}
			}
		}
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	query := url.Values{}
	if pageID != "" {
		query.Set("page", pageID)
	}
	canonical := (&url.URL{Scheme: scheme, Host: r.Host, Path: r.URL.Path, RawQuery: query.Encode()}).String()
	page := string(content)
	for placeholder, value := range map[string]string{"PUBLIC_TITLE": title, "PUBLIC_DESCRIPTION": description, "PUBLIC_CANONICAL": canonical} {
		page = strings.ReplaceAll(page, placeholder, html.EscapeString(value))
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(page))
}

func (s *Server) routesPublications() {
	s.Mux.HandleFunc("GET /api/apps/{id}/publication", s.auth(s.getPublication))
	s.Mux.HandleFunc("PUT /api/apps/{id}/publication", s.auth(s.updatePublication))
	s.Mux.HandleFunc("GET /api/public/{slug}/runtime", s.publicRuntime)
	s.Mux.HandleFunc("GET /api/public/{slug}/records", s.publicRecords)
}

func (s *Server) getPublication(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canManageAppRole(role) {
		writeError(w, 403, "你没有管理此应用的权限")
		return
	}
	writeJSON(w, 200, publicPublication(app))
}

func publicPublication(app map[string]any) map[string]any {
	publication := asMap(app["public_publication"])
	if publication == nil {
		publication = map[string]any{"enabled": false, "slug": "", "pages": []any{}}
	}
	slug := stringValue(publication["slug"])
	return map[string]any{"enabled": boolValue(publication["enabled"]), "slug": slug, "url": func() string {
		if !boolValue(publication["enabled"]) || slug == "" {
			return ""
		}
		return "/s/" + slug
	}(), "pages": anySlice(publication["pages"])}
}

func (s *Server) updatePublication(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canPublishAppRole(role) {
		writeError(w, 403, "你没有管理此应用的权限")
		return
	}
	input := mapBody(r)
	if input["confirm"] != true {
		writeError(w, 403, "公开访问会让访客无需登录即可读取所选数据，请明确确认")
		return
	}
	enabled := input["enabled"] == true
	current := asMap(app["public_publication"])
	profile := map[string]any{"enabled": enabled, "pages": []any{}}
	slug := ""
	if current != nil {
		slug = stringValue(current["slug"])
	}
	if inputSlug := strings.TrimSpace(strings.ToLower(stringValue(input["slug"]))); inputSlug != "" {
		slug = inputSlug
	}
	if enabled {
		if len(slug) < 3 || len(slug) > 64 || !publicationSlugPattern.MatchString(slug) {
			writeError(w, 400, "公开链接标识需为 3–64 位小写字母、数字或连字符")
			return
		}
		versionID := stringValue(app["published_version_id"])
		if versionID == "" {
			writeError(w, 409, "请先发布应用界面版本")
			return
		}
		version, versionErr := s.PB.Get(ctx, "app_versions", versionID)
		if versionErr != nil || version["tenant_id"] != app["tenant_id"] || version["app_id"] != app["id"] {
			writeError(w, 409, "当前正式版本不可用")
			return
		}
		tables, tableErr := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
		if tableErr != nil {
			s.writeBusinessError(w, tableErr)
			return
		}
		pages, message := normalizePublicPages(version, input["pages"], tables)
		if message != "" {
			writeError(w, 400, message)
			return
		}
		profile["pages"] = pages
		if existing, findErr := s.PB.Find(ctx, "apps", "public_slug = "+pbFilterString(slug)); findErr == nil && existing["id"] != app["id"] {
			writeError(w, 409, "公开链接标识已被使用")
			return
		}
	} else if current != nil {
		profile["pages"] = anySlice(current["pages"])
	}
	profile["slug"] = slug
	activeSlug := slug
	if !enabled {
		activeSlug = ""
	}
	saved, err := s.PB.Update(ctx, "apps", stringValue(app["id"]), map[string]any{"public_slug": activeSlug, "public_publication": profile})
	if err != nil {
		s.Logger.Error("application publication update failed", "error", err)
		writeError(w, 409, "公开访问配置未保存；链接标识可能已被使用")
		return
	}
	writeJSON(w, 200, publicPublication(saved))
}

func normalizePublicPages(version map[string]any, raw any, tables []map[string]any) ([]any, string) {
	items := anySlice(raw)
	if len(items) == 0 || len(items) > 32 {
		return nil, "请配置 1–32 个公开页面"
	}
	pageByID := map[string]map[string]any{}
	if isSourceVersion(version) {
		_, manifest, capabilities, message := validateSourceVersion(version["source"], version["manifest"], version["capabilities"])
		if message != "" || !contains(capabilities, "records.read") {
			return nil, "源码正式版本必须有效并声明 records.read 能力"
		}
		for _, route := range anySlice(manifest["routes"]) {
			item := asMap(route)
			pageByID[stringValue(item["path"])] = item
		}
	} else {
		definition, message := validateAppUIDefinition(version["definition"], tables)
		if message != "" {
			return nil, "当前正式界面无效：" + message
		}
		for _, page := range appUIPages(definition) {
			pageByID[stringValue(page["id"])] = page
		}
	}
	result, seenPages := []any{}, map[string]bool{}
	for _, rawPage := range items {
		page := asMap(rawPage)
		id := strings.TrimSpace(stringValue(page["id"]))
		publishedPage, exists := pageByID[id]
		if !exists || seenPages[id] {
			return nil, "公开页面必须唯一引用当前正式版本中的页面或路由"
		}
		seenPages[id] = true
		reads := anySlice(page["reads"])
		if len(reads) == 0 || len(reads) > 12 {
			return nil, "每个公开页面需要授权 1–12 组数据表和字段"
		}
		allowedPageFields := map[string]bool{}
		if !isSourceVersion(version) {
			for _, field := range anySlice(publishedPage["fields"]) {
				allowedPageFields[stringValue(field)] = true
			}
		}
		readResult, seenTables := []any{}, map[string]bool{}
		for _, rawRead := range reads {
			read := asMap(rawRead)
			tableName := stringValue(read["table"])
			var table map[string]any
			for _, candidate := range tables {
				if candidate["slug"] == tableName {
					table = candidate
					break
				}
			}
			fields := uniqueStrings(anySlice(read["fields"]), 24)
			if table == nil || seenTables[tableName] || len(fields) == 0 {
				return nil, "公开读取必须引用当前应用数据表并至少选择一个字段"
			}
			seenTables[tableName] = true
			allowedFields := map[string]bool{}
			for _, field := range asSliceMap(table["fields"]) {
				if containsString([]string{"text", "number", "bool", "date", "email", "url", "select"}, stringValue(field["type"])) {
					allowedFields[stringValue(field["name"])] = true
				}
			}
			for _, field := range fields {
				if !allowedFields[field] || !isSourceVersion(version) && (tableName != stringValue(publishedPage["collection"]) || !allowedPageFields[field]) {
					return nil, "公开字段必须是当前页面展示且不含附件、关联或内部字段的普通字段"
				}
			}
			readResult = append(readResult, map[string]any{"table": tableName, "fields": fields})
		}
		if !isSourceVersion(version) && (len(readResult) != 1 || stringValue(asMap(readResult[0])["table"]) != stringValue(publishedPage["collection"])) {
			return nil, "schema 页面只能读取其绑定的数据表"
		}
		title := strings.TrimSpace(stringValue(page["title"]))
		if title == "" {
			title = strings.TrimSpace(stringValue(publishedPage["title"]))
		}
		if title == "" {
			title = strings.Trim(strings.ReplaceAll(id, "/", " "), " ")
		}
		result = append(result, map[string]any{"id": id, "title": clip(title, 120), "reads": readResult})
	}
	return result, ""
}

func (s *Server) publicApplication(ctx context.Context, slug string) (map[string]any, map[string]any, map[string]any, error) {
	if !publicationSlugPattern.MatchString(slug) {
		return nil, nil, nil, fmt.Errorf("publication unavailable")
	}
	app, err := s.PB.Find(ctx, "apps", "public_slug = "+pbFilterString(slug))
	if err != nil || boolValue(app["archived"]) {
		return nil, nil, nil, fmt.Errorf("publication unavailable")
	}
	publication := asMap(app["public_publication"])
	versionID := stringValue(app["published_version_id"])
	if !boolValue(publication["enabled"]) || versionID == "" {
		return nil, nil, nil, fmt.Errorf("publication unavailable")
	}
	version, err := s.PB.Get(ctx, "app_versions", versionID)
	if err != nil || version["tenant_id"] != app["tenant_id"] || version["app_id"] != app["id"] {
		return nil, nil, nil, fmt.Errorf("publication unavailable")
	}
	return app, version, publication, nil
}

func publicationPage(publication map[string]any, id string) map[string]any {
	for _, raw := range anySlice(publication["pages"]) {
		page := asMap(raw)
		if stringValue(page["id"]) == id {
			return page
		}
	}
	return nil
}

func (s *Server) publicRuntime(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !allowPublicRequest(w, r) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, version, publication, err := s.publicApplication(ctx, r.PathValue("slug"))
	if err != nil {
		writeError(w, 404, "公开页面不存在或已关闭")
		return
	}
	tables, tableErr := s.appTables(ctx, app, stringValue(app["tenant_id"]))
	if tableErr != nil {
		writeError(w, 404, "公开页面暂不可用")
		return
	}
	validatedPages, pageError := normalizePublicPages(version, anySlice(publication["pages"]), tables)
	if pageError != "" {
		writeError(w, 404, "当前正式版本的公开范围需重新确认")
		return
	}
	pageID := r.URL.Query().Get("page")
	if pageID == "" && len(validatedPages) > 0 {
		pageID = stringValue(asMap(validatedPages[0])["id"])
	}
	grant := publicationPage(map[string]any{"pages": validatedPages}, pageID)
	if grant == nil {
		writeError(w, 404, "公开页面不存在")
		return
	}
	if isSourceVersion(version) {
		files, manifest, _, message := validateSourceVersion(version["source"], version["manifest"], version["capabilities"])
		if message != "" {
			writeError(w, 404, "公开页面暂不可用")
			return
		}
		var route map[string]any
		for _, raw := range anySlice(manifest["routes"]) {
			candidate := asMap(raw)
			if candidate["path"] == pageID {
				route = candidate
				break
			}
		}
		if route == nil {
			writeError(w, 404, "公开页面不存在")
			return
		}
		entry := stringValue(route["file"])
		publicFiles := map[string]string{"entry.html": files[entry]}
		if files["styles.css"] != "" {
			publicFiles["styles.css"] = files["styles.css"]
		}
		if files["app.js"] != "" {
			publicFiles["app.js"] = files["app.js"]
		}
		writeJSON(w, 200, map[string]any{"status": "published", "title": app["name"], "app_title": app["name"], "page_title": grant["title"], "description": app["description"], "page": pageID, "pages": publicPageSummaries(validatedPages), "version": version["version"], "source": publicFiles, "manifest": map[string]any{"entry": "entry.html", "routes": []any{map[string]any{"path": pageID, "file": "entry.html"}}}, "capabilities": []any{"records.read"}, "reads": anySlice(grant["reads"])})
		return
	}
	definition, message := validateAppUIDefinition(version["definition"], tables)
	if message != "" {
		writeError(w, 404, "公开页面暂不可用")
		return
	}
	page, err := s.runtimeForVersion(ctx, app, stringValue(app["tenant_id"]), version, map[string]string{"ui_page": pageID, "page": r.URL.Query().Get("page_number"), "perPage": r.URL.Query().Get("perPage"), "search": r.URL.Query().Get("search")}, 20)
	if err != nil || page["status"] != "published" {
		writeError(w, 404, "公开页面暂不可用")
		return
	}
	read := asMap(anySlice(grant["reads"])[0])
	allowedFields := stringSlice(anySlice(read["fields"]))
	page["fields"] = filterPublicFields(asSliceMap(page["fields"]), allowedFields)
	page["items"] = filterPublicItems(asSliceMap(page["items"]), allowedFields)
	page["actions"], page["create_form_fields"], page["create_form_available"], page["relation_labels"] = []any{}, []any{}, false, map[string]any{}
	page["pages"] = publicPageSummaries(validatedPages)
	page["app_title"] = app["name"]
	page["page_title"] = grant["title"]
	page["description"] = app["description"]
	page["version"] = map[string]any{"version": version["version"]}
	page["public_page"] = pageID
	_ = definition
	writeJSON(w, 200, page)
}

func publicPageSummaries(pages []any) []map[string]any {
	result := make([]map[string]any, 0, len(pages))
	for _, raw := range pages {
		page := asMap(raw)
		result = append(result, map[string]any{"id": page["id"], "title": page["title"]})
	}
	return result
}

func filterPublicFields(fields []map[string]any, allowed []string) []map[string]any {
	result := []map[string]any{}
	for _, field := range fields {
		if containsString(allowed, stringValue(field["name"])) {
			result = append(result, field)
		}
	}
	return result
}

func filterPublicItems(items []map[string]any, allowed []string) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		data := map[string]any{}
		for _, name := range allowed {
			if value, ok := asMap(item["data"])[name]; ok {
				data[name] = value
			}
		}
		result = append(result, map[string]any{"data": data})
	}
	return result
}

func (s *Server) publicRecords(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !allowPublicRequest(w, r) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, version, publication, err := s.publicApplication(ctx, r.PathValue("slug"))
	if err != nil {
		writeError(w, 404, "公开页面不存在或已关闭")
		return
	}
	pageID := r.URL.Query().Get("page_id")
	tableName := r.URL.Query().Get("table")
	tables, err := s.appTables(ctx, app, stringValue(app["tenant_id"]))
	if err != nil {
		writeError(w, 404, "公开数据不存在")
		return
	}
	validatedPages, validationMessage := normalizePublicPages(version, anySlice(publication["pages"]), tables)
	if validationMessage != "" {
		writeError(w, 404, "当前正式版本的公开范围需重新确认")
		return
	}
	grant := publicationPage(map[string]any{"pages": validatedPages}, pageID)
	var allowed []string
	if grant != nil {
		for _, raw := range anySlice(grant["reads"]) {
			read := asMap(raw)
			if read["table"] == tableName {
				allowed = stringSlice(anySlice(read["fields"]))
			}
		}
	}
	if len(allowed) == 0 {
		writeError(w, 404, "公开数据不存在")
		return
	}
	var table map[string]any
	for _, candidate := range tables {
		if candidate["slug"] == tableName {
			table = candidate
		}
	}
	if table == nil {
		writeError(w, 404, "公开数据不存在")
		return
	}
	pageNumber := queryInt(r, "page", 1, 1, 100000)
	perPage := queryInt(r, "perPage", 20, 1, 50)
	filter := []string{"tenant_id = " + pbFilterString(stringValue(app["tenant_id"])), "app_id = " + pbFilterString(stringValue(app["id"]))}
	search := clip(strings.TrimSpace(r.URL.Query().Get("search")), 120)
	if search != "" {
		var searchable []string
		for _, field := range asSliceMap(table["fields"]) {
			if containsString(allowed, stringValue(field["name"])) && containsString([]string{"text", "email", "url"}, stringValue(field["type"])) {
				searchable = append(searchable, stringValue(field["name"])+" ~ "+pbFilterString(search))
			}
		}
		if len(searchable) > 0 {
			filter = append(filter, "("+strings.Join(searchable, " || ")+")")
		}
	}
	rows, total, _, err := s.PB.List(ctx, stringValue(table["pb_collection"]), listFilter(filter...), "-created", pageNumber, perPage)
	if err != nil {
		writeError(w, 503, "公开数据暂不可用")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data := map[string]any{}
		for _, field := range allowed {
			data[field] = row[field]
		}
		items = append(items, map[string]any{"data": data})
	}
	writeJSON(w, 200, pageResult(items, pageNumber, perPage, total))
}

func stringSlice(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, stringValue(value))
	}
	return result
}
