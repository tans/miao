package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
)

const connectorMaxBytes = 2 << 20
const connectorMaxItems = 100
const connectorMaxExtractedBytes = 256 << 10

var connectorSelectorPattern = regexp.MustCompile(`^(\*|[a-zA-Z][a-zA-Z0-9_-]*([.#][a-zA-Z][a-zA-Z0-9_-]*)?|#[a-zA-Z][a-zA-Z0-9_-]*|\.[a-zA-Z][a-zA-Z0-9_-]*|\[[a-zA-Z][a-zA-Z0-9_-]*(=["']?[a-zA-Z0-9:_./ -]+["']?)?\])$`)

func (s *Server) routesConnectors() {
	s.Mux.HandleFunc("GET /api/apps/{id}/connectors", s.auth(s.listConnectors))
	s.Mux.HandleFunc("POST /api/apps/{id}/connectors", s.auth(s.createConnector))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/connectors/{connectorId}", s.auth(s.updateConnector))
	s.Mux.HandleFunc("POST /api/apps/{id}/connectors/{connectorId}/enable", s.auth(s.enableConnector))
	s.Mux.HandleFunc("POST /api/apps/{id}/connectors/{connectorId}/fetch", s.auth(s.fetchConnector))
}

func publicConnector(row map[string]any) map[string]any {
	return map[string]any{"id": row["id"], "name": row["name"], "description": row["description"], "definition": row["definition"], "status": row["status"], "revision": row["revision"], "pause_reason": defaultString(stringValue(row["pause_reason"]), "")}
}

func (s *Server) connectorForRequest(ctx context.Context, r *http.Request, manage bool) (map[string]any, map[string]any, error) {
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		return nil, nil, &pocketbaseError{404, "应用不存在或你没有访问权限"}
	}
	if manage && !canManageAppRole(s.appPermission(ctx, app, who(r))) {
		return nil, nil, &pocketbaseError{403, "你没有管理此应用的权限"}
	}
	connector, err := s.PB.Get(ctx, "connectors", pathID(r, "connectorId"))
	if err != nil || connector["tenant_id"] != who(r).Tenant["id"] || connector["app_id"] != app["id"] {
		return nil, nil, &pocketbaseError{404, "连接器不存在"}
	}
	return app, connector, nil
}

func (s *Server) listConnectors(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	rows, err := s.PB.ListAll(ctx, "connectors", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "status != \"archived\""), "-updated")
	if err != nil {
		writeError(w, 503, "连接器暂不可用")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, publicConnector(row))
	}
	writeJSON(w, 200, out)
}

func (s *Server) createConnector(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canManageAppRole(s.appPermission(ctx, app, who(r))) {
		writeError(w, 403, "你没有管理此应用的权限")
		return
	}
	input := mapBody(r)
	definition, msg := normalizeConnectorDefinition(input["definition"])
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	name := strings.TrimSpace(stringValue(input["name"]))
	if name == "" {
		writeError(w, 400, "请输入连接器名称")
		return
	}
	row, err := s.PB.Create(ctx, "connectors", map[string]any{"tenant_id": who(r).Tenant["id"], "app_id": app["id"], "created_by": who(r).User["id"], "name": clip(name, 160), "description": clip(stringValue(input["description"]), 1000), "definition": definition, "status": "draft", "revision": 1})
	if err != nil {
		writeError(w, 503, "连接器创建失败")
		return
	}
	writeJSON(w, 201, publicConnector(row))
}

func (s *Server) updateConnector(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	_, connector, err := s.connectorForRequest(ctx, r, true)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	input := mapBody(r)
	if expected := intValue(input["expected_revision"]); expected != 0 && expected != intValue(connector["revision"]) {
		writeError(w, 409, "连接器版本已变化，请重新读取后修改")
		return
	}
	definition, msg := normalizeConnectorDefinition(input["definition"])
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	updates := map[string]any{"definition": definition, "revision": intValue(connector["revision"]) + 1, "status": "draft", "pause_reason": ""}
	if _, ok := input["name"]; ok {
		updates["name"] = clip(strings.TrimSpace(stringValue(input["name"])), 160)
	}
	if _, ok := input["description"]; ok {
		updates["description"] = clip(stringValue(input["description"]), 1000)
	}
	saved, err := s.PB.Update(ctx, "connectors", stringValue(connector["id"]), updates)
	if err != nil {
		writeError(w, 503, "连接器更新失败")
		return
	}
	writeJSON(w, 200, publicConnector(saved))
}

func (s *Server) enableConnector(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	_, connector, err := s.connectorForRequest(ctx, r, true)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	input := mapBody(r)
	if input["confirm"] != true {
		writeError(w, 400, "需要确认启用连接器")
		return
	}
	if expected := intValue(input["expected_revision"]); expected != 0 && expected != intValue(connector["revision"]) {
		writeError(w, 409, "连接器版本已变化，请重新读取后操作")
		return
	}
	status := "enabled"
	if input["enabled"] == false {
		status = "paused"
	}
	saved, err := s.PB.Update(ctx, "connectors", stringValue(connector["id"]), map[string]any{"status": status, "pause_reason": ""})
	if err != nil {
		writeError(w, 503, "连接器状态更新失败")
		return
	}
	writeJSON(w, 200, publicConnector(saved))
}

func normalizeConnectorDefinition(raw any) (map[string]any, string) {
	definition := asMap(raw)
	if len(definition) == 0 {
		return nil, "连接器定义必须是对象"
	}
	base, err := url.Parse(strings.TrimSpace(stringValue(definition["base_url"])))
	if err != nil || base.Scheme != "https" || base.Hostname() == "" || base.User != nil || base.Fragment != "" || base.RawQuery != "" || base.Opaque != "" {
		return nil, "连接器只支持不带凭据和片段的 HTTPS base_url"
	}
	if base.Port() != "" && base.Port() != "443" {
		return nil, "连接器只允许 HTTPS 默认端口"
	}
	if base.Path != "" && base.Path != "/" {
		return nil, "base_url 只能包含协议和主机，路径请放入 allowed_paths"
	}
	paths := []string{"/"}
	if rawPaths := anySlice(definition["allowed_paths"]); len(rawPaths) > 0 {
		paths = nil
		for _, rawPath := range rawPaths {
			path := stringValue(rawPath)
			if path == "" || !strings.HasPrefix(path, "/") || strings.Contains(path, "..") || strings.ContainsAny(path, "?#") {
				return nil, "allowed_paths 必须是安全的绝对路径前缀"
			}
			paths = append(paths, path)
		}
	}
	maxBytes := intValue(definition["max_bytes"])
	if maxBytes == 0 {
		maxBytes = connectorMaxBytes
	}
	if maxBytes < 1024 || maxBytes > connectorMaxBytes {
		return nil, "max_bytes 必须在 1024 到 2097152 之间"
	}
	result := map[string]any{"type": "https_fetch", "base_url": strings.TrimRight(base.String(), "/"), "allowed_paths": uniqueStrings(paths, 0), "max_bytes": maxBytes}
	if rawExtract, ok := definition["extract"]; ok && rawExtract != nil {
		extract, msg := normalizeConnectorExtract(rawExtract)
		if msg != "" {
			return nil, msg
		}
		result["extract"] = extract
	}
	return result, ""
}

func normalizeConnectorExtract(raw any) (map[string]any, string) {
	extract := asMap(raw)
	format := stringValue(extract["format"])
	fields := asMap(extract["fields"])
	if len(fields) == 0 || len(fields) > 32 {
		return nil, "extract.fields 需要 1–32 个字段映射"
	}
	for name := range fields {
		if !validInputName(name) {
			return nil, "extract.fields 字段名必须使用小写英文、数字和下划线"
		}
	}
	result := map[string]any{"format": format, "fields": fields}
	switch format {
	case "json":
		itemsPath := stringValue(extract["items_path"])
		if itemsPath != "" && !validJSONPointer(itemsPath) {
			return nil, "JSON items_path 和字段映射必须使用 JSON Pointer"
		}
		for _, rawPath := range fields {
			path, ok := rawPath.(string)
			if !ok || !validJSONPointer(path) {
				return nil, "JSON 字段映射必须使用 JSON Pointer"
			}
		}
		result["items_path"] = itemsPath
	case "html":
		itemSelector := stringValue(extract["item_selector"])
		if !validConnectorSelector(itemSelector) {
			return nil, "HTML item_selector 暂只支持单个标签、#id、.class 或 [attribute] 选择器"
		}
		result["item_selector"] = itemSelector
		normalizedFields := map[string]any{}
		for name, rawField := range fields {
			field := asMap(rawField)
			selector := stringValue(field["selector"])
			attribute := stringValue(field["attribute"])
			if !validConnectorSelector(selector) || attribute != "" && !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`).MatchString(attribute) {
				return nil, "HTML 字段映射需要有效 selector 和可选 attribute"
			}
			normalizedFields[name] = map[string]any{"selector": selector, "attribute": attribute}
		}
		result["fields"] = normalizedFields
	default:
		return nil, "extract.format 必须是 json 或 html"
	}
	return result, ""
}

func validJSONPointer(value string) bool { return value == "" || strings.HasPrefix(value, "/") }

func validConnectorSelector(value string) bool { return connectorSelectorPattern.MatchString(value) }

func connectorURL(definition map[string]any, rawPath string) (*url.URL, error) {
	base, err := url.Parse(stringValue(definition["base_url"]))
	if err != nil {
		return nil, err
	}
	if rawPath == "" {
		rawPath = "/"
	}
	if !strings.HasPrefix(rawPath, "/") || strings.Contains(rawPath, "..") {
		return nil, fmt.Errorf("请求路径无效")
	}
	u, err := url.Parse(rawPath)
	if err != nil || u.Host != "" || u.Scheme != "" {
		return nil, fmt.Errorf("请求路径无效")
	}
	base.Path = strings.TrimRight(base.Path, "/") + u.Path
	base.RawQuery = u.RawQuery
	base.Fragment = ""
	return base, nil
}

func connectorPathAllowed(definition map[string]any, path string) bool {
	for _, raw := range anySlice(definition["allowed_paths"]) {
		prefix := stringValue(raw)
		if prefix == "/" || path == prefix || strings.HasPrefix(path, strings.TrimRight(prefix, "/")+"/") {
			return true
		}
	}
	return false
}

func publicNetworkIPs(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return nil, fmt.Errorf("连接器目标地址不是公网地址")
		}
		return []net.IP{ip}, nil
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("连接器目标域名无法解析")
	}
	for _, ip := range ips {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return nil, fmt.Errorf("连接器目标域名解析到非公网地址")
		}
	}
	return ips, nil
}

func publicNetworkHost(ctx context.Context, host string) error {
	_, err := publicNetworkIPs(ctx, host)
	return err
}

func jsonPointer(value any, pointer string) (any, bool) {
	current := value
	if pointer == "" {
		return current, true
	}
	for _, raw := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		part := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
		switch node := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = node[part]
			if !ok {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(node) {
				return nil, false
			}
			current = node[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func scalarConnectorValue(value any) (any, bool) {
	switch value.(type) {
	case nil, string, bool, float64:
		if text, ok := value.(string); ok && len(text) > 4096 {
			return text[:4096], true
		}
		return value, true
	default:
		return nil, false
	}
}

func extractJSONRecords(body []byte, extract map[string]any) ([]map[string]any, error) {
	var document any
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, fmt.Errorf("响应不是有效 JSON")
	}
	items, ok := jsonPointer(document, stringValue(extract["items_path"]))
	if !ok {
		return nil, fmt.Errorf("JSON items_path 未匹配到内容")
	}
	list, ok := items.([]any)
	if !ok {
		list = []any{items}
	}
	fields := asMap(extract["fields"])
	rows := make([]map[string]any, 0, min(len(list), connectorMaxItems+1))
	for _, item := range list[:min(len(list), connectorMaxItems+1)] {
		row := map[string]any{}
		for name, rawPointer := range fields {
			value, found := jsonPointer(item, stringValue(rawPointer))
			if !found {
				continue
			}
			if scalar, valid := scalarConnectorValue(value); valid {
				row[name] = scalar
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func connectorSelectorMatches(node *xhtml.Node, selector string) bool {
	if node.Type != xhtml.ElementNode {
		return false
	}
	if selector == "*" {
		return true
	}
	if strings.Contains(selector, ".") && !strings.HasPrefix(selector, ".") {
		parts := strings.SplitN(selector, ".", 2)
		return strings.EqualFold(node.Data, parts[0]) && connectorSelectorMatches(node, "."+parts[1])
	}
	if strings.Contains(selector, "#") && !strings.HasPrefix(selector, "#") {
		parts := strings.SplitN(selector, "#", 2)
		return strings.EqualFold(node.Data, parts[0]) && connectorSelectorMatches(node, "#"+parts[1])
	}
	if strings.HasPrefix(selector, "#") || strings.HasPrefix(selector, ".") {
		want := selector[1:]
		for _, attr := range node.Attr {
			if selector[0] == '#' && attr.Key == "id" && attr.Val == want {
				return true
			}
			if selector[0] == '.' && attr.Key == "class" && containsString(strings.Fields(attr.Val), want) {
				return true
			}
		}
		return false
	}
	if strings.HasPrefix(selector, "[") {
		inner := strings.TrimSuffix(strings.TrimPrefix(selector, "["), "]")
		parts := strings.SplitN(inner, "=", 2)
		key := strings.TrimSpace(parts[0])
		for _, attr := range node.Attr {
			if attr.Key != key {
				continue
			}
			if len(parts) == 1 {
				return true
			}
			want := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			return attr.Val == want
		}
		return false
	}
	return strings.EqualFold(node.Data, selector)
}

func connectorNodes(root *xhtml.Node, selector string) []*xhtml.Node {
	var matches []*xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if connectorSelectorMatches(node, selector) {
			matches = append(matches, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return matches
}

func connectorNodeText(node *xhtml.Node) string {
	var parts []string
	var walk func(*xhtml.Node)
	walk = func(current *xhtml.Node) {
		if current.Type == xhtml.TextNode {
			if text := strings.TrimSpace(current.Data); text != "" {
				parts = append(parts, text)
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return clip(strings.Join(parts, " "), 4096)
}

func connectorNodeValue(node *xhtml.Node, attribute string) string {
	if attribute != "" {
		for _, attr := range node.Attr {
			if attr.Key == attribute {
				return clip(attr.Val, 4096)
			}
		}
		return ""
	}
	return connectorNodeText(node)
}

func extractHTMLRecords(body []byte, extract map[string]any) ([]map[string]any, error) {
	root, err := xhtml.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("响应不是有效 HTML")
	}
	items := connectorNodes(root, stringValue(extract["item_selector"]))
	fields := asMap(extract["fields"])
	rows := make([]map[string]any, 0, min(len(items), connectorMaxItems+1))
	for _, item := range items[:min(len(items), connectorMaxItems+1)] {
		row := map[string]any{}
		for name, rawField := range fields {
			field := asMap(rawField)
			matches := connectorNodes(item, stringValue(field["selector"]))
			if len(matches) == 0 {
				continue
			}
			row[name] = connectorNodeValue(matches[0], stringValue(field["attribute"]))
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func extractConnectorRecords(body []byte, definition map[string]any) ([]map[string]any, error) {
	extract := asMap(definition["extract"])
	if len(extract) == 0 {
		return nil, nil
	}
	var rows []map[string]any
	var err error
	switch stringValue(extract["format"]) {
	case "json":
		rows, err = extractJSONRecords(body, extract)
	case "html":
		rows, err = extractHTMLRecords(body, extract)
	default:
		err = fmt.Errorf("连接器提取格式无效")
	}
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(rows)
	if err != nil || len(encoded) > connectorMaxExtractedBytes {
		return nil, fmt.Errorf("结构化提取结果超过 256 KiB")
	}
	return rows, nil
}

func (s *Server) fetchConnectorResult(ctx context.Context, connector map[string]any, rawPath, idempotencyKey string, commit bool) (map[string]any, error) {
	definition := asMap(connector["definition"])
	u, err := connectorURL(definition, rawPath)
	if err != nil || !connectorPathAllowed(definition, u.Path) {
		return nil, businessError(400, "请求路径不在连接器允许范围内")
	}
	ips, err := publicNetworkIPs(ctx, u.Hostname())
	if err != nil {
		return nil, businessError(400, err.Error())
	}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].String(), "443"))
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 15 * time.Second, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || req.URL.Scheme != "https" || req.URL.Hostname() != u.Hostname() || !connectorPathAllowed(definition, req.URL.Path) {
			return fmt.Errorf("重定向超出连接器允许范围")
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, businessError(502, "连接器请求失败："+err.Error())
	}
	defer resp.Body.Close()
	limit := int64(intValue(definition["max_bytes"]))
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, businessError(502, "连接器响应读取失败")
	}
	if int64(len(body)) > limit {
		return nil, businessError(413, "连接器响应超过大小限制")
	}
	result := map[string]any{"status": resp.StatusCode, "url": u.String(), "content_type": resp.Header.Get("Content-Type"), "bytes": len(body)}
	if rows, extractErr := extractConnectorRecords(body, definition); extractErr != nil {
		return nil, businessError(502, extractErr.Error())
	} else if rows != nil {
		truncated := len(rows) > connectorMaxItems
		if truncated {
			rows = rows[:connectorMaxItems]
		}
		result["items"] = rows
		result["item_count"] = len(rows)
		result["truncated"] = truncated
	} else {
		result["body"] = string(body)
	}
	if commit {
		_, err = s.PB.Create(ctx, "connector_runs", map[string]any{"tenant_id": connector["tenant_id"], "app_id": connector["app_id"], "connector_id": connector["id"], "revision": connector["revision"], "idempotency_key": idempotencyKey, "status": "completed", "result": result})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Server) fetchConnector(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	_, connector, err := s.connectorForRequest(ctx, r, false)
	if err != nil {
		writeError(w, errStatus(err), err.Error())
		return
	}
	if connector["status"] != "enabled" {
		writeError(w, 409, "连接器尚未启用")
		return
	}
	input := mapBody(r)
	key := strings.TrimSpace(stringValue(input["idempotency_key"]))
	if key == "" || len(key) > 160 {
		writeError(w, 400, "必须提供不超过 160 个字符的 idempotency_key")
		return
	}
	if previous, findErr := s.PB.Find(ctx, "connector_runs", listFilter("connector_id = "+pbFilterString(stringValue(connector["id"])), "idempotency_key = "+pbFilterString(key))); findErr == nil {
		writeJSON(w, 200, previous["result"])
		return
	}
	result, err := s.fetchConnectorResult(ctx, connector, stringValue(input["path"]), key, true)
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) executeTaskConnector(ctx context.Context, run, input map[string]any, assertActive func() error) (string, error) {
	connectorID, key := stringValue(input["connector_id"]), strings.TrimSpace(stringValue(input["idempotency_key"]))
	if connectorID == "" || key == "" || len(key) > 160 {
		return "", fmt.Errorf("连接器必须提供 connector_id 和 idempotency_key")
	}
	allowed := false
	for _, raw := range anySlice(asMap(asMap(run["snapshot"])["scope"])["connector_ids"]) {
		if stringValue(raw) == connectorID {
			allowed = true
		}
	}
	if !allowed {
		return "", fmt.Errorf("连接器未获此任务授权")
	}
	connector, err := s.PB.Get(ctx, "connectors", connectorID)
	if err != nil || connector["tenant_id"] != run["tenant_id"] || connector["app_id"] != run["app_id"] || connector["status"] != "enabled" {
		return "", fmt.Errorf("连接器不存在、未启用或权限已变化")
	}
	if asMap(run["snapshot"])["mode"] == "preview" {
		result, fetchErr := s.fetchConnectorResult(ctx, connector, stringValue(input["path"]), key, false)
		if fetchErr != nil {
			return "", fetchErr
		}
		encoded, _ := json.Marshal(result)
		return string(encoded), nil
	}
	if err := assertActive(); err != nil {
		return "", err
	}
	if previous, findErr := s.PB.Find(ctx, "connector_runs", listFilter("connector_id = "+pbFilterString(connectorID), "idempotency_key = "+pbFilterString(key))); findErr == nil {
		encoded, _ := json.Marshal(previous["result"])
		return string(encoded), nil
	}
	result, err := s.fetchConnectorResult(ctx, connector, stringValue(input["path"]), key, true)
	if err != nil {
		return "", err
	}
	encoded, _ := json.Marshal(result)
	return string(encoded), nil
}
