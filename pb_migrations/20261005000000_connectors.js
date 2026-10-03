migrate((app) => {
  // Keep the attachment protection invariant when this migration is reapplied.
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
    type: "base", name: "connectors",
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
    indexes: ["CREATE INDEX idx_connector_app ON connectors (tenant_id, app_id, status)"],
  }))
  app.save(new Collection({
    type: "base", name: "connector_runs",
    listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
    fields: [
      { name: "created", type: "autodate", onCreate: true, system: true },
      { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true },
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "app_id", type: "text", required: true, max: 64 },
      { name: "connector_id", type: "text", required: true, max: 64 },
      { name: "revision", type: "number", required: true, min: 1 },
      { name: "idempotency_key", type: "text", required: true, max: 160 },
      { name: "status", type: "select", required: true, maxSelect: 1, values: ["completed", "failed"] },
      { name: "result", type: "json" },
      { name: "error", type: "text", max: 1000 },
    ],
    indexes: ["CREATE UNIQUE INDEX idx_connector_run_key ON connector_runs (connector_id, idempotency_key)"],
  }))
}, (app) => {
  try { app.delete(app.findCollectionByNameOrId("connector_runs")) } catch {}
  try { app.delete(app.findCollectionByNameOrId("connectors")) } catch {}
})
