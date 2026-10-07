package settings

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/tans/miao/internal/pocketbase"
)

// ImportResult records what the one-time bootstrap import did per group.
type ImportResult struct {
	Group  string `json:"group"`
	Result string `json:"result"`
}

type importState struct {
	Groups map[string]string `json:"groups"`
}

// ImportEnvironment moves bootstrap environment variables into the database
// exactly once per settings group: groups with a saved row keep it ("database"),
// groups recorded in the env_import marker are never re-read ("imported"), and
// only groups that still have neither are imported from the environment. The
// result is recorded so that deliberately cleared settings stay cleared and
// later environment changes never override console edits. Failures leave the
// group unmarked so the next startup retries; reads keep using the environment
// bootstrap until then.
func ImportEnvironment(ctx context.Context, pb *pocketbase.Client) []ImportResult {
	results := []ImportResult{}
	for _, group := range []string{"registration", "mail", "llm", "jev", "admins", "backup"} {
		result, err := importGroup(ctx, pb, group)
		if err != nil {
			result = "import failed: " + err.Error()
			slog.Warn("settings bootstrap import failed", "group", group, "error", err)
		}
		results = append(results, ImportResult{Group: group, Result: result})
	}
	return results
}

func importGroup(ctx context.Context, pb *pocketbase.Client, group string) (string, error) {
	switch group {
	case "registration":
		return importRegistration(ctx, pb)
	case "mail":
		return importMail(ctx, pb)
	case "llm":
		return importLLM(ctx, pb)
	case "jev":
		return importJev(ctx, pb)
	case "admins":
		return importAdmins(ctx, pb)
	case "backup":
		return importBackup(ctx, pb)
	}
	return "skipped", nil
}

// groupRowExists reports whether any row of a settings group is already saved;
// such groups keep their database values and are never re-imported.
func groupRowExists(ctx context.Context, pb *pocketbase.Client, names ...string) (bool, error) {
	for _, name := range names {
		_, found, err := Find(ctx, pb, name)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

func importRegistration(ctx context.Context, pb *pocketbase.Client) (string, error) {
	row, found, err := Find(ctx, pb, RowRegistration)
	if err != nil {
		return "", err
	}
	if found {
		// Patch rows saved before the verification policy moved into the
		// database, then record the group as imported.
		raw := map[string]any{}
		_ = json.Unmarshal([]byte(stringValue(row["value"])), &raw)
		if _, stated := raw["require_email_verification"]; stated || !envSet("registration") {
			return "database", nil
		}
		config := defaultRegistration()
		if err := decodeRow(row, &config); err != nil {
			return "", err
		}
		config.RequireEmailVerification = env("MIAO_REQUIRE_EMAIL_VERIFICATION") == "true"
		return finishImport(ctx, pb, "registration", func(tx *pocketbase.Client) error {
			return Put(ctx, tx, RowRegistration, encode(config), "system")
		}, "环境验证策略并入已保存的注册设置")
	}
	if !envSet("registration") {
		return "empty", nil
	}
	config := Registration{
		Mode:                     env("MIAO_REGISTRATION_MODE"),
		AllowedEmailDomains:      splitListEnv("MIAO_ALLOWED_EMAIL_DOMAINS"),
		RequireEmailVerification: env("MIAO_REQUIRE_EMAIL_VERIFICATION") == "true",
	}
	if config.Mode == "" {
		config.Mode = "open"
	}
	if err := ValidateRegistration(config); err != nil {
		return "", err
	}
	return finishImport(ctx, pb, "registration", func(tx *pocketbase.Client) error {
		return Put(ctx, tx, RowRegistration, encode(config), "system")
	}, "注册策略自环境导入")
}

func importMail(ctx context.Context, pb *pocketbase.Client) (string, error) {
	if found, err := groupRowExists(ctx, pb, RowMail, RowMailKey); err != nil || found {
		return "database", err
	}
	if !envSet("mail") {
		return "empty", nil
	}
	config := Mail{PublicURL: strings.TrimRight(env("MIAO_PUBLIC_URL"), "/"), From: env("MIAO_MAIL_FROM")}
	if err := ValidateMail(config); err != nil {
		return "", err
	}
	sealed := ""
	if env("RESEND_API_KEY") != "" {
		var err error
		if sealed, err = EncryptSecret(env("RESEND_API_KEY")); err != nil {
			return "", err
		}
	}
	return finishImport(ctx, pb, "mail", func(tx *pocketbase.Client) error {
		if err := Put(ctx, tx, RowMail, encode(config), "system"); err != nil {
			return err
		}
		if sealed != "" {
			return Put(ctx, tx, RowMailKey, sealed, "system")
		}
		return nil
	}, "邮件服务配置自环境导入")
}

func importLLM(ctx context.Context, pb *pocketbase.Client) (string, error) {
	if found, err := groupRowExists(ctx, pb, RowLLM, RowLLMKey); err != nil || found {
		return "database", err
	}
	if !envSet("llm") {
		return "empty", nil
	}
	config := defaultLLM()
	if value := env("MIAO_AI_PROVIDER"); value != "" {
		config.Provider = value
	}
	if value := env("MIAO_AI_BASE_URL"); value != "" {
		config.BaseURL = strings.TrimRight(value, "/")
	}
	if value := env("MIAO_AI_MODEL"); value != "" {
		config.Model = value
	}
	if config.Provider == "vercel" {
		config.BaseURL = GatewayBase + "/v1"
	}
	if err := ValidateLLM(config); err != nil {
		return "", err
	}
	sealed := ""
	if env("AI_GATEWAY_API_KEY") != "" {
		var err error
		if sealed, err = EncryptSecret(env("AI_GATEWAY_API_KEY")); err != nil {
			return "", err
		}
	}
	return finishImport(ctx, pb, "llm", func(tx *pocketbase.Client) error {
		if err := Put(ctx, tx, RowLLM, encode(config), "system"); err != nil {
			return err
		}
		if sealed != "" {
			return Put(ctx, tx, RowLLMKey, sealed, "system")
		}
		return nil
	}, "LLM 服务配置自环境导入")
}

func importJev(ctx context.Context, pb *pocketbase.Client) (string, error) {
	if found, err := groupRowExists(ctx, pb, RowJev, RowJevKey); err != nil || found {
		return "database", err
	}
	if !envSet("jev") {
		return "empty", nil
	}
	config := defaultJev()
	if value := env("MIAO_JEV_PROVIDER"); value != "" {
		config.Provider = value
	}
	if value := env("MIAO_JEV_MODEL"); value != "" {
		config.Model = value
	}
	if err := ValidateJev(config); err != nil {
		return "", err
	}
	sealed := ""
	if env("MIAO_JEV_API_KEY") != "" {
		var err error
		if sealed, err = EncryptSecret(env("MIAO_JEV_API_KEY")); err != nil {
			return "", err
		}
	}
	return finishImport(ctx, pb, "jev", func(tx *pocketbase.Client) error {
		if err := Put(ctx, tx, RowJev, encode(config), "system"); err != nil {
			return err
		}
		if sealed != "" {
			return Put(ctx, tx, RowJevKey, sealed, "system")
		}
		return nil
	}, "JEV 服务配置自环境导入")
}

func importAdmins(ctx context.Context, pb *pocketbase.Client) (string, error) {
	if found, err := groupRowExists(ctx, pb, RowAdmins); err != nil || found {
		return "database", err
	}
	if !envSet("admins") {
		return "empty", nil
	}
	emails, err := NormalizeAdminEmails(splitListEnv("MIAO_ADMIN_EMAILS"))
	if err != nil {
		return "", err
	}
	return finishImport(ctx, pb, "admins", func(tx *pocketbase.Client) error {
		return Put(ctx, tx, RowAdmins, encode(Admins{Emails: emails}), "system")
	}, "平台管理员名单自环境导入")
}

func importBackup(ctx context.Context, pb *pocketbase.Client) (string, error) {
	if found, err := groupRowExists(ctx, pb, RowBackup); err != nil || found {
		return "database", err
	}
	if !envSet("backup") {
		return "empty", nil
	}
	config := Backup{Directory: env("MIAO_BACKUP_DIR"), RetentionDays: 30}
	if days, err := strconv.Atoi(strings.TrimSpace(env("MIAO_BACKUP_RETENTION_DAYS"))); err == nil && days >= 1 {
		config.RetentionDays = days
	}
	if err := ValidateBackup(config); err != nil {
		return "", err
	}
	return finishImport(ctx, pb, "backup", func(tx *pocketbase.Client) error {
		return Put(ctx, tx, RowBackup, encode(config), "system")
	}, "备份策略自环境导入")
}

// finishImport runs the row writes and the marker plus audit entry in one
// transaction, so an aborted import leaves the group unmarked and retryable.
func finishImport(ctx context.Context, pb *pocketbase.Client, group string, write func(tx *pocketbase.Client) error, reason string) (string, error) {
	err := pb.Transaction(ctx, func(tx *pocketbase.Client) error {
		if err := write(tx); err != nil {
			return err
		}
		state := importState{Groups: map[string]string{}}
		row, found, err := Find(ctx, tx, RowEnvImport)
		if err != nil {
			return err
		}
		if found {
			if err := decodeRow(row, &state); err != nil {
				return err
			}
		}
		if state.Groups == nil {
			state.Groups = map[string]string{}
		}
		state.Groups[group] = time.Now().UTC().Format(time.RFC3339)
		if err := Put(ctx, tx, RowEnvImport, encode(state), "system"); err != nil {
			return err
		}
		_, err = tx.Create(ctx, "platform_audit_logs", map[string]any{
			"actor_id": "system", "actor_email": "system@miao.local", "action": "settings.environment.imported",
			"target_type": "setting", "target_id": group, "reason": reason, "status": 200,
		})
		return err
	})
	if err != nil {
		return "", err
	}
	return "imported", nil
}

func encode(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
