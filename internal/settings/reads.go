package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tans/miao/internal/jev"
	"github.com/tans/miao/internal/pocketbase"
)

// bootstrapActive reports whether the group still runs on its environment
// bootstrap: no saved row and no recorded one-time import. A recorded import
// means deliberate clearing must not silently fall back to old values.
func bootstrapActive(ctx context.Context, pb *pocketbase.Client, group string) (bool, error) {
	var marker struct {
		Groups map[string]string `json:"groups"`
	}
	row, found, err := Find(ctx, pb, RowEnvImport)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	if err := decodeRow(row, &marker); err != nil {
		return false, err
	}
	_, marked := marker.Groups[group]
	return !marked, nil
}

// ReadRegistration resolves the effective registration policy.
func ReadRegistration(ctx context.Context, pb *pocketbase.Client) (Registration, error) {
	config := defaultRegistration()
	row, found, err := Find(ctx, pb, RowRegistration)
	if err != nil {
		return config, err
	}
	if found {
		if err := decodeRow(row, &config); err != nil {
			return config, err
		}
		// Rows saved before the verification policy moved into the database
		// keep their environment value until the one-time import patches them.
		if _, stated := jsonRaw(row)["require_email_verification"]; !stated {
			config.RequireEmailVerification = env("MIAO_REQUIRE_EMAIL_VERIFICATION") == "true"
		}
		if err := ValidateRegistration(config); err != nil {
			return config, err
		}
		return config, nil
	}
	bootstrapping, err := bootstrapActive(ctx, pb, "registration")
	if err != nil {
		return config, err
	}
	if !bootstrapping {
		return config, nil
	}
	config = Registration{
		Mode:                     env("MIAO_REGISTRATION_MODE"),
		AllowedEmailDomains:      splitListEnv("MIAO_ALLOWED_EMAIL_DOMAINS"),
		RequireEmailVerification: env("MIAO_REQUIRE_EMAIL_VERIFICATION") == "true",
	}
	if config.Mode == "" {
		config.Mode = "open"
	}
	if config.AllowedEmailDomains == nil {
		config.AllowedEmailDomains = []string{}
	}
	if err := ValidateRegistration(config); err != nil {
		return config, err
	}
	return config, nil
}

func jsonRaw(row map[string]any) map[string]any {
	raw := map[string]any{}
	_ = json.Unmarshal([]byte(stringValue(row["value"])), &raw)
	return raw
}

// ReadMail resolves the effective mail transport and decrypts the stored
// provider key. Decryption failures surface instead of masking the broken
// credential with the bootstrap environment value.
func ReadMail(ctx context.Context, pb *pocketbase.Client) (MailConfig, error) {
	config := MailConfig{Source: "none"}
	stored := Mail{}
	row, found, err := Find(ctx, pb, RowMail)
	if err != nil {
		return config, err
	}
	if found {
		if err := decodeRow(row, &stored); err != nil {
			return config, err
		}
		config.PublicURL, config.From, config.Source = stored.PublicURL, stored.From, "database"
	} else {
		bootstrapping, err := bootstrapActive(ctx, pb, "mail")
		if err != nil {
			return config, err
		}
		if bootstrapping {
			config.PublicURL, config.From = env("MIAO_PUBLIC_URL"), env("MIAO_MAIL_FROM")
			if config.PublicURL != "" || config.From != "" {
				config.Source = "environment"
			}
		}
	}
	keyRow, keyFound, err := Find(ctx, pb, RowMailKey)
	if err != nil {
		return config, err
	}
	switch {
	case keyFound:
		key, err := DecryptSecret(stringValue(keyRow["value"]))
		if err != nil {
			return config, fmt.Errorf("邮件服务密钥无法解密: %w", err)
		}
		config.Key, config.Source = key, "database"
	default:
		bootstrapping, err := bootstrapActive(ctx, pb, "mail")
		if err != nil {
			return config, err
		}
		if bootstrapping && env("RESEND_API_KEY") != "" {
			config.Key, config.Source = env("RESEND_API_KEY"), "environment"
		}
	}
	return config, nil
}

// ReadAdmins resolves the platform admin authorization list.
func ReadAdmins(ctx context.Context, pb *pocketbase.Client) (AdminList, error) {
	list := AdminList{Emails: []string{}, Source: "none"}
	row, found, err := Find(ctx, pb, RowAdmins)
	if err != nil {
		return list, err
	}
	if found {
		stored := Admins{}
		if err := decodeRow(row, &stored); err != nil {
			return list, err
		}
		list.Emails, list.Source = []string{}, "database"
		for _, email := range stored.Emails {
			if email = normalizeEmail(email); email != "" {
				list.Emails = append(list.Emails, email)
			}
		}
		return list, nil
	}
	bootstrapping, err := bootstrapActive(ctx, pb, "admins")
	if err != nil {
		return list, err
	}
	if bootstrapping && envSet("admins") {
		list.Emails, list.Source = splitListEnv("MIAO_ADMIN_EMAILS"), "environment"
	}
	return list, nil
}

// ReadBackup resolves the backup directory and retention policy. Both the
// backup CLI and the admin console read this one source.
func ReadBackup(ctx context.Context, pb *pocketbase.Client) (BackupPolicy, error) {
	policy := defaultBackup()
	row, found, err := Find(ctx, pb, RowBackup)
	if err != nil {
		return policy, err
	}
	if found {
		stored := Backup{}
		if err := decodeRow(row, &stored); err != nil {
			return policy, err
		}
		policy = BackupPolicy{Directory: stored.Directory, RetentionDays: stored.RetentionDays, Source: "database"}
		if policy.RetentionDays < 1 {
			policy.RetentionDays = 30
		}
		return policy, nil
	}
	bootstrapping, err := bootstrapActive(ctx, pb, "backup")
	if err != nil {
		return policy, err
	}
	if bootstrapping && envSet("backup") {
		policy.Source = "environment"
		policy.Directory = env("MIAO_BACKUP_DIR")
		if days, err := strconv.Atoi(strings.TrimSpace(env("MIAO_BACKUP_RETENTION_DAYS"))); err == nil && days >= 1 {
			policy.RetentionDays = days
		}
	}
	return policy, nil
}

// ReadBranding resolves the browser title used by the embedded application.
func ReadBranding(ctx context.Context, pb *pocketbase.Client) (Branding, error) {
	branding := defaultBranding()
	row, found, err := Find(ctx, pb, RowBranding)
	if err != nil {
		return branding, err
	}
	if found {
		if err := decodeRow(row, &branding); err != nil {
			return branding, err
		}
		branding.Title = strings.TrimSpace(branding.Title)
		if err := ValidateBranding(branding); err != nil {
			return defaultBranding(), err
		}
		return branding, nil
	}
	bootstrapping, err := bootstrapActive(ctx, pb, "branding")
	if err != nil {
		return branding, err
	}
	if bootstrapping && env("MIAO_TITLE") != "" {
		branding.Title = strings.TrimSpace(env("MIAO_TITLE"))
		if err := ValidateBranding(branding); err != nil {
			return defaultBranding(), err
		}
	}
	return branding, nil
}

// ReadLLMProvider resolves the saved or bootstrapped LLM provider settings
// without the credential.
func ReadLLMProvider(ctx context.Context, pb *pocketbase.Client) (LLM, error) {
	config := defaultLLM()
	row, found, err := Find(ctx, pb, RowLLM)
	if err != nil {
		return config, err
	}
	if found {
		if err := decodeRow(row, &config); err != nil {
			return config, err
		}
		if err := ValidateLLM(config); err != nil {
			return config, err
		}
		return config, nil
	}
	bootstrapping, err := bootstrapActive(ctx, pb, "llm")
	if err != nil {
		return config, err
	}
	if !bootstrapping {
		return config, nil
	}
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
		return config, err
	}
	return config, nil
}

// ReadJevProvider resolves the saved or bootstrapped Jev provider settings
// without the credential.
func ReadJevProvider(ctx context.Context, pb *pocketbase.Client) (Jev, error) {
	config := defaultJev()
	row, found, err := Find(ctx, pb, RowJev)
	if err != nil {
		return config, err
	}
	if found {
		if err := decodeRow(row, &config); err != nil {
			return config, err
		}
		if config.Provider == "" {
			config.Provider = jev.ProviderTypesafe
		}
		if err := ValidateJev(config); err != nil {
			return config, err
		}
		if config.Model == "" {
			config.Model = jevDefaultModel(config.Provider)
		}
		return config, nil
	}
	bootstrapping, err := bootstrapActive(ctx, pb, "jev")
	if err != nil {
		return config, err
	}
	if !bootstrapping {
		if config.Model == "" {
			config.Model = jevDefaultModel(config.Provider)
		}
		return config, nil
	}
	if value := env("MIAO_JEV_MODEL"); value != "" {
		config.Model = value
	}
	if config.Model == "" {
		config.Model = jevDefaultModel(config.Provider)
	}
	if err := ValidateJev(config); err != nil {
		return config, err
	}
	return config, nil
}

func jevDefaultModel(provider string) string {
	return jev.OfficialDefaultModel
}

// storedKey reads a credential row, distinguishing "not saved" from a row that
// exists but cannot be opened with the current environment key.
func storedKey(ctx context.Context, pb *pocketbase.Client, rowName string) (key string, rowFound bool, err error) {
	row, found, err := Find(ctx, pb, rowName)
	if err != nil || !found {
		return "", false, err
	}
	key, err = DecryptSecret(stringValue(row["value"]))
	if err != nil {
		return "", true, err
	}
	return key, true, nil
}

func bootstrapKey(ctx context.Context, pb *pocketbase.Client, group, envKey string) (string, error) {
	bootstrapping, err := bootstrapActive(ctx, pb, group)
	if err != nil || !bootstrapping {
		return "", err
	}
	return env(envKey), nil
}

// ReadLLMConfig resolves the effective LLM transport including the credential.
// A stored credential that fails to decrypt is an error, never a silent
// fallback to the bootstrap environment value.
func ReadLLMConfig(ctx context.Context, pb *pocketbase.Client) (AIConfig, error) {
	provider, err := ReadLLMProvider(ctx, pb)
	if err != nil {
		return AIConfig{}, err
	}
	config := AIConfig{Provider: provider.Provider, BaseURL: provider.BaseURL, Model: provider.Model, Enabled: provider.Enabled, Source: "none"}
	key, found, err := storedKey(ctx, pb, RowLLMKey)
	if err != nil {
		if found {
			return config, fmt.Errorf("LLM 服务密钥无法解密: %w", err)
		}
		return config, err
	}
	if found {
		config.Key, config.Source = key, "admin"
		return config, nil
	}
	if config.Key, err = bootstrapKey(ctx, pb, "llm", "AI_GATEWAY_API_KEY"); err != nil {
		return config, err
	}
	if config.Key != "" {
		config.Source = "environment"
	}
	return config, nil
}

// ReadJevConfig resolves the effective Jev transport including the credential.
// Jev always uses its own Typesafe credential; it never reuses the LLM
// service credential.
func ReadJevConfig(ctx context.Context, pb *pocketbase.Client) (JevConfig, error) {
	provider, err := ReadJevProvider(ctx, pb)
	if err != nil {
		return JevConfig{}, err
	}
	config := JevConfig{Provider: provider.Provider, Model: provider.Model, Enabled: provider.Enabled, Source: "none"}
	key, found, err := storedKey(ctx, pb, RowJevKey)
	if err != nil {
		if found {
			return config, fmt.Errorf("JEV 服务密钥无法解密: %w", err)
		}
		return config, err
	}
	if found {
		config.Key, config.Source = key, "admin"
		return config, nil
	}
	if config.Key, err = bootstrapKey(ctx, pb, "jev", "MIAO_JEV_API_KEY"); err != nil {
		return config, err
	}
	if config.Key != "" {
		config.Source = "environment"
		return config, nil
	}
	return config, nil
}
