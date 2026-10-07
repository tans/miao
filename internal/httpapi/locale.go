package httpapi

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/tans/miao/internal/harness"
)

// Supported UI languages. zh-CN is the source language of every server string,
// so its catalog is implicit; the other languages live in messages_*.go and
// adding one is a pure dictionary extension.
const (
	LangZH    = "zh-CN"
	LangEN    = "en"
	langCookie = "miao_lang"
)

var supportedLanguages = map[string]bool{LangZH: true, LangEN: true}

func normalizeLanguage(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "en", "en-us", "en-gb", "en-*":
		return LangEN
	case "zh", "zh-cn", "zh-sg", "zh-hans":
		return LangZH
	}
	return ""
}

type languageKey struct{}

// withLanguage records the negotiated request language for T() lookups.
func withLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, languageKey{}, lang)
}

func languageFromCtx(ctx context.Context) string {
	lang, _ := ctx.Value(languageKey{}).(string)
	if lang == "" {
		return LangZH
	}
	return lang
}

// requestLanguage resolves the effective language for a request: explicit
// ?lang= override, then the account setting carried in the identity, then the
// value negotiated by the locale middleware (cookie / Accept-Language).
func requestLanguage(r *http.Request) string {
	if override := normalizeLanguage(r.URL.Query().Get("lang")); override != "" {
		return override
	}
	if user := who(r).User; user != nil {
		if lang := normalizeLanguage(stringValue(user["language"])); lang != "" {
			return lang
		}
	}
	if lang := languageFromCtx(r.Context()); lang != "" {
		return lang
	}
	return LangZH
}

// T localizes a user-visible server string built during a request. zh-CN
// strings are the source of truth and pass through unchanged; other languages
// look the string up in the message catalogs and fall back to zh-CN.
func T(r *http.Request, template string, params ...map[string]string) string {
	return localize(requestLanguage(r), template, params...)
}

// TLang localizes for a language resolved without a request, e.g. a run's
// stored actor language inside background workers.
func TLang(lang, template string, params ...map[string]string) string {
	return localize(defaultString(lang, LangZH), template, params...)
}

// userLanguage resolves the saved UI language of an account; empty follows the
// browser, which server-side defaults to zh-CN.
func (s *Server) userLanguage(ctx context.Context, userID string) string {
	if user, err := s.PB.Get(ctx, "users", userID); err == nil {
		if lang := normalizeLanguage(stringValue(user["language"])); lang != "" {
			return lang
		}
	}
	return LangZH
}

// runLanguage resolves the UI language of the user a harness run acts for, so
// user-visible run content and model output follow their account setting even
// when the step executes in a background worker.
func (s *Server) runLanguage(ctx context.Context, run *harness.Run) string {
	if lang := normalizeLanguage(stringValue(asMap(run.Context)["language"])); lang != "" {
		return lang
	}
	return s.userLanguage(ctx, run.UserID)
}

// languageDisplayName names a language for embedding into model output
// directives.
func languageDisplayName(lang string) string {
	if lang == LangEN {
		return "English"
	}
	return "简体中文"
}

// outputLanguageDirective is appended to model system prompts so user-facing
// model output follows the run's UI language. The prompt contract itself is
// not rewritten per UI language.
func outputLanguageDirective(lang string) string {
	return TLang(lang, "\n所有面向用户的输出（总结、question 字段、补充信息请求）都使用 {lang}。", map[string]string{"lang": languageDisplayName(lang)})
}

// localize translates a source zh-CN message, interpolating {name} params.
func localize(lang, template string, params ...map[string]string) string {
	message := template
	if lang != LangZH {
		if translated, ok := translateMessage(lang, template); ok {
			message = translated
		}
	}
	var optional map[string]string
	if len(params) > 0 {
		optional = params[0]
	}
	return interpolate(message, optional)
}

// languageWriter carries the negotiated language to the response boundary so
// writeError and writeBusinessError can localize without threading the request
// through every handler. It only decorates /api/ responses.
type languageWriter struct {
	http.ResponseWriter
	lang atomic.Value
}

func (w *languageWriter) setLanguage(lang string) { w.lang.Store(lang) }
func (w *languageWriter) currentLanguage() string {
	if lang, ok := w.lang.Load().(string); ok && lang != "" {
		return lang
	}
	return LangZH
}
func (w *languageWriter) WriteHeader(code int) { w.ResponseWriter.WriteHeader(code) }
func (w *languageWriter) Write(data []byte) (int, error) {
	return w.ResponseWriter.Write(data)
}
func (w *languageWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// negotiateLanguage picks the anonymous language: ?lang= override, cookie,
// then Accept-Language, defaulting to zh-CN.
func negotiateLanguage(r *http.Request) string {
	if lang := normalizeLanguage(r.URL.Query().Get("lang")); lang != "" {
		return lang
	}
	if cookie, err := r.Cookie(langCookie); err == nil {
		if lang := normalizeLanguage(cookie.Value); lang != "" {
			return lang
		}
	}
	for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		weight := part
		if idx := strings.IndexByte(part, ';'); idx >= 0 {
			weight = part[:idx]
		}
		if lang := normalizeLanguage(weight); lang != "" {
			return lang
		}
	}
	return LangZH
}

// wrapLanguage decorates /api/ responses with the negotiated language and
// remembers an explicit ?lang= choice in a cookie for anonymous visitors.
func wrapLanguage(w http.ResponseWriter, r *http.Request) (http.ResponseWriter, *http.Request) {
	lang := negotiateLanguage(r)
	if r.URL.Query().Get("lang") != "" {
		http.SetCookie(w, &http.Cookie{Name: langCookie, Value: lang, Path: "/", MaxAge: 31536000, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil})
	}
	writer, ok := w.(*languageWriter)
	if !ok {
		writer = &languageWriter{ResponseWriter: w}
		writer.setLanguage(lang)
	}
	return writer, r.WithContext(withLanguage(r.Context(), lang))
}
