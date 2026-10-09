package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/tans/miao/internal/jev"
	"github.com/tans/miao/internal/pocketbase"
	"github.com/tans/miao/internal/settings"
)

// readCheckRow loads the saved connection check result for a service.
func readCheckRow[T any](ctx context.Context, pb *pocketbase.Client, name string, target *T) error {
	row, err := pb.Find(ctx, "platform_settings", "name = "+pbFilterString(name))
	if err != nil {
		var pbErr *pocketbase.Error
		if errors.As(err, &pbErr) && pbErr.Status == 404 {
			return nil
		}
		return err
	}
	return json.Unmarshal([]byte(stringValue(row["value"])), target)
}

func keyHint(key string) string {
	if key == "" {
		return ""
	}
	return "••••••" + key[max(0, len(key)-4):]
}

func (s *Server) adminAIServices(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	decision, err := settings.ReadJevConfig(ctx, s.PB)
	if err != nil {
		writeError(w, 503, "AI 配置读取失败，请检查服务端加密密钥和已保存配置")
		return
	}
	var check map[string]any
	if err := readCheckRow(ctx, s.PB, "ai_check_jev", &check); err != nil {
		writeError(w, 503, "连接检查历史暂不可用")
		return
	}
	writeJSON(w, 200, map[string]any{
		"encryption_ready": settings.EncryptionReady(),
		"jev":              map[string]any{"enabled": decision.Enabled, "configured": decision.Key != "", "provider": decision.Provider, "endpoint": jev.EndpointFor(decision.Provider), "model": decision.Model, "source": decision.Source, "inherited": decision.Inherited, "key_hint": keyHint(decision.Key), "last_check": check},
	})
}

func (s *Server) adminAIServiceUpdate(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if kind != "jev" {
		writeError(w, 404, "服务不存在")
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	input := mapBody(r)
	enabled, ok := input["enabled"].(bool)
	model := strings.TrimSpace(stringValue(input["model"]))
	keyMode := stringValue(input["key_mode"])
	key := strings.TrimSpace(stringValue(input["api_key"]))
	if !ok || model == "" || len(model) > 160 || strings.ContainsAny(model, "\r\n") || !containsString([]string{"keep", "replace", "environment"}, keyMode) {
		writeError(w, 400, "服务状态、模型或密钥操作无效")
		return
	}
	if keyMode == "replace" && (len(key) < 16 || len(key) > 2000 || strings.ContainsAny(key, "\r\n")) {
		writeError(w, 400, "新密钥必须为 16–2000 个字符且不能换行")
		return
	}
	var config any
	configName, keyName := "jev_config", "jev_api_key"
	provider := stringValue(input["provider"])
	if provider == "" {
		provider = jev.ProviderTypesafe
	}
	if provider != jev.ProviderTypesafe {
		writeError(w, 400, "Jev 仅支持 Typesafe 官方接口")
		return
	}
	previous, err := settings.ReadJevProvider(ctx, s.PB)
	if err != nil {
		writeError(w, 503, "当前 JEV 设置暂不可用")
		return
	}
	if keyMode == "keep" && previous.Provider != provider {
		writeError(w, 400, "切换提供商时，请明确选择新密钥或环境密钥，避免误发凭据")
		return
	}
	config = settings.Jev{Enabled: enabled, Provider: provider, Model: model}
	id := who(r)
	encoded, _ := json.Marshal(config)
	var encrypted string
	if keyMode == "replace" {
		var err error
		encrypted, err = settings.EncryptSecret(key)
		if err != nil {
			writeError(w, 503, "需先配置至少 32 个字符的服务端加密密钥")
			return
		}
	}
	err = s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		if err := settings.Put(ctx, tx, configName, string(encoded), stringValue(id.User["id"])); err != nil {
			return err
		}
		if keyMode == "replace" {
			if err := settings.Put(ctx, tx, keyName, encrypted, stringValue(id.User["id"])); err != nil {
				return err
			}
		}
		if keyMode == "environment" {
			if err := settings.Delete(ctx, tx, keyName); err != nil {
				return err
			}
		}
		if err := settings.Delete(ctx, tx, "ai_check_"+kind); err != nil {
			return err
		}
		_, err := tx.Create(ctx, "platform_audit_logs", map[string]any{"actor_id": id.User["id"], "actor_email": id.User["email"], "action": kind + ".configuration.updated", "target_type": "setting", "target_id": configName, "reason": "key_mode=" + keyMode, "status": 200})
		return err
	})
	if err != nil {
		writeError(w, 503, "AI 配置保存失败，未修改配置")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) adminAIServiceReset(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if !containsString([]string{"llm", "jev"}, kind) {
		writeError(w, 404, "服务不存在")
		return
	}
	names := []string{"jev_config", "jev_api_key", "ai_check_jev"}
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		for _, name := range names {
			if err := settings.Delete(ctx, tx, name); err != nil {
				return err
			}
		}
		_, err := tx.Create(ctx, "platform_audit_logs", map[string]any{"actor_id": id.User["id"], "actor_email": id.User["email"], "action": kind + ".environment.restored", "target_type": "setting", "target_id": kind, "status": 200})
		return err
	})
	if err != nil {
		writeError(w, 503, "恢复环境配置失败")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) adminAIServiceCheck(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if kind != "jev" {
		writeError(w, 404, "服务不存在")
		return
	}
	id := who(r)
	ctx, cancel := context.WithTimeout(r.Context(), 65*time.Second)
	defer cancel()
	start := time.Now()
	var err error
	_, err = s.evaluateJev(ctx, stringValue(id.Tenant["id"]), stringValue(id.User["id"]), "", map[string]any{"connection_check": true}, map[string]jevQuestion{"connectivity": {Type: "choice", Instructions: "Choose ok to confirm connectivity.", Criteria: map[string]string{"ok": "Connection check"}}})
	message := "模型请求成功"
	if err != nil {
		// Never persist provider bodies, prompts, keys or arbitrary network errors.
		message = "请求失败：请核对凭据、模型权限、配额与网络；未配置或已停用时请先保存有效配置"
		for _, safe := range []string{"LLM 服务已停用", "JEV 服务已停用", "企业尚未配置 AI 服务密钥", "JEV 尚未配置独立密钥，且无法复用 LLM 密钥", "工作区已达到今日 AI 请求预算", "工作区已达到今日 LLM 请求预算", "工作区已达到今日 JEV 请求预算", "AI 请求次数过多，请稍后重试"} {
			if err.Error() == safe {
				message = safe
				break
			}
		}
	}
	check := map[string]any{"ok": err == nil, "message": message, "checked_at": nowISO(), "latency_ms": time.Since(start).Milliseconds()}
	encoded, _ := json.Marshal(check)
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer saveCancel()
	saveErr := s.PB.Transaction(saveCtx, func(tx *pocketbase.Client) error {
		if e := settings.Put(saveCtx, tx, "ai_check_"+kind, string(encoded), stringValue(id.User["id"])); e != nil {
			return e
		}
		status := 200
		if err != nil {
			status = 502
		}
		_, e := tx.Create(saveCtx, "platform_audit_logs", map[string]any{"actor_id": id.User["id"], "actor_email": id.User["email"], "action": kind + ".connection.checked", "target_type": "setting", "target_id": kind, "status": status})
		return e
	})
	if saveErr != nil {
		writeError(w, 503, "连接检查结果未能保存，请重新读取配置")
		return
	}
	writeJSON(w, 200, check)
}
