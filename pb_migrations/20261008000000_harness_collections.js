// Add the durable harness collections to data directories created before the
// harness schema was included in the initial migration.
const definitions = [
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
]

function collection(app, name) {
  try {
    return app.findCollectionByNameOrId(name)
  } catch (_) {
    return null
  }
}

migrate((app) => {
  for (const definition of definitions) {
    if (collection(app, definition.name)) continue
    app.save(new Collection({
      type: "base",
      ...definition,
      listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
      fields: [
        { name: "created", type: "autodate", onCreate: true, system: true },
        { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true },
        ...definition.fields,
      ],
    }))
  }
}, (app) => {
  for (const definition of [...definitions].reverse()) {
    const existing = collection(app, definition.name)
    if (existing) app.delete(existing)
  }
})
