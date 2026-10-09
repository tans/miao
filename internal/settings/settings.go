// Package settings is the single read/write service for platform business
// settings: registration, mail, LLM/Jev credentials, platform admins, and
// backup policy. Valid database records are the authoritative source; the
// admin console reads and writes the same rows through the HTTP API. Select
// environment variables bootstrap a fresh installation once and are ignored
// afterwards; the data directory, listen address, and the settings encryption
// key stay environment-only.
package settings

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strings"

	"github.com/tans/miao/internal/jev"
	"github.com/tans/miao/internal/pocketbase"
)

// EncryptionEnv is the only secret that stays outside the database; it
// protects the credential rows stored in platform_settings.
const EncryptionEnv = "MIAO_SETTINGS_ENCRYPTION_KEY"

// GatewayBase is the Vercel AI Gateway root shared by the LLM chat transport
// and the Jev evaluation endpoint.
const GatewayBase = "https://ai-gateway.vercel.sh"

// platform_settings row names.
const (
	RowRegistration = "registration"
	RowMail         = "mail_config"
	RowMailKey      = "mail_api_key"
	RowLLM          = "ai_llm_config"
	RowLLMKey       = "ai_gateway_api_key"
	RowJev          = "jev_config"
	RowJevKey       = "jev_api_key"
	RowAdmins       = "platform_admins"
	RowBackup       = "backup_config"
	RowBranding     = "branding"
	RowEnvImport    = "env_import"
)

// Bootstrap environment variables per settings group. They are only read while
// the group has neither a saved row nor a recorded one-time import; see import.go.
var bootstrapEnv = map[string][]string{
	"registration": {"MIAO_REGISTRATION_MODE", "MIAO_ALLOWED_EMAIL_DOMAINS", "MIAO_REQUIRE_EMAIL_VERIFICATION"},
	"mail":         {"MIAO_PUBLIC_URL", "MIAO_MAIL_FROM", "RESEND_API_KEY"},
	"llm":          {"MIAO_AI_PROVIDER", "MIAO_AI_BASE_URL", "MIAO_AI_MODEL", "AI_GATEWAY_API_KEY"},
	"jev":          {"MIAO_JEV_MODEL", "MIAO_JEV_API_KEY"},
	"admins":       {"MIAO_ADMIN_EMAILS"},
	"backup":       {"MIAO_BACKUP_DIR", "MIAO_BACKUP_RETENTION_DAYS"},
	"branding":     {"MIAO_TITLE"},
}

type Registration struct {
	Mode                     string   `json:"mode"`
	AllowedEmailDomains      []string `json:"allowed_email_domains"`
	RequireEmailVerification bool     `json:"require_email_verification"`
}

type Mail struct {
	PublicURL string `json:"public_url"`
	From      string `json:"from"`
}

type Admins struct {
	Emails []string `json:"emails"`
}

type Backup struct {
	Directory     string `json:"directory"`
	RetentionDays int    `json:"retention_days"`
}

type Branding struct {
	Title string `json:"title"`
}

type LLM struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
}

type Jev struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// MailConfig is the effective mail transport: link base, sender, and the
// decrypted provider key. Source states which layer provided the values.
type MailConfig struct {
	PublicURL, From, Key, Source string
}

func (c MailConfig) Configured() bool { return c.From != "" && c.Key != "" }

type AIConfig struct {
	Key, Provider, BaseURL, Model, Source string
	Enabled                               bool
}

type JevConfig struct {
	Key, Provider, Model, Source string
	Enabled, Inherited           bool
}

// AdminList is the effective platform admin authorization. The environment
// variable only bootstraps: once a row is saved or imported, it is ignored.
type AdminList struct {
	Emails []string `json:"emails"`
	Source string   `json:"source"`
}

type BackupPolicy struct {
	Directory     string `json:"directory"`
	RetentionDays int    `json:"retention_days"`
	Source        string `json:"source"`
}

func defaultRegistration() Registration {
	return Registration{Mode: "open", AllowedEmailDomains: []string{}}
}

func defaultBackup() BackupPolicy {
	return BackupPolicy{Directory: "", RetentionDays: 30, Source: "default"}
}

func defaultBranding() Branding {
	return Branding{Title: "MIAO · 企业内部工具"}
}

func defaultLLM() LLM {
	return LLM{Enabled: true, Provider: "vercel", BaseURL: "http://127.0.0.1:3210/api/v1", Model: "gpt-5.2"}
}

func defaultJev() Jev {
	return Jev{Enabled: true, Provider: jev.ProviderTypesafe, Model: ""}
}

func env(key string) string { return os.Getenv(key) }

func envSet(group string) bool {
	for _, key := range bootstrapEnv[group] {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

func splitListEnv(key string) []string {
	out := []string{}
	for _, item := range strings.Split(os.Getenv(key), ",") {
		if item = strings.ToLower(strings.TrimSpace(item)); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// Find returns the raw platform_settings row for name; found is false when the
// row does not exist yet.
func Find(ctx context.Context, pb *pocketbase.Client, name string) (row map[string]any, found bool, err error) {
	row, err = pb.Find(ctx, "platform_settings", "name = "+pbFilterString(name))
	if err != nil {
		var pbErr *pocketbase.Error
		if errors.As(err, &pbErr) && pbErr.Status == 404 {
			return nil, false, nil
		}
		return nil, false, err
	}
	return row, true, nil
}

// Put upserts a settings row inside the caller's transaction.
func Put(ctx context.Context, tx *pocketbase.Client, name, value, updatedBy string) error {
	row, err := tx.Find(ctx, "platform_settings", "name = "+pbFilterString(name))
	if err == nil {
		_, err = tx.Update(ctx, "platform_settings", stringValue(row["id"]), map[string]any{"value": value, "updated_by": updatedBy})
	} else {
		var pbErr *pocketbase.Error
		if !errors.As(err, &pbErr) || pbErr.Status != 404 {
			return err
		}
		_, err = tx.Create(ctx, "platform_settings", map[string]any{"name": name, "value": value, "updated_by": updatedBy})
	}
	return err
}

// Delete removes a settings row; deleting an absent row succeeds.
func Delete(ctx context.Context, tx *pocketbase.Client, name string) error {
	row, err := tx.Find(ctx, "platform_settings", "name = "+pbFilterString(name))
	if err != nil {
		var pbErr *pocketbase.Error
		if errors.As(err, &pbErr) && pbErr.Status == 404 {
			return nil
		}
		return err
	}
	return tx.Delete(ctx, "platform_settings", stringValue(row["id"]))
}

func decodeRow[T any](row map[string]any, target *T) error {
	return json.Unmarshal([]byte(stringValue(row["value"])), target)
}

func pbFilterString(value string) string { encoded, _ := json.Marshal(value); return string(encoded) }

func stringValue(raw any) string { value, _ := raw.(string); return value }

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// EncryptionReady reports whether stored credentials can be encrypted and
// decrypted with the configured environment key.
func EncryptionReady() bool { return len(env(EncryptionEnv)) >= 32 }

// EncryptSecret seals a credential with the environment key in the same v1
// format used by every stored credential row.
func EncryptSecret(key string) (string, error) {
	secret := env(EncryptionEnv)
	if len(secret) < 32 {
		return "", fmt.Errorf("%s must contain at least 32 characters", EncryptionEnv)
	}
	hash := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(key), nil)
	overhead := gcm.Overhead()
	tag, ciphertext := sealed[len(sealed)-overhead:], sealed[:len(sealed)-overhead]
	enc := base64.RawURLEncoding
	return "v1." + enc.EncodeToString(nonce) + "." + enc.EncodeToString(tag) + "." + enc.EncodeToString(ciphertext), nil
}

// DecryptSecret opens a stored credential. Failures are surfaced to the caller
// instead of falling back to a bootstrap environment value.
func DecryptSecret(stored string) (string, error) {
	secret := env(EncryptionEnv)
	if len(secret) < 32 {
		return "", fmt.Errorf("%s must contain at least 32 characters", EncryptionEnv)
	}
	key := sha256.Sum256([]byte(secret))
	parts := strings.Split(stored, ".")
	if len(parts) != 4 || parts[0] != "v1" {
		return "", errors.New("stored credential has an unsupported format")
	}
	iv, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	tag, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(iv) != gcm.NonceSize() || len(tag) != gcm.Overhead() {
		return "", errors.New("stored credential is invalid")
	}
	plain, err := gcm.Open(nil, iv, append(ciphertext, tag...), nil)
	return string(plain), err
}

// ValidAIBaseURL accepts HTTPS addresses or loopback HTTP for CAPI endpoints.
func ValidAIBaseURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "http" && containsString([]string{"localhost", "127.0.0.1", "::1"}, u.Hostname())
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// NormalizeAdminEmails lowercases, dedupes, and validates an admin list.
func NormalizeAdminEmails(emails []string) ([]string, error) {
	out, seen := []string{}, map[string]bool{}
	for _, email := range emails {
		email = normalizeEmail(email)
		if email == "" {
			continue
		}
		if _, err := mail.ParseAddress(email); err != nil || !strings.Contains(email, "@") || len(email) > 254 {
			return nil, errors.New("平台管理员须为有效邮箱地址")
		}
		if !seen[email] {
			seen[email] = true
			out = append(out, email)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("至少保留一个平台管理员邮箱")
	}
	if len(out) > 50 {
		return nil, errors.New("平台管理员最多 50 个")
	}
	return out, nil
}

// MarshalAdmins encodes an admin list for the platform_admins row.
func MarshalAdmins(emails []string) string {
	encoded, _ := json.Marshal(Admins{Emails: emails})
	return string(encoded)
}

// ValidateRegistration checks a registration policy row before saving.
func ValidateRegistration(config Registration) error {
	if !containsString([]string{"open", "invite", "closed"}, config.Mode) {
		return errors.New("注册方式无效")
	}
	if len(config.AllowedEmailDomains) > 50 {
		return errors.New("允许的邮箱域名最多 50 个")
	}
	for _, domain := range config.AllowedEmailDomains {
		if len(domain) > 253 || !ValidRegistrationDomain(domain) {
			return errors.New("邮箱域名无效，须为不含 @ 的域名")
		}
	}
	return nil
}

// ValidRegistrationDomain accepts ASCII domain labels or punycode.
func ValidRegistrationDomain(domain string) bool {
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

// ValidateLLM checks an LLM provider row before saving.
func ValidateLLM(config LLM) error {
	if config.Provider != "vercel" && config.Provider != "capi" {
		return errors.New("不支持的 LLM 提供商")
	}
	if config.Model == "" || len(config.Model) > 160 || strings.ContainsAny(config.Model, "\r\n") {
		return errors.New("LLM 模型名称无效")
	}
	if config.Provider == "capi" && !ValidAIBaseURL(config.BaseURL) {
		return errors.New("LLM 接口地址无效；仅支持 HTTPS 或本机 HTTP")
	}
	return nil
}

// ValidateJev checks a Jev provider row before saving.
func ValidateJev(config Jev) error {
	if config.Provider != jev.ProviderTypesafe {
		return errors.New("Jev 仅支持 Typesafe 官方接口")
	}
	if len(config.Model) > 160 || strings.ContainsAny(config.Model, "\r\n") {
		return errors.New("Jev 模型名称无效")
	}
	return nil
}

// ValidateMail checks a mail transport row before saving.
func ValidateMail(config Mail) error {
	if len(config.From) > 200 || strings.ContainsAny(config.From, "\r\n") {
		return errors.New("发件人地址无效")
	}
	if config.PublicURL != "" {
		u, err := url.Parse(config.PublicURL)
		if err != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" || u.Scheme != "https" && u.Scheme != "http" || len(config.PublicURL) > 300 {
			return errors.New("公开访问地址无效，须为 http(s) 链接")
		}
	}
	return nil
}

// ValidateBackup checks a backup policy row before saving.
func ValidateBackup(config Backup) error {
	if len(config.Directory) > 500 || strings.ContainsAny(config.Directory, "\r\n\x00") {
		return errors.New("备份目录无效")
	}
	if config.RetentionDays < 1 || config.RetentionDays > 3650 {
		return errors.New("备份保留天数须为 1–3650")
	}
	return nil
}

// ValidateBranding checks the browser title before saving it to the platform.
func ValidateBranding(config Branding) error {
	config.Title = strings.TrimSpace(config.Title)
	if len([]rune(config.Title)) < 1 || len([]rune(config.Title)) > 120 || strings.ContainsAny(config.Title, "\r\n") {
		return errors.New("平台标题必须为 1 到 120 个字符")
	}
	return nil
}
