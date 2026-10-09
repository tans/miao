// Converge data directories created before the initial migration: directories
// upgraded from the pre-refactor service keep their original tables, the
// initial migration only creates collections that do not exist yet, so those
// tables never gained the columns and collections the current code writes.
// Create the missing collections and add the missing fields and indexes.
// Every step is idempotent; a clean directory passes through unchanged.
// Definitions mirror 20260928000000_initial.js.
const definitions = [
  {
    "name": "app_backend_plans",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "user_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "status",
        "type": "text",
        "max": 24,
        "required": true
      },
      {
        "name": "revision",
        "type": "number",
        "min": 1
      },
      {
        "name": "baseline_hash",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "expires_at",
        "type": "text",
        "max": 40,
        "required": true
      },
      {
        "name": "operations",
        "type": "json",
        "maxSize": 1000000,
        "required": true
      },
      {
        "name": "dependency_order",
        "type": "json",
        "maxSize": 100000,
        "required": true
      },
      {
        "name": "impact",
        "type": "json",
        "maxSize": 100000,
        "required": true
      },
      {
        "name": "receipt",
        "type": "json",
        "maxSize": 1000000,
        "required": true
      },
      {
        "name": "harness_step_id",
        "type": "text",
        "max": 80
      }
    ],
    "indexes": [
      "CREATE INDEX idx_backend_plans_app ON app_backend_plans (tenant_id, app_id, created)",
      "CREATE INDEX idx_backend_plans_owner ON app_backend_plans (tenant_id, user_id, status)",
      "CREATE UNIQUE INDEX idx_app_backend_plans_harness_step ON app_backend_plans (harness_step_id) WHERE harness_step_id != ''"
    ]
  },
  {
    "name": "business_actions",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "created_by",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "name",
        "type": "text",
        "max": 160,
        "required": true
      },
      {
        "name": "description",
        "type": "text",
        "max": 1000
      },
      {
        "name": "definition",
        "type": "json",
        "required": true
      },
      {
        "name": "status",
        "type": "select",
        "maxSelect": 1,
        "required": true,
        "values": [
          "draft",
          "enabled",
          "paused",
          "archived"
        ]
      },
      {
        "name": "revision",
        "type": "number",
        "min": 1,
        "required": true
      },
      {
        "name": "pause_reason",
        "type": "text",
        "max": 300
      }
    ],
    "indexes": [
      "CREATE INDEX idx_business_action_app ON business_actions (tenant_id, app_id, status)"
    ]
  },
  {
    "name": "business_action_runs",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "action_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "revision",
        "type": "number",
        "min": 1,
        "required": true
      },
      {
        "name": "idempotency_key",
        "type": "text",
        "max": 160,
        "required": true
      },
      {
        "name": "status",
        "type": "select",
        "maxSelect": 1,
        "required": true,
        "values": [
          "completed",
          "failed"
        ]
      },
      {
        "name": "result",
        "type": "json"
      },
      {
        "name": "error",
        "type": "text",
        "max": 1000
      }
    ],
    "indexes": [
      "CREATE UNIQUE INDEX idx_business_action_run_key ON business_action_runs (action_id, idempotency_key)"
    ]
  },
  {
    "name": "collection_scripts",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "created_by",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "name",
        "type": "text",
        "max": 160,
        "required": true
      },
      {
        "name": "revision",
        "type": "number",
        "min": 1
      },
      {
        "name": "definition",
        "type": "json",
        "maxSize": 500000,
        "required": true
      },
      {
        "name": "status",
        "type": "text",
        "max": 24,
        "required": true
      },
      {
        "name": "pause_reason",
        "type": "text",
        "max": 300
      },
      {
        "name": "next_run_at",
        "type": "text",
        "max": 40
      }
    ],
    "indexes": [
      "CREATE INDEX idx_collection_script_app ON collection_scripts (tenant_id, app_id, status)"
    ]
  },
  {
    "name": "collection_script_versions",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "script_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "version",
        "type": "number",
        "min": 1
      },
      {
        "name": "created_by",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "definition",
        "type": "json",
        "maxSize": 500000,
        "required": true
      }
    ],
    "indexes": [
      "CREATE UNIQUE INDEX idx_collection_script_version ON collection_script_versions (script_id, version)"
    ]
  },
  {
    "name": "collection_script_runs",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "script_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "version",
        "type": "number",
        "min": 1
      },
      {
        "name": "created_by",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "event_key",
        "type": "text",
        "max": 180,
        "required": true
      },
      {
        "name": "mode",
        "type": "text",
        "max": 16,
        "required": true
      },
      {
        "name": "status",
        "type": "text",
        "max": 24,
        "required": true
      },
      {
        "name": "snapshot",
        "type": "json",
        "maxSize": 500000,
        "required": true
      },
      {
        "name": "started_at",
        "type": "text",
        "max": 40
      },
      {
        "name": "finished_at",
        "type": "text",
        "max": 40
      },
      {
        "name": "counts",
        "type": "json",
        "maxSize": 2000000
      },
      {
        "name": "errors",
        "type": "json",
        "maxSize": 100000
      },
      {
        "name": "result",
        "type": "json",
        "maxSize": 600000
      },
      {
        "name": "error",
        "type": "text",
        "max": 1000
      }
    ],
    "indexes": [
      "CREATE UNIQUE INDEX idx_collection_script_run_event ON collection_script_runs (script_id, event_key)",
      "CREATE INDEX idx_collection_script_run_status ON collection_script_runs (tenant_id, app_id, status, created)"
    ]
  },
  {
    "name": "collection_script_items",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "script_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "dedup_key",
        "type": "text",
        "max": 180,
        "required": true
      },
      {
        "name": "status",
        "type": "text",
        "max": 24,
        "required": true
      },
      {
        "name": "target_record_id",
        "type": "text",
        "max": 64
      },
      {
        "name": "last_run_id",
        "type": "text",
        "max": 64
      },
      {
        "name": "source",
        "type": "json",
        "maxSize": 2000000
      },
      {
        "name": "error",
        "type": "text",
        "max": 1000
      }
    ],
    "indexes": [
      "CREATE UNIQUE INDEX idx_collection_script_item_key ON collection_script_items (script_id, dedup_key)",
      "CREATE INDEX idx_collection_script_item_status ON collection_script_items (script_id, status)"
    ]
  },
  {
    "name": "collection_script_notifications",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "script_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "run_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "item_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "recipient_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "event_key",
        "type": "text",
        "max": 220,
        "required": true
      },
      {
        "name": "message",
        "type": "text",
        "max": 1000,
        "required": true
      },
      {
        "name": "status",
        "type": "text",
        "max": 24,
        "required": true
      },
      {
        "name": "error",
        "type": "text",
        "max": 1000
      }
    ],
    "indexes": [
      "CREATE UNIQUE INDEX idx_collection_script_notification_key ON collection_script_notifications (script_id, item_id, recipient_id)",
      "CREATE INDEX idx_collection_script_notification_pending ON collection_script_notifications (status, created)"
    ]
  },
  {
    "name": "connectors",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "created_by",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "name",
        "type": "text",
        "max": 160,
        "required": true
      },
      {
        "name": "description",
        "type": "text",
        "max": 1000
      },
      {
        "name": "definition",
        "type": "json",
        "required": true
      },
      {
        "name": "status",
        "type": "select",
        "maxSelect": 1,
        "required": true,
        "values": [
          "draft",
          "enabled",
          "paused",
          "archived"
        ]
      },
      {
        "name": "revision",
        "type": "number",
        "min": 1,
        "required": true
      },
      {
        "name": "pause_reason",
        "type": "text",
        "max": 300
      }
    ],
    "indexes": [
      "CREATE INDEX idx_connector_app ON connectors (tenant_id, app_id, status)"
    ]
  },
  {
    "name": "connector_runs",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "connector_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "revision",
        "type": "number",
        "min": 1,
        "required": true
      },
      {
        "name": "idempotency_key",
        "type": "text",
        "max": 160,
        "required": true
      },
      {
        "name": "status",
        "type": "select",
        "maxSelect": 1,
        "required": true,
        "values": [
          "completed",
          "failed"
        ]
      },
      {
        "name": "result",
        "type": "json"
      },
      {
        "name": "error",
        "type": "text",
        "max": 1000
      }
    ],
    "indexes": [
      "CREATE UNIQUE INDEX idx_connector_run_key ON connector_runs (connector_id, idempotency_key)"
    ]
  },
  {
    "name": "workflows",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "created_by",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "name",
        "type": "text",
        "max": 160,
        "required": true
      },
      {
        "name": "description",
        "type": "text",
        "max": 1000
      },
      {
        "name": "definition",
        "type": "json",
        "required": true
      },
      {
        "name": "status",
        "type": "select",
        "maxSelect": 1,
        "required": true,
        "values": [
          "draft",
          "enabled",
          "paused",
          "archived"
        ]
      },
      {
        "name": "revision",
        "type": "number",
        "min": 1,
        "required": true
      },
      {
        "name": "pause_reason",
        "type": "text",
        "max": 300
      }
    ],
    "indexes": [
      "CREATE INDEX idx_workflow_app ON workflows (tenant_id, app_id, status)"
    ]
  },
  {
    "name": "workflow_runs",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "workflow_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "revision",
        "type": "number",
        "min": 1,
        "required": true
      },
      {
        "name": "transition_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "record_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "idempotency_key",
        "type": "text",
        "max": 160,
        "required": true
      },
      {
        "name": "status",
        "type": "select",
        "maxSelect": 1,
        "required": true,
        "values": [
          "completed",
          "failed"
        ]
      },
      {
        "name": "result",
        "type": "json"
      },
      {
        "name": "error",
        "type": "text",
        "max": 1000
      }
    ],
    "indexes": [
      "CREATE UNIQUE INDEX idx_workflow_run_key ON workflow_runs (workflow_id, transition_id, record_id, idempotency_key)"
    ]
  },
  {
    "name": "apps",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "name",
        "type": "text",
        "max": 160,
        "required": true
      },
      {
        "name": "description",
        "type": "text",
        "max": 4000
      },
      {
        "name": "icon",
        "type": "text",
        "max": 512000
      },
      {
        "name": "archived",
        "type": "bool"
      },
      {
        "name": "restricted",
        "type": "bool"
      },
      {
        "name": "published_version_id",
        "type": "text",
        "max": 64
      },
      {
        "name": "creator_id",
        "type": "text",
        "max": 64
      },
      {
        "name": "business_context",
        "type": "text",
        "max": 16000
      },
      {
        "name": "context_revision",
        "type": "number",
        "min": 0
      },
      {
        "name": "public_publication",
        "type": "json"
      },
      {
        "name": "public_slug",
        "type": "text",
        "max": 64
      },
      {
        "name": "view_count",
        "type": "number",
        "min": 0
      },
      {
        "name": "harness_step_id",
        "type": "text",
        "max": 80
      }
    ],
    "indexes": [
      "CREATE UNIQUE INDEX idx_apps_public_slug ON apps (public_slug) WHERE public_slug != ''",
      "CREATE UNIQUE INDEX idx_apps_harness_step ON apps (harness_step_id) WHERE harness_step_id != ''"
    ]
  },
  {
    "name": "app_versions",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "version",
        "type": "number",
        "min": 1,
        "required": true
      },
      {
        "name": "definition",
        "type": "json"
      },
      {
        "name": "summary",
        "type": "text",
        "max": 1000
      },
      {
        "name": "created_by",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "published_at",
        "type": "date"
      },
      {
        "name": "based_on_version_id",
        "type": "text",
        "max": 64
      },
      {
        "name": "source",
        "type": "json"
      },
      {
        "name": "manifest",
        "type": "json"
      },
      {
        "name": "capabilities",
        "type": "json"
      },
      {
        "name": "harness_step_id",
        "type": "text",
        "max": 80
      }
    ],
    "indexes": [
      "CREATE UNIQUE INDEX idx_app_versions_app_version ON app_versions (app_id, version)",
      "CREATE INDEX idx_app_versions_tenant_app ON app_versions (tenant_id, app_id, version)",
      "CREATE UNIQUE INDEX idx_harness_ui_step ON app_versions (harness_step_id) WHERE harness_step_id != ''"
    ]
  },
  {
    "name": "ai_usage",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "user_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64
      },
      {
        "name": "status",
        "type": "number",
        "max": 599,
        "min": 100
      },
      {
        "name": "input_tokens",
        "type": "number",
        "min": 0
      },
      {
        "name": "output_tokens",
        "type": "number",
        "min": 0
      },
      {
        "name": "kind",
        "type": "text",
        "max": 16
      },
      {
        "name": "provider",
        "type": "text",
        "max": 64
      },
      {
        "name": "model",
        "type": "text",
        "max": 160
      },
      {
        "name": "latency_ms",
        "type": "number",
        "min": 0
      },
      {
        "name": "input_known",
        "type": "bool"
      },
      {
        "name": "output_known",
        "type": "bool"
      }
    ],
    "indexes": [
      "CREATE INDEX idx_ai_usage_tenant_created ON ai_usage (tenant_id, created)",
      "CREATE INDEX idx_ai_usage_created ON ai_usage (created)"
    ]
  },
  {
    "name": "miao_runs",
    "fields": [
      {
        "name": "tenant_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "app_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "task_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "created_by",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "event_key",
        "type": "text",
        "max": 160,
        "required": true
      },
      {
        "name": "snapshot",
        "type": "json",
        "maxSize": 2000000,
        "required": true
      },
      {
        "name": "status",
        "type": "text",
        "max": 24,
        "required": true
      },
      {
        "name": "started_at",
        "type": "text",
        "max": 40
      },
      {
        "name": "finished_at",
        "type": "text",
        "max": 40
      },
      {
        "name": "output",
        "type": "text",
        "max": 30000
      },
      {
        "name": "error",
        "type": "text",
        "max": 1000
      },
      {
        "name": "pending",
        "type": "json",
        "maxSize": 2000000
      },
      {
        "name": "checkpoint",
        "type": "json",
        "maxSize": 6500000
      },
      {
        "name": "attempts",
        "type": "number"
      },
      {
        "name": "model_requests",
        "type": "number"
      },
      {
        "name": "delivery_status",
        "type": "text",
        "max": 24
      },
      {
        "name": "cancel_requested",
        "type": "bool"
      },
      {
        "name": "retry_count",
        "type": "number",
        "max": 2,
        "min": 0
      },
      {
        "name": "harness_state",
        "type": "text",
        "max": 40
      },
      {
        "name": "harness_phase",
        "type": "text",
        "max": 40
      },
      {
        "name": "harness_sequence",
        "type": "number",
        "min": 0
      },
      {
        "name": "harness_version",
        "type": "number",
        "min": 0
      },
      {
        "name": "harness_candidate",
        "type": "json",
        "maxSize": 200000
      },
      {
        "name": "harness_authority",
        "type": "json",
        "maxSize": 200000
      },
      {
        "name": "harness_result",
        "type": "json",
        "maxSize": 600000
      },
      {
        "name": "harness_loop",
        "type": "json",
        "maxSize": 6500000
      },
      {
        "name": "harness_storage_revision",
        "type": "number",
        "min": 0
      },
      {
        "name": "harness_lease_owner",
        "type": "text",
        "max": 80
      },
      {
        "name": "harness_lease_expires_at",
        "type": "text",
        "max": 40
      },
      {
        "name": "harness_active_started_at",
        "type": "text",
        "max": 40
      }
    ],
    "indexes": [
      "CREATE UNIQUE INDEX idx_miao_run_event ON miao_runs (task_id, event_key)",
      "CREATE INDEX idx_miao_run_status ON miao_runs (status, created)"
    ]
  },
  {
    "name": "tenants",
    "fields": [
      {
        "name": "name",
        "type": "text",
        "max": 160,
        "required": true
      },
      {
        "name": "owner_id",
        "type": "text",
        "max": 64,
        "required": true
      },
      {
        "name": "slug",
        "type": "text",
        "max": 160,
        "required": true
      },
      {
        "name": "ai_daily_limit",
        "type": "number",
        "min": 0
      },
      {
        "name": "ai_llm_daily_limit",
        "type": "number",
        "min": 0
      },
      {
        "name": "ai_jev_daily_limit",
        "type": "number",
        "min": 0
      }
    ]
  }
]

function findCollection(app, name) {
  try {
    return app.findCollectionByNameOrId(name)
  } catch (_) {
    return null
  }
}

function indexName(sql) {
  return sql.match(/INDEX `?([A-Za-z0-9_]+)`?/i)[1]
}

function hasIndex(collection, name) {
  const target = name.toLowerCase()
  return (collection.indexes || []).some(sql => indexName(sql).toLowerCase() === target)
}

function addIndex(collection, sql) {
  const name = indexName(sql)
  const unique = /\bUNIQUE\b/i.test(sql)
  const on = sql.match(/ON\s+`?[A-Za-z0-9_]+`?\s*\(/i)
  const start = sql.indexOf('(', on.index + on[0].length - 1)
  let depth = 0
  let end = -1
  for (let i = start; i < sql.length; i++) {
    if (sql[i] === '(') depth++
    else if (sql[i] === ')') {
      depth--
      if (depth === 0) { end = i; break }
    }
  }
  const columns = sql.slice(start + 1, end)
  const where = sql.slice(end + 1).replace(/^\s*WHERE\s+/i, '').trim()
  collection.addIndex(name, unique, columns, where)
}

migrate((app) => {
  for (const definition of definitions) {
    const existing = findCollection(app, definition.name)
    if (!existing) {
      app.save(new Collection({
        type: 'base',
        ...definition,
        listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
        fields: [
          { name: 'created', type: 'autodate', onCreate: true, system: true },
          { name: 'updated', type: 'autodate', onCreate: true, onUpdate: true, system: true },
          ...definition.fields,
        ],
      }))
      continue
    }
    let dirty = false
    for (const field of definition.fields) {
      if (existing.fields.getByName(field.name)) continue
      // Existing tables may hold rows predating the field; columns are added
      // without the required flag so historical records stay valid.
      const { required, ...descriptor } = field
      existing.fields.add(new Field(descriptor))
      dirty = true
    }
    for (const sql of definition.indexes || []) {
      if (hasIndex(existing, indexName(sql))) continue
      addIndex(existing, sql)
      dirty = true
    }
    if (dirty) app.save(existing)
  }
}, (app) => {
  // Down only strips fields and indexes; collections that this migration
  // created may hold data written after the upgrade, so they are kept.
  for (const definition of [...definitions].reverse()) {
    const existing = findCollection(app, definition.name)
    if (!existing) continue
    for (const sql of definition.indexes || []) {
      if (hasIndex(existing, indexName(sql))) existing.removeIndex(indexName(sql))
    }
    for (const field of [...definition.fields].reverse()) {
      if (existing.fields.getByName(field.name)) existing.fields.removeByName(field.name)
    }
    app.save(existing)
  }
})
