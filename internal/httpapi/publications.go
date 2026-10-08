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
	if !allowPublicRequest(w, r) {
		return
	}
	content, err := fs.ReadFile(assets, "site.html")
	if err != nil {
		http.Error(w, "site unavailable", http.StatusNotFound)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || len(parts) == 4 || len(parts) > 5 || parts[0] != "s" {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, version, publication, lookupErr := s.publicApplication(ctx, parts[1])
	if lookupErr != nil {
		http.NotFound(w, r)
		return
	}
	tables, tableErr := s.appTables(ctx, app, stringValue(app["tenant_id"]))
	if tableErr != nil {
		http.NotFound(w, r)
		return
	}
	pages, validationMessage := normalizePublicPages(version, anySlice(publication["pages"]), tables)
	if validationMessage != "" || len(pages) == 0 {
		http.NotFound(w, r)
		return
	}
	pageID := r.URL.Query().Get("page")
	if len(parts) >= 3 {
		pageID = parts[2]
	}
	grant := publicationPage(map[string]any{"pages": pages}, pageID)
	if pageID == "" {
		grant = asMap(pages[0])
		pageID = stringValue(grant["id"])
	}
	if grant == nil {
		http.NotFound(w, r)
		return
	}
	title := defaultString(stringValue(grant["title"]), stringValue(app["name"]))
	description := defaultString(stringValue(app["description"]), title+" · MIAO")
	body, article, renderErr := s.publicHTML(ctx, app, grant, parts)
	if renderErr != nil {
		http.NotFound(w, r)
		return
	}
	if article != nil {
		read := publicRead(grant, "", parts[3])
		data := asMap(article["data"])
		if name := stringValue(read["seo_title_field"]); name != "" {
			title = defaultString(stringValue(data[name]), title)
		}
		if name := stringValue(read["seo_description_field"]); name != "" {
			description = defaultString(stringValue(data[name]), description)
		}
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	canonicalURL := url.URL{Scheme: scheme, Host: r.Host, Path: r.URL.Path}
	if len(parts) == 2 {
		canonicalURL.RawQuery = "page=" + url.QueryEscape(pageID)
	}
	canonical := canonicalURL.String()
	page := string(content)
	for placeholder, value := range map[string]string{"PUBLIC_TITLE": title, "PUBLIC_DESCRIPTION": description, "PUBLIC_CANONICAL": canonical} {
		page = strings.ReplaceAll(page, placeholder, html.EscapeString(value))
	}
	page = strings.Replace(page, "PUBLIC_BODY", body, 1)
	page = strings.Replace(page, "</body>", localize(negotiateLanguage(r), "<noscript>此页面中的内容已由服务器呈现。</noscript></body>"), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(page))
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

func normalizePublicPages(version map[string]any, raw any, tables []map[string]any) ([]any, string) {
	items := anySlice(raw)
	if len(items) == 0 || len(items) > 32 {
		return nil, "请配置 1–32 个公开页面"
	}
	definition, message := validateAppUIDefinition(version["definition"], tables)
	if message != "" {
		return nil, "当前正式界面无效"
	}
	pageByID := map[string]map[string]any{}
	for _, page := range appUIPages(definition) {
		pageByID[stringValue(page["id"])] = page
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
		sources := asSliceMap(publishedPage["data_sources"])
		for _, source := range sources {
			if source["context"] != nil {
				return nil, "关联私有记录上下文的页面不能匿名公开，请配置独立公开页面"
			}
		}
		if len(reads) != len(sources) {
			return nil, "每个公开数据源都必须单独授权"
		}
		readResult, seenSources := []any{}, map[string]bool{}
		for _, rawRead := range reads {
			read := asMap(rawRead)
			tableName := stringValue(read["table"])
			sourceID := stringValue(read["source"])
			var source map[string]any
			for _, candidate := range sources {
				if candidate["id"] == sourceID {
					source = candidate
					break
				}
			}
			if source == nil || seenSources[sourceID] || source["collection"] != tableName {
				return nil, "公开数据源不存在、重复或绑定表不匹配"
			}
			seenSources[sourceID] = true
			var table map[string]any
			for _, candidate := range tables {
				if candidate["slug"] == tableName {
					table = candidate
					break
				}
			}
			fields := uniqueStrings(anySlice(read["fields"]), 24)
			if table == nil || len(fields) == 0 {
				return nil, "公开读取必须引用当前应用数据表并至少选择一个字段"
			}
			allowedFields := map[string]string{}
			for _, field := range asSliceMap(table["fields"]) {
				allowedFields[stringValue(field["name"])] = stringValue(field["type"])
			}
			images := uniqueStrings(anySlice(read["images"]), 4)
			for _, image := range images {
				if allowedFields[image] != "file" || !containsString(fields, image) {
					return nil, "公开图片必须是页面展示的附件字段且显式授权"
				}
			}
			for _, field := range fields {
				if !containsString(stringSlice(anySlice(source["fields"])), field) || !containsString([]string{"text", "number", "bool", "date", "email", "url", "select"}, allowedFields[field]) && !containsString(images, field) {
					return nil, "公开字段必须来自当前数据源并且是普通字段或显式授权图片"
				}
			}
			htmlFields := uniqueStrings(anySlice(read["html_fields"]), 4)
			if len(anySlice(read["html_fields"])) > 4 || read["html_fields"] != nil && !isUIArray(read["html_fields"]) {
				return nil, "正文 HTML 最多授权 4 个文本字段"
			}
			for _, name := range htmlFields {
				if allowedFields[name] != "text" || !containsString(fields, name) {
					return nil, "HTML 正文必须是本页已授权的文本字段"
				}
				if name == read["status_field"] || name == read["slug_field"] || name == read["seo_title_field"] || name == read["seo_description_field"] {
					return nil, "状态、Slug 和 SEO 字段必须保持纯文本"
				}
			}
			policy := map[string]any{"source": sourceID, "table": tableName, "fields": fields, "images": images, "html_fields": htmlFields}
			// Query fields have to be explicitly public too, including sort.
			query := asMap(source["query"])
			for _, filter := range asSliceMap(query["filters"]) {
				if !containsString(fields, stringValue(filter["field"])) {
					return nil, "公开筛选只能使用本数据源授权的字段"
				}
			}
			sortField := strings.TrimPrefix(stringValue(query["sort"]), "-")
			if sortField != "" && sortField != "created" && sortField != "updated" && !containsString(fields, sortField) {
				return nil, "公开排序只能使用本数据源授权的字段"
			}
			if query != nil {
				policy["query"] = query
			}
			statusField, slugField := stringValue(read["status_field"]), stringValue(read["slug_field"])
			for _, key := range []string{"seo_title_field", "seo_description_field"} {
				if name := stringValue(read[key]); name != "" {
					if !containsString(fields, name) || allowedFields[name] != "text" {
						return nil, "SEO 字段必须是本页已公开的文本字段"
					}
					policy[key] = name
				}
			}
			if statusField == "" || slugField == "" || !containsString([]string{"text", "select"}, allowedFields[statusField]) || allowedFields[slugField] != "text" || !containsString(fields, slugField) || strings.TrimSpace(stringValue(read["published_value"])) == "" {
				return nil, "公开内容需要状态字段、已发布值和公开文本 slug 字段"
			}
			if field := findField(asSliceMap(table["fields"]), statusField); field != nil && field["type"] == "select" && !contains(field["options"], read["published_value"]) {
				return nil, "已发布值不在状态选项中"
			}
			policy["status_field"], policy["published_value"], policy["slug_field"] = statusField, read["published_value"], slugField
			readResult = append(readResult, policy)
		}
		if len(readResult) != len(sources) {
			return nil, "公开数据源不完整"
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

// publicVisit records one anonymous open of a published public page. Like the
// workspace visit counter it is advisory: failures only degrade the statistic.
func (s *Server) publicVisit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !allowPublicRequest(w, r) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, _, err := s.publicApplication(ctx, r.PathValue("slug"))
	if err != nil {
		writeError(w, 404, "公开页面不存在或已关闭")
		return
	}
	if err := s.PB.Increment(ctx, "apps", stringValue(app["id"]), "view_count"); err != nil {
		s.Logger.Error("application visit counter failed", "error", err)
	}
	writeJSON(w, 200, map[string]any{"ok": true})
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
	definition, message := validateAppUIDefinition(version["definition"], tables)
	if message != "" {
		writeError(w, 404, "公开页面暂不可用")
		return
	}
	var publishedPage map[string]any
	for _, candidate := range appUIPages(definition) {
		if candidate["id"] == pageID {
			publishedPage = candidate
			break
		}
	}
	if publishedPage == nil {
		writeError(w, 404, "公开页面不存在")
		return
	}
	pageNumber := queryInt(r, "page_number", 1, 1, 100000)
	perPage := queryInt(r, "perPage", 20, 1, 50)
	sources := map[string]any{}
	for _, raw := range anySlice(grant["reads"]) {
		read := asMap(raw)
		sourceID, tableName := stringValue(read["source"]), stringValue(read["table"])
		var table map[string]any
		for _, candidate := range tables {
			if candidate["slug"] == tableName {
				table = candidate
				break
			}
		}
		if table == nil {
			writeError(w, 404, "公开数据源不存在")
			return
		}
		filter := []string{"tenant_id = " + pbFilterString(stringValue(app["tenant_id"])), "app_id = " + pbFilterString(stringValue(app["id"]))}
		filter = append(filter, uiQueryFilter(read)...)
		if statusField := stringValue(read["status_field"]); statusField != "" {
			filter = append(filter, statusField+" = "+pbFilterString(stringValue(read["published_value"])))
		}
		search := clip(strings.TrimSpace(r.URL.Query().Get("search")), 120)
		if search != "" {
			var searchable []string
			for _, field := range asSliceMap(table["fields"]) {
				if containsString(stringSlice(anySlice(read["fields"])), stringValue(field["name"])) && containsString([]string{"text", "email", "url"}, stringValue(field["type"])) {
					searchable = append(searchable, stringValue(field["name"])+" ~ "+pbFilterString(search))
				}
			}
			if len(searchable) > 0 {
				filter = append(filter, "("+strings.Join(searchable, " || ")+")")
			}
		}
		rows, total, totalPages, err := s.PB.List(ctx, stringValue(table["pb_collection"]), listFilter(filter...), uiQuerySort(read), pageNumber, perPage)
		if err != nil {
			writeError(w, 503, "公开数据暂不可用")
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, publicRecordFields(r.PathValue("slug"), pageID, tableName, row, read))
		}
		searchSupported := false
		for _, field := range asSliceMap(table["fields"]) {
			if containsString(stringSlice(anySlice(read["fields"])), stringValue(field["name"])) && containsString([]string{"text", "email", "url"}, stringValue(field["type"])) {
				searchSupported = true
				break
			}
		}
		sources[sourceID] = map[string]any{"id": sourceID, "collection": tableName, "fields": publicDisplayFields(asSliceMap(table["fields"]), read), "sanitized_markup": true, "items": items, "total_items": total, "total_pages": totalPages, "page": pageNumber, "per_page": perPage, "search": search, "search_supported": searchSupported, "actions": []any{}, "create_form_available": false, "create_form_fields": []any{}}
	}
	publicPage := map[string]any{"id": pageID, "title": publishedPage["title"], "spec": publishedPage["spec"], "data_sources": []any{}}
	for _, raw := range anySlice(grant["reads"]) {
		read := asMap(raw)
		publicPage["data_sources"] = append(publicPage["data_sources"].([]any), map[string]any{"id": read["source"], "collection": read["table"], "fields": read["fields"]})
	}
	writeJSON(w, 200, map[string]any{"status": "published", "read_only": true, "title": grant["title"], "app_title": app["name"], "page_title": grant["title"], "description": app["description"], "ui_page": pageID, "public_page": pageID, "pages": publicPageSummaries(validatedPages), "definition": map[string]any{"schema_version": 3, "title": definition["title"], "pages": []any{publicPage}}, "sources": sources, "version": map[string]any{"version": version["version"]}})

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
	read := publicRead(grant, tableName, r.URL.Query().Get("source"))
	if read == nil {
		writeError(w, 404, "公开数据不存在")
		return
	}
	allowed := stringSlice(anySlice(read["fields"]))
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
	filter = append(filter, uiQueryFilter(read)...)
	if statusField := stringValue(read["status_field"]); statusField != "" {
		filter = append(filter, statusField+" = "+pbFilterString(stringValue(read["published_value"])))
	}
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
	rows, total, _, err := s.PB.List(ctx, stringValue(table["pb_collection"]), listFilter(filter...), uiQuerySort(read), pageNumber, perPage)
	if err != nil {
		writeError(w, 503, "公开数据暂不可用")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, publicRecordFields(r.PathValue("slug"), pageID, tableName, row, read))
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

func publicRead(page map[string]any, tableName, source string) map[string]any {
	if page == nil {
		return nil
	}
	for _, raw := range anySlice(page["reads"]) {
		read := asMap(raw)
		if source != "" && read["source"] == source && (tableName == "" || read["table"] == tableName) {
			return read
		}
	}
	return nil
}

func publicRecordFields(siteSlug, pageID, tableName string, row, read map[string]any) map[string]any {
	data := map[string]any{}
	for _, name := range stringSlice(anySlice(read["fields"])) {
		value := row[name]
		if containsString(stringSlice(anySlice(read["images"])), name) {
			value = nil
			if stringValue(row[name]) != "" {
				value = "/api/public/" + url.PathEscape(siteSlug) + "/images/" + url.PathEscape(pageID) + "/" + url.PathEscape(stringValue(read["source"])) + "/" + url.PathEscape(tableName) + "/" + url.PathEscape(stringValue(row["id"])) + "/" + url.PathEscape(name)
			}
		}
		if containsString(stringSlice(anySlice(read["html_fields"])), name) {
			value = sanitizePublicMarkup(stringValue(value))
		}
		data[name] = value
	}
	result := map[string]any{"id": row["id"], "data": data}
	if slugField := stringValue(read["slug_field"]); slugField != "" {
		result["url"] = "/s/" + url.PathEscape(siteSlug) + "/" + url.PathEscape(pageID) + "/" + url.PathEscape(stringValue(read["source"])) + "/" + url.PathEscape(stringValue(row[slugField]))
	}
	return result
}

func (s *Server) publicRecordContext(ctx context.Context, r *http.Request, pageID, tableName, source string) (map[string]any, map[string]any, map[string]any, error) {
	app, version, publication, err := s.publicApplication(ctx, r.PathValue("slug"))
	if err != nil {
		return nil, nil, nil, err
	}
	tables, err := s.appTables(ctx, app, stringValue(app["tenant_id"]))
	if err != nil {
		return nil, nil, nil, err
	}
	pages, message := normalizePublicPages(version, publication["pages"], tables)
	if message != "" {
		return nil, nil, nil, fmt.Errorf("publication invalid: %s", message)
	}
	read := publicRead(publicationPage(map[string]any{"pages": pages}, pageID), tableName, source)
	if read == nil {
		return nil, nil, nil, fmt.Errorf("read not published")
	}
	for _, table := range tables {
		if table["slug"] == tableName {
			return app, table, read, nil
		}
	}
	return nil, nil, nil, fmt.Errorf("table missing")
}

func (s *Server) publicRecordDetail(w http.ResponseWriter, r *http.Request) {
	if !allowPublicRequest(w, r) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	pageID, tableName := r.URL.Query().Get("page_id"), r.URL.Query().Get("table")
	app, table, read, err := s.publicRecordContext(ctx, r, pageID, tableName, r.URL.Query().Get("source"))
	if err != nil || stringValue(read["slug_field"]) == "" || !publicationSlugPattern.MatchString(r.PathValue("itemSlug")) {
		writeError(w, 404, "公开内容不存在")
		return
	}
	parts := []string{"tenant_id = " + pbFilterString(stringValue(app["tenant_id"])), "app_id = " + pbFilterString(stringValue(app["id"])), stringValue(read["status_field"]) + " = " + pbFilterString(stringValue(read["published_value"])), stringValue(read["slug_field"]) + " = " + pbFilterString(r.PathValue("itemSlug"))}
	filter := listFilter(append(parts, uiQueryFilter(read)...)...)
	rows, _, _, err := s.PB.List(ctx, stringValue(table["pb_collection"]), filter, "", 1, 2)
	if err != nil || len(rows) != 1 {
		writeError(w, 404, "公开内容不存在")
		return
	}
	result := publicRecordFields(r.PathValue("slug"), pageID, tableName, rows[0], read)
	data := asMap(result["data"])
	result["seo"] = map[string]any{"title": data[stringValue(read["seo_title_field"])], "description": data[stringValue(read["seo_description_field"])]}
	writeJSON(w, 200, result)
}

func (s *Server) publicImage(w http.ResponseWriter, r *http.Request) {
	if !allowPublicRequest(w, r) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	pageID, tableName, fieldName := r.PathValue("pageId"), r.PathValue("table"), r.PathValue("field")
	app, table, read, err := s.publicRecordContext(ctx, r, pageID, tableName, r.PathValue("source"))
	if err != nil || !containsString(stringSlice(anySlice(read["images"])), fieldName) || stringValue(read["status_field"]) == "" {
		writeError(w, 404, "公开图片不存在")
		return
	}
	parts := []string{"id = " + pbFilterString(r.PathValue("recordId")), "tenant_id = " + pbFilterString(stringValue(app["tenant_id"])), "app_id = " + pbFilterString(stringValue(app["id"])), stringValue(read["status_field"]) + " = " + pbFilterString(stringValue(read["published_value"]))}
	rows, _, _, err := s.PB.List(ctx, stringValue(table["pb_collection"]), listFilter(append(parts, uiQueryFilter(read)...)...), "", 1, 1)
	var row map[string]any
	if len(rows) == 1 {
		row = rows[0]
	}
	if err != nil || row == nil || stringValue(row[fieldName]) == "" {
		writeError(w, 404, "公开图片不存在")
		return
	}
	data, contentType, _, err := s.PB.ProtectedFile(ctx, stringValue(table["pb_collection"]), stringValue(row["id"]), stringValue(row[fieldName]))
	if err != nil || !containsString([]string{"image/png", "image/jpeg", "image/webp", "image/gif"}, contentType) {
		writeError(w, 404, "公开图片不存在")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// publicHTML renders only publication-granted fields, image proxies and
// explicitly opted-in sanitized prose. Page/record code is never executed.
func (s *Server) publicHTML(ctx context.Context, app, page map[string]any, pathParts []string) (string, map[string]any, error) {
	pageID := stringValue(page["id"])
	var body strings.Builder
	body.WriteString("<section id=\"public-content\"><h1>")
	body.WriteString(html.EscapeString(stringValue(page["title"])))
	body.WriteString("</h1>")
	for _, raw := range anySlice(page["reads"]) {
		read := asMap(raw)
		table, err := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(app["tenant_id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(stringValue(read["table"]))))
		if err != nil {
			return "", nil, err
		}
		filter := []string{"tenant_id = " + pbFilterString(stringValue(app["tenant_id"])), "app_id = " + pbFilterString(stringValue(app["id"]))}
		filter = append(filter, uiQueryFilter(read)...)
		if statusField := stringValue(read["status_field"]); statusField != "" {
			filter = append(filter, statusField+" = "+pbFilterString(stringValue(read["published_value"])))
		}
		if len(pathParts) == 5 {
			if stringValue(read["source"]) != pathParts[3] {
				continue
			}
			if stringValue(read["slug_field"]) == "" || !publicationSlugPattern.MatchString(pathParts[4]) {
				return "", nil, fmt.Errorf("detail unavailable")
			}
			filter = append(filter, stringValue(read["slug_field"])+" = "+pbFilterString(pathParts[4]))
		}
		limit := 50
		if len(pathParts) == 5 {
			limit = 2
		}
		rows, _, _, err := s.PB.List(ctx, stringValue(table["pb_collection"]), listFilter(filter...), uiQuerySort(read), 1, limit)
		if err != nil {
			return "", nil, err
		}
		if len(pathParts) == 5 && len(rows) != 1 {
			return "", nil, fmt.Errorf("detail unavailable")
		}
		for _, row := range rows {
			visible := publicRecordFields(stringValue(app["public_slug"]), pageID, stringValue(read["table"]), row, read)
			body.WriteString("<article>")
			if link := stringValue(visible["url"]); link != "" && len(pathParts) != 5 {
				body.WriteString("<p><a href=\"")
				body.WriteString(html.EscapeString(link))
				body.WriteString("\">查看内容</a></p>")
			}

			for _, field := range stringSlice(anySlice(read["fields"])) {
				value := stringValue(asMap(visible["data"])[field])
				if containsString(stringSlice(anySlice(read["images"])), field) {
					if value != "" {
						body.WriteString("<img loading=\"lazy\" alt=\"\" src=\"")
						body.WriteString(html.EscapeString(value))
						body.WriteString("\">")
					}
				} else if containsString(stringSlice(anySlice(read["html_fields"])), field) && value != "" {
					body.WriteString("<div class=\"jr-rich-text\">")
					body.WriteString(value)
					body.WriteString("</div>")
				} else if value != "" {
					body.WriteString("<p>")
					body.WriteString(html.EscapeString(value))
					body.WriteString("</p>")
				}
			}

			body.WriteString("</article>")
			if len(pathParts) == 5 {
				body.WriteString("</section>")
				return body.String(), visible, nil
			}
		}
	}
	body.WriteString("</section>")
	if len(pathParts) == 5 {
		return "", nil, fmt.Errorf("detail unavailable")
	}
	return body.String(), nil, nil
}
