package httpapi

import "github.com/microcosm-cc/bluemonday"

var publicMarkupPolicy = func() *bluemonday.Policy {
	policy := bluemonday.NewPolicy()
	policy.AllowElements("p", "br", "strong", "b", "em", "i", "u", "s", "h2", "h3", "h4", "blockquote", "ul", "ol", "li", "code", "pre", "hr", "a")
	policy.AllowAttrs("href").OnElements("a")
	policy.RequireParseableURLs(true)
	policy.AllowURLSchemes("https", "http")
	policy.RequireNoFollowOnLinks(true)
	policy.RequireNoReferrerOnLinks(true)
	policy.AddTargetBlankToFullyQualifiedLinks(true)
	return policy
}()

func sanitizePublicMarkup(value string) string {
	return publicMarkupPolicy.Sanitize(clip(value, 64000))
}

func publicDisplayFields(fields []map[string]any, read map[string]any) []map[string]any {
	result := filterPublicFields(fields, stringSlice(anySlice(read["fields"])))
	for index, field := range result {
		copy := cloneAnyMap(field)
		if containsString(stringSlice(anySlice(read["html_fields"])), stringValue(field["name"])) {
			copy["format"] = "sanitized_html"
		}
		result[index] = copy
	}
	return result
}
