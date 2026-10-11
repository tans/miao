package httpapi

import (
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

// Anonymous public site serving under /s/: rate limiting and the server-side
// renderer called from the top-level Handler in server.go. The /api/public
// publication endpoints stay in publications.go.

var publicationSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var publicRateMu sync.Mutex
var publicRateBuckets = map[string]publicRateBucket{}
var publicSubmitBuckets = map[string]publicRateBucket{}

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

func allowPublicSubmission(w http.ResponseWriter, r *http.Request) bool {
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
	bucket := publicSubmitBuckets[ip]
	if now.Sub(bucket.window) >= time.Minute {
		bucket = publicRateBucket{window: now}
	}
	bucket.count++
	publicSubmitBuckets[ip] = bucket
	publicRateMu.Unlock()
	if bucket.count > 5 {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "提交过于频繁，请稍后重试")
		return false
	}
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
