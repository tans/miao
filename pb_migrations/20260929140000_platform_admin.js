migrate((app) => {
  app.save(new Collection({
    type: "base",
    name: "platform_audit_logs",
    listRule: null,
    viewRule: null,
    createRule: null,
    updateRule: null,
    deleteRule: null,
    fields: [
      { name: "created", type: "autodate", onCreate: true, system: true },
      { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true },
      { name: "actor_id", type: "text", required: true, max: 64 },
      { name: "actor_email", type: "email", required: true },
      { name: "action", type: "text", required: true, max: 80 },
      { name: "target_type", type: "text", required: true, max: 32 },
      { name: "target_id", type: "text", max: 64 },
      { name: "reason", type: "text", max: 500 },
      { name: "status", type: "number", min: 100, max: 599 },
    ],
    indexes: [
      "CREATE INDEX idx_platform_audit_created ON platform_audit_logs (created)",
      "CREATE INDEX idx_platform_audit_action_created ON platform_audit_logs (action, created)",
    ],
  }))

  const usage = app.findCollectionByNameOrId("ai_usage")
  if (!usage.indexes.some((index) => index.includes("idx_ai_usage_created"))) {
    usage.indexes.push("CREATE INDEX idx_ai_usage_created ON ai_usage (created)")
    app.save(usage)
  }
}, (app) => {
  try { app.delete(app.findCollectionByNameOrId("platform_audit_logs")) } catch {}
  const usage = app.findCollectionByNameOrId("ai_usage")
  usage.indexes = usage.indexes.filter((index) => !index.includes("idx_ai_usage_created"))
  app.save(usage)
})
