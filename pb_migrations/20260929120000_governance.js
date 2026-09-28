migrate((app) => {
  const tenants = app.findCollectionByNameOrId("tenants")
  if (!tenants.fields.getByName("ai_daily_limit")) {
    tenants.fields.add(new NumberField({ name: "ai_daily_limit", min: 0 }))
    app.save(tenants)
  }

  app.save(new Collection({
    type: "base",
    name: "platform_settings",
    listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
    fields: [
      { name: "created", type: "autodate", onCreate: true, system: true },
      { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true },
      { name: "name", type: "text", required: true, max: 64 },
      { name: "value", type: "text", required: true, max: 10000 },
      { name: "updated_by", type: "text", max: 64 },
    ],
    indexes: ["CREATE UNIQUE INDEX idx_platform_settings_name ON platform_settings (name)"],
  }))

  app.save(new Collection({
    type: "base",
    name: "ai_usage",
    listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
    fields: [
      { name: "created", type: "autodate", onCreate: true, system: true },
      { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true },
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "user_id", type: "text", required: true, max: 64 },
      { name: "app_id", type: "text", max: 64 },
      { name: "status", type: "number", min: 100, max: 599 },
      { name: "input_tokens", type: "number", min: 0 },
      { name: "output_tokens", type: "number", min: 0 },
    ],
    indexes: ["CREATE INDEX idx_ai_usage_tenant_created ON ai_usage (tenant_id, created)"],
  }))
}, (app) => {
  for (const name of ["ai_usage", "platform_settings"]) {
    try { app.delete(app.findCollectionByNameOrId(name)) } catch {}
  }
  const tenants = app.findCollectionByNameOrId("tenants")
  tenants.fields.removeByName("ai_daily_limit")
  app.save(tenants)
})
