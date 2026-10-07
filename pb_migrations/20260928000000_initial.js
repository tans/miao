// Pre-launch schema: define the final shape here instead of adding upgrade migrations.
const definitions = [
  {
    name: "tenants",
    fields: [
      { name: "name", type: "text", max: 160, required: true },
      { name: "owner_id", type: "text", max: 64, required: true },
      { name: "slug", type: "text", max: 160, required: true },
      { name: "ai_daily_limit", type: "number", min: 0 },
      { name: "ai_llm_daily_limit", type: "number", min: 0 },
      { name: "ai_jev_daily_limit", type: "number", min: 0 },
    ],
   },
  {
    name: "apps",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "name", type: "text", max: 160, required: true },
      { name: "description", type: "text", max: 4000 },
      { name: "archived", type: "bool" },
      { name: "restricted", type: "bool" },
      { name: "published_version_id", type: "text", max: 64 },
      { name: "creator_id", type: "text", max: 64 },
      { name: "business_context", type: "text", max: 16000 },
      { name: "context_revision", type: "number", min: 0 },
      { name: "public_publication", type: "json" },
      { name: "public_slug", type: "text", max: 64 },
      { name: "view_count", type: "number", min: 0 },
      { name: "harness_step_id", type: "text", max: 80 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_apps_public_slug ON apps (public_slug) WHERE public_slug != ''",
      "CREATE UNIQUE INDEX idx_apps_harness_step ON apps (harness_step_id) WHERE harness_step_id != ''",
    ],
   },
  {
    name: "app_collections",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "name", type: "text", max: 160, required: true },
      { name: "slug", type: "text", max: 40, required: true },
      { name: "pb_collection", type: "text", max: 80, required: true },
      { name: "fields", type: "json" },
    ],
   },
  {
    name: "tenant_members",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "role", type: "text", max: 16, required: true },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_tenant_members_tenant_user ON tenant_members (tenant_id, user_id)",
    ],
   },
  {
    name: "tenant_invites",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "email", type: "email", required: true },
      { name: "token_hash", type: "text", max: 64, required: true },
      { name: "expires_at", type: "text", max: 40, required: true },
      { name: "invited_by", type: "text", max: 64, required: true },
      { name: "status", type: "text", max: 16, required: true },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_tenant_invites_token_hash ON tenant_invites (token_hash)",
    ],
   },
  {
    name: "account_tokens",
    fields: [
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "token_hash", type: "text", max: 64, required: true },
      { name: "kind", type: "select", maxSelect: 1, required: true, values: ["verify", "reset"] },
      { name: "expires_at", type: "text", max: 40, required: true },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_account_tokens_hash ON account_tokens (token_hash)",
    ],
   },
  {
    name: "platform_settings",
    fields: [
      { name: "name", type: "text", max: 64, required: true },
      { name: "value", type: "text", max: 10000, required: true },
      { name: "updated_by", type: "text", max: 64 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_platform_settings_name ON platform_settings (name)",
    ],
   },
  {
    name: "ai_usage",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64 },
      { name: "status", type: "number", max: 599, min: 100 },
      { name: "input_tokens", type: "number", min: 0 },
      { name: "output_tokens", type: "number", min: 0 },
      { name: "kind", type: "text", max: 16 },
      { name: "provider", type: "text", max: 64 },
      { name: "model", type: "text", max: 160 },
      { name: "latency_ms", type: "number", min: 0 },
      { name: "input_known", type: "bool" },
      { name: "output_known", type: "bool" },
    ],
    indexes: [
      "CREATE INDEX idx_ai_usage_tenant_created ON ai_usage (tenant_id, created)",
      "CREATE INDEX idx_ai_usage_created ON ai_usage (created)",
    ],
   },
  {
    name: "audit_logs",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "actor_id", type: "text", max: 64, required: true },
      { name: "actor_email", type: "email", required: true },
      { name: "action", type: "text", max: 16, required: true },
      { name: "route", type: "text", max: 200, required: true },
      { name: "target_id", type: "text", max: 80 },
      { name: "status", type: "number", max: 399, min: 200 },
    ],
    indexes: [
      "CREATE INDEX idx_audit_logs_tenant_created ON audit_logs (tenant_id, created)",
    ],
   },
  {
    name: "app_members",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "role", type: "select", maxSelect: 1, required: true, values: ["viewer", "editor", "manager", "publisher"] },
      { name: "can_batch", type: "bool" },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_app_members_app_user ON app_members (app_id, user_id)",
    ],
   },
  {
    name: "platform_audit_logs",
    fields: [
      { name: "actor_id", type: "text", max: 64, required: true },
      { name: "actor_email", type: "email", required: true },
      { name: "action", type: "text", max: 80, required: true },
      { name: "target_type", type: "text", max: 32, required: true },
      { name: "target_id", type: "text", max: 64 },
      { name: "reason", type: "text", max: 500 },
      { name: "status", type: "number", max: 599, min: 100 },
    ],
    indexes: [
      "CREATE INDEX idx_platform_audit_created ON platform_audit_logs (created)",
      "CREATE INDEX idx_platform_audit_action_created ON platform_audit_logs (action, created)",
    ],
   },
  {
    name: "app_versions",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "version", type: "number", min: 1, required: true },
      { name: "definition", type: "json" },
      { name: "summary", type: "text", max: 1000 },
      { name: "created_by", type: "text", max: 64, required: true },
      { name: "published_at", type: "date" },
      { name: "based_on_version_id", type: "text", max: 64 },
      { name: "source", type: "json" },
      { name: "manifest", type: "json" },
      { name: "capabilities", type: "json" },
      { name: "harness_step_id", type: "text", max: 80 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_app_versions_app_version ON app_versions (app_id, version)",
      "CREATE INDEX idx_app_versions_tenant_app ON app_versions (tenant_id, app_id, version)",
      "CREATE UNIQUE INDEX idx_harness_ui_step ON app_versions (harness_step_id) WHERE harness_step_id != ''",
    ],
   },
  {
    name: "agent_threads",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64 },
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "title", type: "text", max: 160 },
    ],
   },
  {
    name: "agent_messages",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "thread_id", type: "text", max: 64, required: true },
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "role", type: "select", maxSelect: 1, required: true, values: ["user", "assistant"] },
      { name: "content", type: "text", max: 30000, required: true },
    ],
   },
  {
    name: "batch_jobs",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "plan", type: "json", required: true },
      { name: "status", type: "text", max: 24, required: true },
      { name: "result", type: "json" },
    ],
   },
  {
    name: "automation_rules",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "created_by", type: "text", max: 64, required: true },
      { name: "name", type: "text", max: 160, required: true },
      { name: "definition", type: "json", required: true },
      { name: "enabled", type: "bool" },
      { name: "pause_reason", type: "text", max: 200 },
    ],
   },
  {
    name: "automation_runs",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "rule_id", type: "text", max: 64, required: true },
      { name: "event_key", type: "text", max: 160, required: true },
      { name: "status", type: "text", max: 24, required: true },
      { name: "result", type: "json" },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_automation_run_event ON automation_runs (rule_id, event_key)",
    ],
   },
  {
    name: "automation_notifications",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "rule_id", type: "text", max: 64, required: true },
      { name: "event_key", type: "text", max: 160, required: true },
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "message", type: "text", max: 500, required: true },
      { name: "read", type: "bool" },
      { name: "run_id", type: "text", max: 64 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_automation_notification_event ON automation_notifications (rule_id, event_key, user_id)",
    ],
   },
  {
    name: "miao_tasks",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "created_by", type: "text", max: 64, required: true },
      { name: "name", type: "text", max: 160, required: true },
      { name: "revision", type: "number", min: 1 },
      { name: "definition", type: "json", maxSize: 2000000, required: true },
      { name: "status", type: "text", max: 24, required: true },
      { name: "pause_reason", type: "text", max: 300 },
      { name: "next_run_at", type: "text", max: 40 },
    ],
   },
  {
    name: "miao_runs",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "task_id", type: "text", max: 64, required: true },
      { name: "created_by", type: "text", max: 64, required: true },
      { name: "event_key", type: "text", max: 160, required: true },
      { name: "snapshot", type: "json", maxSize: 2000000, required: true },
      { name: "status", type: "text", max: 24, required: true },
      { name: "started_at", type: "text", max: 40 },
      { name: "finished_at", type: "text", max: 40 },
      { name: "output", type: "text", max: 30000 },
      { name: "error", type: "text", max: 1000 },
      { name: "pending", type: "json", maxSize: 2000000 },
      { name: "checkpoint", type: "json", maxSize: 6500000 },
      { name: "attempts", type: "number" },
      { name: "model_requests", type: "number" },
      { name: "delivery_status", type: "text", max: 24 },
      { name: "cancel_requested", type: "bool" },
      { name: "retry_count", type: "number", max: 2, min: 0 },
      { name: "harness_state", type: "text", max: 40 },
      { name: "harness_phase", type: "text", max: 40 },
      { name: "harness_sequence", type: "number", min: 0 },
      { name: "harness_version", type: "number", min: 0 },
      { name: "harness_candidate", type: "json", maxSize: 200000 },
      { name: "harness_authority", type: "json", maxSize: 200000 },
      { name: "harness_result", type: "json", maxSize: 600000 },
      { name: "harness_loop", type: "json", maxSize: 6500000 },
      { name: "harness_storage_revision", type: "number", min: 0 },
      { name: "harness_lease_owner", type: "text", max: 80 },
      { name: "harness_lease_expires_at", type: "text", max: 40 },
      { name: "harness_active_started_at", type: "text", max: 40 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_miao_run_event ON miao_runs (task_id, event_key)",
      "CREATE INDEX idx_miao_run_status ON miao_runs (status, created)",
    ],
   },
  {
    name: "miao_actions",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "run_id", type: "text", max: 64, required: true },
      { name: "action_key", type: "text", max: 64, required: true },
      { name: "tool", type: "text", max: 80, required: true },
      { name: "input", type: "json", maxSize: 2000000, required: true },
      { name: "status", type: "text", max: 24, required: true },
      { name: "result", type: "json", maxSize: 2000000 },
      { name: "evidence", type: "json", maxSize: 2000000 },
      { name: "approved_by", type: "text", max: 64 },
      { name: "approved_at", type: "text", max: 40 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_miao_action_key ON miao_actions (run_id, action_key)",
    ],
   },
  {
    name: "miao_runtime_locks",
    fields: [
      { name: "name", type: "text", max: 64, required: true },
      { name: "owner", type: "text", max: 64, required: true },
      { name: "expires_at", type: "text", max: 40, required: true },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_miao_runtime_lock ON miao_runtime_locks (name)",
    ],
   },
  {
    name: "miao_run_attempts",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "run_id", type: "text", max: 64, required: true },
      { name: "sequence", type: "number", min: 1, required: true },
      { name: "status", type: "text", max: 24, required: true },
      { name: "started_at", type: "text", max: 40 },
      { name: "finished_at", type: "text", max: 40 },
      { name: "output", type: "text", max: 30000 },
      { name: "error", type: "text", max: 1000 },
      { name: "model_requests", type: "number" },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_miao_attempt_sequence ON miao_run_attempts (run_id, sequence)",
    ],
   },
  {
    name: "agent_sessions",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "scope", type: "text", max: 100000, required: true },
      { name: "revision", type: "number", min: 0 },
      { name: "checkpoint", type: "text", max: 6000000 },
      { name: "messages", type: "json", maxSize: 300000 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_agent_sessions_owner ON agent_sessions (tenant_id, user_id)",
    ],
   },
  {
    name: "app_files",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "name", type: "text", max: 120, required: true },
      { name: "file", type: "file", maxSelect: 1, maxSize: 5242880, protected: true, required: true },
    ],
   },
  {
    name: "miao_record_changes",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "table", type: "text", max: 64, required: true },
      { name: "record_id", type: "text", max: 64, required: true },
      { name: "actor_id", type: "text", max: 64 },
      { name: "source", type: "text", max: 32 },
      { name: "before", type: "json" },
      { name: "after", type: "json" },
      { name: "event", type: "text", max: 16 },
      { name: "automation_processed", type: "bool" },
    ],
    indexes: [
      "CREATE INDEX idx_record_changes_app ON miao_record_changes (tenant_id, app_id, record_id)",
      "CREATE INDEX idx_record_changes_automation ON miao_record_changes (automation_processed, created)",
    ],
   },
  {
    name: "business_actions",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "created_by", type: "text", max: 64, required: true },
      { name: "name", type: "text", max: 160, required: true },
      { name: "description", type: "text", max: 1000 },
      { name: "definition", type: "json", required: true },
      { name: "status", type: "select", maxSelect: 1, required: true, values: ["draft", "enabled", "paused", "archived"] },
      { name: "revision", type: "number", min: 1, required: true },
      { name: "pause_reason", type: "text", max: 300 },
    ],
    indexes: [
      "CREATE INDEX idx_business_action_app ON business_actions (tenant_id, app_id, status)",
    ],
   },
  {
    name: "business_action_runs",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "action_id", type: "text", max: 64, required: true },
      { name: "revision", type: "number", min: 1, required: true },
      { name: "idempotency_key", type: "text", max: 160, required: true },
      { name: "status", type: "select", maxSelect: 1, required: true, values: ["completed", "failed"] },
      { name: "result", type: "json" },
      { name: "error", type: "text", max: 1000 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_business_action_run_key ON business_action_runs (action_id, idempotency_key)",
    ],
   },
  {
    name: "workflows",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "created_by", type: "text", max: 64, required: true },
      { name: "name", type: "text", max: 160, required: true },
      { name: "description", type: "text", max: 1000 },
      { name: "definition", type: "json", required: true },
      { name: "status", type: "select", maxSelect: 1, required: true, values: ["draft", "enabled", "paused", "archived"] },
      { name: "revision", type: "number", min: 1, required: true },
      { name: "pause_reason", type: "text", max: 300 },
    ],
    indexes: [
      "CREATE INDEX idx_workflow_app ON workflows (tenant_id, app_id, status)",
    ],
   },
  {
    name: "workflow_runs",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "workflow_id", type: "text", max: 64, required: true },
      { name: "revision", type: "number", min: 1, required: true },
      { name: "transition_id", type: "text", max: 64, required: true },
      { name: "record_id", type: "text", max: 64, required: true },
      { name: "idempotency_key", type: "text", max: 160, required: true },
      { name: "status", type: "select", maxSelect: 1, required: true, values: ["completed", "failed"] },
      { name: "result", type: "json" },
      { name: "error", type: "text", max: 1000 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_workflow_run_key ON workflow_runs (workflow_id, transition_id, record_id, idempotency_key)",
    ],
   },
  {
    name: "connectors",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "created_by", type: "text", max: 64, required: true },
      { name: "name", type: "text", max: 160, required: true },
      { name: "description", type: "text", max: 1000 },
      { name: "definition", type: "json", required: true },
      { name: "status", type: "select", maxSelect: 1, required: true, values: ["draft", "enabled", "paused", "archived"] },
      { name: "revision", type: "number", min: 1, required: true },
      { name: "pause_reason", type: "text", max: 300 },
    ],
    indexes: [
      "CREATE INDEX idx_connector_app ON connectors (tenant_id, app_id, status)",
    ],
   },
  {
    name: "connector_runs",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "connector_id", type: "text", max: 64, required: true },
      { name: "revision", type: "number", min: 1, required: true },
      { name: "idempotency_key", type: "text", max: 160, required: true },
      { name: "status", type: "select", maxSelect: 1, required: true, values: ["completed", "failed"] },
      { name: "result", type: "json" },
      { name: "error", type: "text", max: 1000 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_connector_run_key ON connector_runs (connector_id, idempotency_key)",
    ],
   },
  {
    name: "miao_harness_runs",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64 },
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "prompt", type: "text", max: 12000, required: true },
      { name: "input", type: "json", maxSize: 1000000 },
      { name: "state", type: "text", max: 40, required: true },
      { name: "phase", type: "text", max: 40, required: true },
      { name: "sequence", type: "number", min: 0 },
      { name: "version", type: "number", min: 0 },
      { name: "candidate", type: "json", maxSize: 1000000 },
      { name: "authority", type: "json", maxSize: 1000000 },
      { name: "result", type: "json", maxSize: 1000000 },
      { name: "error", type: "text", max: 2000 },
      { name: "cancel_requested", type: "bool" },
      { name: "loop", type: "json", maxSize: 6500000 },
      { name: "storage_revision", type: "number", min: 0 },
      { name: "lease_owner", type: "text", max: 80 },
      { name: "lease_expires_at", type: "text", max: 40 },
      { name: "active_started_at", type: "text", max: 40 },
    ],
    indexes: [
      "CREATE INDEX idx_harness_runs_owner ON miao_harness_runs (tenant_id, user_id, created)",
      "CREATE INDEX idx_harness_runs_state ON miao_harness_runs (state, updated)",
    ],
   },
  {
    name: "miao_harness_events",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64 },
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "run_id", type: "text", max: 64, required: true },
      { name: "sequence", type: "number", min: 1 },
      { name: "event_type", type: "text", max: 80, required: true },
      { name: "data", type: "json", maxSize: 1000000 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_harness_event_sequence ON miao_harness_events (run_id, sequence)",
      "CREATE INDEX idx_harness_events_run ON miao_harness_events (run_id, sequence)",
    ],
   },
  {
    name: "collection_scripts",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "created_by", type: "text", max: 64, required: true },
      { name: "name", type: "text", max: 160, required: true },
      { name: "revision", type: "number", min: 1 },
      { name: "definition", type: "json", maxSize: 500000, required: true },
      { name: "status", type: "text", max: 24, required: true },
      { name: "pause_reason", type: "text", max: 300 },
      { name: "next_run_at", type: "text", max: 40 },
    ],
    indexes: [
      "CREATE INDEX idx_collection_script_app ON collection_scripts (tenant_id, app_id, status)",
    ],
   },
  {
    name: "collection_script_versions",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "script_id", type: "text", max: 64, required: true },
      { name: "version", type: "number", min: 1 },
      { name: "created_by", type: "text", max: 64, required: true },
      { name: "definition", type: "json", maxSize: 500000, required: true },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_collection_script_version ON collection_script_versions (script_id, version)",
    ],
   },
  {
    name: "collection_script_runs",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "script_id", type: "text", max: 64, required: true },
      { name: "version", type: "number", min: 1 },
      { name: "created_by", type: "text", max: 64, required: true },
      { name: "event_key", type: "text", max: 180, required: true },
      { name: "mode", type: "text", max: 16, required: true },
      { name: "status", type: "text", max: 24, required: true },
      { name: "snapshot", type: "json", maxSize: 500000, required: true },
      { name: "started_at", type: "text", max: 40 },
      { name: "finished_at", type: "text", max: 40 },
      { name: "counts", type: "json", maxSize: 2000000 },
      { name: "errors", type: "json", maxSize: 100000 },
      { name: "result", type: "json", maxSize: 600000 },
      { name: "error", type: "text", max: 1000 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_collection_script_run_event ON collection_script_runs (script_id, event_key)",
      "CREATE INDEX idx_collection_script_run_status ON collection_script_runs (tenant_id, app_id, status, created)",
    ],
   },
  {
    name: "collection_script_items",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "script_id", type: "text", max: 64, required: true },
      { name: "dedup_key", type: "text", max: 180, required: true },
      { name: "status", type: "text", max: 24, required: true },
      { name: "target_record_id", type: "text", max: 64 },
      { name: "last_run_id", type: "text", max: 64 },
      { name: "source", type: "json", maxSize: 2000000 },
      { name: "error", type: "text", max: 1000 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_collection_script_item_key ON collection_script_items (script_id, dedup_key)",
      "CREATE INDEX idx_collection_script_item_status ON collection_script_items (script_id, status)",
    ],
   },
  {
    name: "collection_script_notifications",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "script_id", type: "text", max: 64, required: true },
      { name: "run_id", type: "text", max: 64, required: true },
      { name: "item_id", type: "text", max: 64, required: true },
      { name: "recipient_id", type: "text", max: 64, required: true },
      { name: "event_key", type: "text", max: 220, required: true },
      { name: "message", type: "text", max: 1000, required: true },
      { name: "status", type: "text", max: 24, required: true },
      { name: "error", type: "text", max: 1000 },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_collection_script_notification_key ON collection_script_notifications (script_id, item_id, recipient_id)",
      "CREATE INDEX idx_collection_script_notification_pending ON collection_script_notifications (status, created)",
    ],
   },
  {
    name: "app_backend_plans",
    fields: [
      { name: "tenant_id", type: "text", max: 64, required: true },
      { name: "app_id", type: "text", max: 64, required: true },
      { name: "user_id", type: "text", max: 64, required: true },
      { name: "status", type: "text", max: 24, required: true },
      { name: "revision", type: "number", min: 1 },
      { name: "baseline_hash", type: "text", max: 64, required: true },
      { name: "expires_at", type: "text", max: 40, required: true },
      { name: "operations", type: "json", maxSize: 1000000, required: true },
      { name: "dependency_order", type: "json", maxSize: 100000, required: true },
      { name: "impact", type: "json", maxSize: 100000, required: true },
      { name: "receipt", type: "json", maxSize: 1000000, required: true },
      { name: "harness_step_id", type: "text", max: 80 },
    ],
    indexes: [
      "CREATE INDEX idx_backend_plans_app ON app_backend_plans (tenant_id, app_id, created)",
      "CREATE INDEX idx_backend_plans_owner ON app_backend_plans (tenant_id, user_id, status)",
      "CREATE UNIQUE INDEX idx_app_backend_plans_harness_step ON app_backend_plans (harness_step_id) WHERE harness_step_id != ''",
    ],
   },
]

migrate((app) => {
  // PocketBase owns users; only apply MIAO-specific account fields and file protection.
  const users = app.findCollectionByNameOrId("users")
  users.fields.add(new BoolField({ name: "disabled" }))
  users.fields.getByName("avatar").protected = true
  app.save(users)

  for (const definition of definitions) {
    app.save(new Collection({
      type: "base",
      ...definition,
      listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
      fields: [
        { name: "created", type: "autodate", onCreate: true, system: true  },
        { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true  },
        ...definition.fields,
      ],
    }))
  }

  const email = $os.getenv("POCKETBASE_SUPERUSER_EMAIL")
  const password = $os.getenv("POCKETBASE_SUPERUSER_PASSWORD")
  if (email && password) {
    const record = new Record(app.findCollectionByNameOrId("_superusers"))
    record.set("email", email)
    record.set("password", password)
    app.save(record)
  }
}, (app) => {
  for (const definition of [...definitions].reverse()) {
    app.delete(app.findCollectionByNameOrId(definition.name))
  }
  const users = app.findCollectionByNameOrId("users")
  users.fields.removeByName("disabled")
  users.fields.getByName("avatar").protected = false
  app.save(users)
})
