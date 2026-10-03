migrate((app) => {
  // Preserve the invariant when this migration is the latest one reapplied.
  for (const collection of app.findAllCollections()) {
    let changed = false
    for (const field of collection.fields) {
      if (field.type() === "file" && field.protected !== true) {
        field.protected = true
        changed = true
      }
    }
    if (changed) app.save(collection)
  }
  app.save(new Collection({
    type: "base", name: "workflows",
    listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
    fields: [
      { name: "created", type: "autodate", onCreate: true, system: true },
      { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true },
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "app_id", type: "text", required: true, max: 64 },
      { name: "created_by", type: "text", required: true, max: 64 },
      { name: "name", type: "text", required: true, max: 160 },
      { name: "description", type: "text", max: 1000 },
      { name: "definition", type: "json", required: true },
      { name: "status", type: "select", required: true, maxSelect: 1, values: ["draft", "enabled", "paused", "archived"] },
      { name: "revision", type: "number", required: true, min: 1 },
      { name: "pause_reason", type: "text", max: 300 },
    ],
    indexes: ["CREATE INDEX idx_workflow_app ON workflows (tenant_id, app_id, status)"],
  }))
  app.save(new Collection({
    type: "base", name: "workflow_runs",
    listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
    fields: [
      { name: "created", type: "autodate", onCreate: true, system: true },
      { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true },
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "app_id", type: "text", required: true, max: 64 },
      { name: "workflow_id", type: "text", required: true, max: 64 },
      { name: "revision", type: "number", required: true, min: 1 },
      { name: "transition_id", type: "text", required: true, max: 64 },
      { name: "record_id", type: "text", required: true, max: 64 },
      { name: "idempotency_key", type: "text", required: true, max: 160 },
      { name: "status", type: "select", required: true, maxSelect: 1, values: ["completed", "failed"] },
      { name: "result", type: "json" },
      { name: "error", type: "text", max: 1000 },
    ],
    indexes: ["CREATE UNIQUE INDEX idx_workflow_run_key ON workflow_runs (workflow_id, transition_id, record_id, idempotency_key)"],
  }))
}, (app) => {
  try { app.delete(app.findCollectionByNameOrId("workflow_runs")) } catch {}
  try { app.delete(app.findCollectionByNameOrId("workflows")) } catch {}
})
