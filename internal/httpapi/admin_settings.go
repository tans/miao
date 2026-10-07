package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tans/miao/internal/pocketbase"
	"github.com/tans/miao/internal/settings"
)

// adminSettings returns every managed settings group with credentials masked;
// plaintext keys never leave the server.
func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	registration, err := settings.ReadRegistration(ctx, s.PB)
	if err != nil {
		writeError(w, 503, "注册策略读取失败，请检查已保存配置")
		return
	}
	mailConfig, err := settings.ReadMail(ctx, s.PB)
	if err != nil {
		writeError(w, 503, "邮件服务配置读取失败，请检查服务端加密密钥和已保存配置")
		return
	}
	admins, err := settings.ReadAdmins(ctx, s.PB)
	if err != nil {
		writeError(w, 503, "平台管理员名单读取失败")
		return
	}
	backup, err := settings.ReadBackup(ctx, s.PB)
	if err != nil {
		writeError(w, 503, "备份策略读取失败")
		return
	}
	writeJSON(w, 200, map[string]any{
		"encryption_ready": settings.EncryptionReady(),
		"registration":     registration,
		"mail": map[string]any{
			"configured": mailConfig.Configured(), "public_url": mailConfig.PublicURL, "from": mailConfig.From,
			"source": mailConfig.Source, "key_hint": keyHint(mailConfig.Key),
		},
		"admins": admins,
		"backup": backup,
	})
}

// saveSettingRow persists one settings group and its audit entry in the same
// transaction, so a failed save leaves both the value and the audit trail untouched.
func (s *Server) saveSettingRow(w http.ResponseWriter, r *http.Request, action, target string, value any, apply func(ctx context.Context, tx *pocketbase.Client) error) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		if err := apply(ctx, tx); err != nil {
			return err
		}
		_, err := tx.Create(ctx, "platform_audit_logs", map[string]any{
			"actor_id": id.User["id"], "actor_email": id.User["email"], "action": action,
			"target_type": "setting", "target_id": target, "status": 200,
		})
		return err
	})
	if err != nil {
		writeError(w, 503, "平台设置保存失败，未修改配置")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, target: value})
}

func (s *Server) adminRegistrationUpdate(w http.ResponseWriter, r *http.Request) {
	input := mapBody(r)
	config := settings.Registration{Mode: stringValue(input["mode"]), AllowedEmailDomains: []string{}, RequireEmailVerification: boolValue(input["require_email_verification"])}
	domains, ok := input["allowed_email_domains"].([]any)
	if !ok || len(domains) > 50 {
		writeError(w, 400, "请选择有效注册方式，最多允许 50 个邮箱域名")
		return
	}
	seen := map[string]bool{}
	for _, raw := range domains {
		domain, isString := raw.(string)
		domain = strings.ToLower(strings.TrimSpace(domain))
		if !isString || len(domain) > 253 || !settings.ValidRegistrationDomain(domain) {
			writeError(w, 400, "请输入有效邮箱域名，不含 @ 或网址前缀")
			return
		}
		if !seen[domain] {
			seen[domain] = true
			config.AllowedEmailDomains = append(config.AllowedEmailDomains, domain)
		}
	}
	if err := settings.ValidateRegistration(config); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if config.RequireEmailVerification {
		mailConfig, err := settings.ReadMail(r.Context(), s.PB)
		if err != nil || !mailConfig.Configured() {
			writeError(w, 409, "邮箱验证依赖邮件服务，请先保存有效的发件人和服务密钥")
			return
		}
	}
	s.saveSettingRow(w, r, "registration.updated", "registration", config, func(ctx context.Context, tx *pocketbase.Client) error {
		return settings.Put(ctx, tx, settings.RowRegistration, marshalJSON(config), stringValue(who(r).User["id"]))
	})
}

func (s *Server) adminMailUpdate(w http.ResponseWriter, r *http.Request) {
	input := mapBody(r)
	config := settings.Mail{
		PublicURL: strings.TrimRight(strings.TrimSpace(stringValue(input["public_url"])), "/"),
		From:      strings.TrimSpace(stringValue(input["from"])),
	}
	if err := settings.ValidateMail(config); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	keyMode := stringValue(input["key_mode"])
	key := strings.TrimSpace(stringValue(input["api_key"]))
	if !containsString([]string{"keep", "replace", "clear"}, keyMode) {
		writeError(w, 400, "密钥操作无效")
		return
	}
	if keyMode == "replace" && (len(key) < 16 || len(key) > 2000 || strings.ContainsAny(key, "\r\n")) {
		writeError(w, 400, "新密钥必须为 16–2000 个字符且不能换行")
		return
	}
	var sealed string
	if keyMode == "replace" {
		var err error
		if sealed, err = settings.EncryptSecret(key); err != nil {
			writeError(w, 503, "需先配置至少 32 个字符的服务端加密密钥")
			return
		}
	}
	s.saveSettingRow(w, r, "settings.mail.updated", "mail", map[string]any{"public_url": config.PublicURL, "from": config.From}, func(ctx context.Context, tx *pocketbase.Client) error {
		if err := settings.Put(ctx, tx, settings.RowMail, marshalJSON(config), stringValue(who(r).User["id"])); err != nil {
			return err
		}
		switch keyMode {
		case "replace":
			return settings.Put(ctx, tx, settings.RowMailKey, sealed, stringValue(who(r).User["id"]))
		case "clear":
			return settings.Delete(ctx, tx, settings.RowMailKey)
		}
		return nil
	})
}

func (s *Server) adminAdminsUpdate(w http.ResponseWriter, r *http.Request) {
	input := mapBody(r)
	raw, ok := input["emails"].([]any)
	if !ok {
		writeError(w, 400, "平台管理员名单无效")
		return
	}
	requested := make([]string, 0, len(raw))
	for _, item := range raw {
		email, isString := item.(string)
		if !isString {
			writeError(w, 400, "平台管理员名单无效")
			return
		}
		requested = append(requested, email)
	}
	emails, err := settings.NormalizeAdminEmails(requested)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	self := strings.ToLower(strings.TrimSpace(stringValue(who(r).User["email"])))
	if !containsString(emails, self) {
		// Self-removal stays possible with a handover, but only once another
		// listed admin is an actually usable account.
		handover := false
		for _, email := range emails {
			if row, e := s.PB.Find(r.Context(), "users", "email = "+pbFilterString(email)); e == nil && !boolValue(row["disabled"]) && boolValue(row["verified"]) {
				handover = true
				break
			}
		}
		if !handover {
			writeError(w, 409, "移除当前管理员前，名单中需要另一位已注册且可用的管理员")
			return
		}
	}
	s.saveSettingRow(w, r, "settings.admins.updated", "admins", map[string]any{"emails": emails}, func(ctx context.Context, tx *pocketbase.Client) error {
		return settings.Put(ctx, tx, settings.RowAdmins, marshalJSON(settings.Admins{Emails: emails}), stringValue(who(r).User["id"]))
	})
}

func (s *Server) adminBackupUpdate(w http.ResponseWriter, r *http.Request) {
	input := mapBody(r)
	config := settings.Backup{
		Directory:     strings.TrimSpace(stringValue(input["directory"])),
		RetentionDays: intValue(input["retention_days"]),
	}
	if config.RetentionDays == 0 {
		config.RetentionDays = 30
	}
	if err := settings.ValidateBackup(config); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	s.saveSettingRow(w, r, "settings.backup.updated", "backup", config, func(ctx context.Context, tx *pocketbase.Client) error {
		return settings.Put(ctx, tx, settings.RowBackup, marshalJSON(config), stringValue(who(r).User["id"]))
	})
}

func marshalJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
