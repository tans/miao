package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const externalMaxBytes = 2 << 20
const externalMaxItems = 100
const externalMaxExtractedBytes = 256 << 10

var externalSelectorPattern = regexp.MustCompile(`^(\*|[a-zA-Z][a-zA-Z0-9_-]*([.#][a-zA-Z][a-zA-Z0-9_-]*)?|#[a-zA-Z][a-zA-Z0-9_-]*|\.[a-zA-Z][a-zA-Z0-9_-]*|\[[a-zA-Z][a-zA-Z0-9_-]*(=["']?[a-zA-Z0-9:_./ -]+["']?)?\])$`)

func externalURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("请求 URL 无效")
	}
	if u.Path == "" {
		u.Path = "/"
	}
	if strings.Contains(u.Path, "..") || strings.ContainsAny(u.Path, "\\\x00") {
		return nil, fmt.Errorf("请求 URL 路径无效")
	}
	return u, nil
}

func normalizeExternalExtract(raw any) (map[string]any, string) {
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
		if !validExternalSelector(itemSelector) {
			return nil, "HTML item_selector 暂只支持单个标签、#id、.class 或 [attribute] 选择器"
		}
		result["item_selector"] = itemSelector
		normalizedFields := map[string]any{}
		for name, rawField := range fields {
			field := asMap(rawField)
			selector := stringValue(field["selector"])
			attribute := stringValue(field["attribute"])
			if !validExternalSelector(selector) || attribute != "" && !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`).MatchString(attribute) {
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

// validJSONPointer, connectorURL, connectorPathAllowed, publicNetworkIPs,
// publicNetworkHost and jsonPointer keep their single definitions in
// connectors.go; the external* helpers below intentionally reuse them.

func validExternalSelector(value string) bool { return externalSelectorPattern.MatchString(value) }

func scalarExternalValue(value any) (any, bool) {
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

func extractExternalJSONRecords(body []byte, extract map[string]any) ([]map[string]any, error) {
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
	rows := make([]map[string]any, 0, min(len(list), externalMaxItems+1))
	for _, item := range list[:min(len(list), externalMaxItems+1)] {
		row := map[string]any{}
		for name, rawPointer := range fields {
			value, found := jsonPointer(item, stringValue(rawPointer))
			if !found {
				continue
			}
			if scalar, valid := scalarExternalValue(value); valid {
				row[name] = scalar
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func externalSelectorMatches(node *html.Node, selector string) bool {
	if node.Type != html.ElementNode {
		return false
	}
	if selector == "*" {
		return true
	}
	if strings.Contains(selector, ".") && !strings.HasPrefix(selector, ".") {
		parts := strings.SplitN(selector, ".", 2)
		return strings.EqualFold(node.Data, parts[0]) && externalSelectorMatches(node, "."+parts[1])
	}
	if strings.Contains(selector, "#") && !strings.HasPrefix(selector, "#") {
		parts := strings.SplitN(selector, "#", 2)
		return strings.EqualFold(node.Data, parts[0]) && externalSelectorMatches(node, "#"+parts[1])
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

func externalNodes(root *html.Node, selector string) []*html.Node {
	var matches []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if externalSelectorMatches(node, selector) {
			matches = append(matches, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return matches
}

func externalNodeText(node *html.Node) string {
	var parts []string
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
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

func externalNodeValue(node *html.Node, attribute string) string {
	if attribute != "" {
		for _, attr := range node.Attr {
			if attr.Key == attribute {
				return clip(attr.Val, 4096)
			}
		}
		return ""
	}
	return externalNodeText(node)
}

func extractExternalHTMLRecords(body []byte, extract map[string]any) ([]map[string]any, error) {
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("响应不是有效 HTML")
	}
	items := externalNodes(root, stringValue(extract["item_selector"]))
	fields := asMap(extract["fields"])
	rows := make([]map[string]any, 0, min(len(items), externalMaxItems+1))
	for _, item := range items[:min(len(items), externalMaxItems+1)] {
		row := map[string]any{}
		for name, rawField := range fields {
			field := asMap(rawField)
			matches := externalNodes(item, stringValue(field["selector"]))
			if len(matches) == 0 {
				continue
			}
			row[name] = externalNodeValue(matches[0], stringValue(field["attribute"]))
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func extractExternalRecords(body []byte, definition map[string]any) ([]map[string]any, error) {
	extract := asMap(definition["extract"])
	if len(extract) == 0 {
		return nil, nil
	}
	var rows []map[string]any
	var err error
	switch stringValue(extract["format"]) {
	case "json":
		rows, err = extractExternalJSONRecords(body, extract)
	case "html":
		rows, err = extractExternalHTMLRecords(body, extract)
	default:
		err = fmt.Errorf("连接器提取格式无效")
	}
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(rows)
	if err != nil || len(encoded) > externalMaxExtractedBytes {
		return nil, fmt.Errorf("结构化提取结果超过 256 KiB")
	}
	return rows, nil
}

func fetchExternalResult(ctx context.Context, rawURL string, extracts ...any) (map[string]any, error) {
	u, err := externalURL(rawURL)
	if err != nil {
		return nil, businessError(400, "请求 URL 无效")
	}
	definition := map[string]any{"max_bytes": externalMaxBytes}
	if len(extracts) > 0 && extracts[0] != nil {
		extract, msg := normalizeExternalExtract(extracts[0])
		if msg != "" {
			return nil, businessError(400, msg)
		}
		definition["extract"] = extract
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
			return fmt.Errorf("重定向超出请求范围")
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, businessError(502, "外部请求失败："+err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, businessError(502, fmt.Sprintf("来源响应 HTTP %d，未解析为业务数据", resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(externalMaxBytes)+1))
	if err != nil {
		return nil, businessError(502, "外部响应读取失败")
	}
	if len(body) > externalMaxBytes {
		return nil, businessError(413, "外部响应超过大小限制")
	}
	result := map[string]any{"status": resp.StatusCode, "url": u.String(), "content_type": resp.Header.Get("Content-Type"), "bytes": len(body)}
	if rows, extractErr := extractExternalRecords(body, definition); extractErr != nil {
		return nil, businessError(502, extractErr.Error())
	} else if rows != nil {
		truncated := len(rows) > externalMaxItems
		if truncated {
			rows = rows[:externalMaxItems]
		}
		result["items"], result["item_count"], result["truncated"] = rows, len(rows), truncated
	} else {
		result["body"] = string(body)
	}
	return result, nil
}
