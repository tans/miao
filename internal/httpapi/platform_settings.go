package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/tans/miao/internal/pocketbase"
)

type registrationSettings struct {
	Mode    string   `json:"mode"`
	Domains []string `json:"allowed_email_domains"`
}

func (s *Server) readRegistrationSettings(ctx context.Context) (registrationSettings, error) {
	config := registrationSettings{Mode: s.RegistrationMode, Domains: []string{}}
	for domain := range s.AllowedDomains {
		config.Domains = append(config.Domains, domain)
	}
	sort.Strings(config.Domains)
	row, err := s.PB.Find(ctx, "platform_settings", "name = "+pbFilterString("registration"))
	if err != nil {
		var pbErr *pocketbase.Error
		if errors.As(err, &pbErr) && pbErr.Status == http.StatusNotFound {
			if !containsString([]string{"open", "invite", "closed"}, config.Mode) {
				return config, errors.New("invalid registration environment")
			}
			return config, nil
		}
		return config, err
	}
	config = registrationSettings{}
	if err = json.Unmarshal([]byte(stringValue(row["value"])), &config); err != nil {
		return config, err
	}
	if !containsString([]string{"open", "invite", "closed"}, config.Mode) || len(config.Domains) > 50 {
		return config, errors.New("invalid registration settings")
	}
	for _, domain := range config.Domains {
		if len(domain) > 253 || !validRegistrationDomain(domain) {
			return config, errors.New("invalid registration domain")
		}
	}
	if config.Domains == nil {
		config.Domains = []string{}
	}
	return config, nil
}

func (s *Server) adminSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	input := mapBody(r)
	mode := stringValue(input["mode"])
	domains, ok := input["allowed_email_domains"].([]any)
	if !containsString([]string{"open", "invite", "closed"}, mode) || !ok || len(domains) > 50 {
		writeError(w, 400, "请选择有效注册方式，最多允许 50 个邮箱域名")
		return
	}
	config := registrationSettings{Mode: mode, Domains: []string{}}
	seen := map[string]bool{}
	for _, raw := range domains {
		domain, isString := raw.(string)
		domain = strings.ToLower(strings.TrimSpace(domain))
		if !isString || len(domain) > 253 || !validRegistrationDomain(domain) {
			writeError(w, 400, "请输入有效邮箱域名，不含 @ 或网址前缀")
			return
		}
		if !seen[domain] {
			seen[domain] = true
			config.Domains = append(config.Domains, domain)
		}
	}
	sort.Strings(config.Domains)
	value, _ := json.Marshal(config)
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		row, err := tx.Find(ctx, "platform_settings", "name = "+pbFilterString("registration"))
		if err == nil {
			_, err = tx.Update(ctx, "platform_settings", stringValue(row["id"]), map[string]any{"value": string(value), "updated_by": id.User["id"]})
		} else {
			var pbErr *pocketbase.Error
			if !errors.As(err, &pbErr) || pbErr.Status != http.StatusNotFound {
				return err
			}
			_, err = tx.Create(ctx, "platform_settings", map[string]any{"name": "registration", "value": string(value), "updated_by": id.User["id"]})
		}
		if err != nil {
			return err
		}
		_, err = tx.Create(ctx, "platform_audit_logs", map[string]any{
			"actor_id": id.User["id"], "actor_email": id.User["email"], "action": "registration.updated",
			"target_type": "setting", "target_id": "registration", "status": 200,
		})
		return err
	})
	if err != nil {
		writeError(w, 503, "平台设置保存失败，未修改注册策略")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "registration": config})
}

func validRegistrationDomain(domain string) bool {
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
