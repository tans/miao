package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/tans/miao/internal/settings"
)

// maxNotifyChannels bounds the stored JSON document and each webhook URL;
// both failures are validation errors, never silent truncation.
const maxNotifyChannels = 4000

type notifyChannels struct {
	Emails    []string          `json:"emails"`
	WeCom     string            `json:"wecom_webhook"`
	DingTalk  string            `json:"dingtalk_webhook"`
	Feishu    string            `json:"feishu_webhook"`
	RawFields map[string]string `json:"-"`
}

// parseNotifyChannels reads the workspace's stored channel configuration.
func parseNotifyChannels(raw string) notifyChannels {
	config := notifyChannels{Emails: []string{}}
	if strings.TrimSpace(raw) == "" {
		return config
	}
	_ = json.Unmarshal([]byte(raw), &config)
	if config.Emails == nil {
		config.Emails = []string{}
	}
	return config
}

func validNotifyWebhook(value string) bool {
	if value == "" {
		return true
	}
	return len(value) <= 1000 && strings.HasPrefix(value, "https://")
}

// validateNotifyChannels checks one workspace's channel configuration before
// saving. Emails stay as entered (normalized on delivery); URLs must be HTTPS.
func validateNotifyChannels(raw string) (string, error) {
	config := parseNotifyChannels(raw)
	if len(config.Emails) > 20 {
		return "", fmt.Errorf("通知邮箱最多 20 个")
	}
	seen := map[string]bool{}
	for _, email := range config.Emails {
		email = normalizeEmail(email)
		if email == "" {
			continue
		}
		if !validEmail(email) || len(email) > 254 {
			return "", fmt.Errorf("通知邮箱无效：%s", email)
		}
		if !seen[email] {
			seen[email] = true
		}
	}
	for name, url := range map[string]string{"wecom_webhook": config.WeCom, "dingtalk_webhook": config.DingTalk, "feishu_webhook": config.Feishu} {
		if !validNotifyWebhook(url) {
			return "", fmt.Errorf("Webhook 地址无效，须为不超过 1000 个字符的 https 链接：%s", name)
		}
	}
	encoded, err := json.Marshal(config)
	if err != nil || len(encoded) > maxNotifyChannels {
		return "", fmt.Errorf("通知渠道配置过长")
	}
	return string(encoded), nil
}

// notifyChannelPayload renders one message as the body each channel expects.
func notifyChannelPayload(kind, title, text string) any {
	switch kind {
	case "wecom":
		return map[string]any{"msgtype": "text", "text": map[string]any{"content": title + "\n" + text}}
	case "dingtalk":
		return map[string]any{"msgtype": "text", "text": map[string]any{"content": title + "\n" + text}}
	case "feishu":
		return map[string]any{"msg_type": "text", "content": map[string]any{"text": title + "\n" + text}}
	}
	return nil
}

// postWebhook delivers one bot message; non-2xx responses become errors so the
// run stays pending and a later scan retries.
func postWebhook(ctx context.Context, url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", res.StatusCode)
	}
	return nil
}

// dispatchNotifyChannels sends one automation message to the workspace's
// configured external channels. Every failure is logged and recorded on the
// run; the in-app notification and audit trail stay untouched. Delivery uses a
// detached context so the caller's request/scan timeout cannot drop it.
// getNotifyChannels returns the workspace's channel configuration for the
// settings page; webhooks are already stored in plain configuration form.
func (s *Server) getNotifyChannels(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireManager(w, id) {
		return
	}
	config := parseNotifyChannels(stringValue(id.Tenant["notify_channels"]))
	writeJSON(w, 200, map[string]any{"emails": config.Emails, "wecom_webhook": config.WeCom, "dingtalk_webhook": config.DingTalk, "feishu_webhook": config.Feishu, "can_manage": stringValue(id.Membership["role"]) == "owner"})
}

// updateNotifyChannels lets the workspace owner save the external channel
// configuration on the tenant record.
func (s *Server) updateNotifyChannels(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireOwner(w, id) {
		return
	}
	input := mapBody(r)
	emails := []string{}
	if raw, ok := input["emails"].([]any); ok {
		for _, item := range raw {
			emails = append(emails, normalizeEmail(stringValue(item)))
		}
	}
	draft := notifyChannels{
		Emails:   emails,
		WeCom:    strings.TrimSpace(stringValue(input["wecom_webhook"])),
		DingTalk: strings.TrimSpace(stringValue(input["dingtalk_webhook"])),
		Feishu:   strings.TrimSpace(stringValue(input["feishu_webhook"])),
	}
	encoded, err := json.Marshal(draft)
	if err != nil {
		writeError(w, 400, "通知渠道配置无效")
		return
	}
	saved, err := validateNotifyChannels(string(encoded))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	if _, err := s.PB.Update(ctx, "tenants", stringValue(id.Tenant["id"]), map[string]any{"notify_channels": saved}); err != nil {
		writeError(w, 503, "通知渠道保存失败")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "emails": draft.Emails, "wecom_webhook": draft.WeCom, "dingtalk_webhook": draft.DingTalk, "feishu_webhook": draft.Feishu})
}

// testNotifyChannels sends a test message to every configured channel so the
// owner can verify the endpoints before relying on real automations.
func (s *Server) testNotifyChannels(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireOwner(w, id) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	tenant, err := s.PB.Get(ctx, "tenants", stringValue(id.Tenant["id"]))
	if err != nil {
		writeError(w, 503, "通知渠道暂不可用")
		return
	}
	failures := s.dispatchNotifyChannels(ctx, tenant, "test", "MIAO 测试通知", "这是一条来自工作区「"+stringValue(tenant["name"])+"」的测试通知。")
	if len(failures) > 0 {
		writeJSON(w, 200, map[string]any{"ok": false, "failures": failures})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) dispatchNotifyChannels(ctx context.Context, tenant map[string]any, key, title, text string) []string {
	config := parseNotifyChannels(stringValue(tenant["notify_channels"]))
	failures := []string{}
	if len(config.Emails) == 0 && config.WeCom == "" && config.DingTalk == "" && config.Feishu == "" {
		return failures
	}
	// ponytail: single shared timeout for all channels; per-channel budgets if one provider stalls routinely.
	deliveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	var mailConfig settings.MailConfig
	if len(config.Emails) > 0 {
		mailConfig, _ = settings.ReadMail(deliveryCtx, s.PB)
		if !mailConfig.Configured() {
			failures = append(failures, "email:邮件服务尚未配置")
		}
	}
	base := s.publicBaseURL(deliveryCtx)
	content := strings.TrimSpace(text) + "\n\n" + base + "/#notifications"
	for kind, url := range map[string]string{"wecom": config.WeCom, "dingtalk": config.DingTalk, "feishu": config.Feishu} {
		if url == "" {
			continue
		}
		if err := postWebhook(deliveryCtx, url, notifyChannelPayload(kind, title, content)); err != nil {
			failures = append(failures, kind+":"+err.Error())
		}
	}
	for _, email := range config.Emails {
		if email == "" || !mailConfig.Configured() {
			continue
		}
		subject := title
		if tenantName := stringValue(tenant["name"]); tenantName != "" {
			subject = "[" + tenantName + "] " + title
		}
		message := "<p>" + html.EscapeString(title) + "</p><p>" + html.EscapeString(text) + "</p><p><a href=\"" + html.EscapeString(base+"/#notifications") + "\">" + html.EscapeString("打开 MIAO 通知中心") + "</a></p>"
		if err := s.sendMail(deliveryCtx, email, subject, message); err != nil {
			failures = append(failures, "email:"+err.Error())
		}
	}
	for _, failure := range failures {
		s.Logger.Warn("外部通知渠道投递失败", "event_key", key, "error", failure)
	}
	return failures
}
