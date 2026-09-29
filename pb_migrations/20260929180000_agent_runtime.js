migrate((app) => {
  const apps = app.findCollectionByNameOrId("apps")
  apps.fields.add(new TextField({ name: "published_version_id", max: 64 }))
  apps.fields.add(new TextField({ name: "draft_version_id", max: 64 }))
  apps.fields.add(new TextField({ name: "creator_id", max: 64 }))
  app.save(apps)

  const members = app.findCollectionByNameOrId("app_members")
  const role = members.fields.getByName("role")
  role.values = ["viewer", "editor", "manager", "publisher"]
  members.fields.add(new BoolField({ name: "can_batch" }))
  app.save(members)

  const definitions = [
    { name: "app_versions", fields: [
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "app_id", type: "text", required: true, max: 64 },
      { name: "base_version_id", type: "text", max: 64 },
      { name: "definition", type: "json", required: true },
      { name: "summary", type: "text", max: 2000 },
      { name: "created_by", type: "text", required: true, max: 64 },
      { name: "published_at", type: "date" },
    ] },
    { name: "agent_threads", fields: [
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "app_id", type: "text", max: 64 },
      { name: "user_id", type: "text", required: true, max: 64 },
      { name: "title", type: "text", max: 160 },
    ] },
    { name: "agent_messages", fields: [
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "thread_id", type: "text", required: true, max: 64 },
      { name: "user_id", type: "text", required: true, max: 64 },
      { name: "role", type: "select", required: true, maxSelect: 1, values: ["user", "assistant"] },
      { name: "content", type: "text", required: true, max: 30000 },
    ] },
    { name: "batch_jobs", fields: [
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "app_id", type: "text", required: true, max: 64 },
      { name: "user_id", type: "text", required: true, max: 64 },
      { name: "plan", type: "json", required: true },
      { name: "status", type: "text", required: true, max: 24 },
      { name: "result", type: "json" },
    ] },
    { name: "automation_rules", fields: [
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "app_id", type: "text", required: true, max: 64 },
      { name: "created_by", type: "text", required: true, max: 64 },
      { name: "name", type: "text", required: true, max: 160 },
      { name: "definition", type: "json", required: true },
      { name: "enabled", type: "bool" },
      { name: "pause_reason", type: "text", max: 200 },
    ] },
    { name: "automation_runs", fields: [
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "app_id", type: "text", required: true, max: 64 },
      { name: "rule_id", type: "text", required: true, max: 64 },
      { name: "event_key", type: "text", required: true, max: 160 },
      { name: "status", type: "text", required: true, max: 24 },
      { name: "result", type: "json" },
    ] },
    { name: "automation_notifications", fields: [
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "app_id", type: "text", required: true, max: 64 },
      { name: "rule_id", type: "text", required: true, max: 64 },
      { name: "event_key", type: "text", required: true, max: 160 },
      { name: "user_id", type: "text", required: true, max: 64 },
      { name: "message", type: "text", required: true, max: 500 },
      { name: "read", type: "bool" },
    ] },
  ]
  for (const definition of definitions) app.save(new Collection({
    type: "base", name: definition.name,
    listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
    fields: [
      { name: "created", type: "autodate", onCreate: true, system: true },
      { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true },
      ...definition.fields,
    ],
    indexes: definition.name === "automation_runs" ? ["CREATE UNIQUE INDEX idx_automation_run_event ON automation_runs (rule_id, event_key)"] : definition.name === "automation_notifications" ? ["CREATE UNIQUE INDEX idx_automation_notification_event ON automation_notifications (rule_id, event_key, user_id)"] : [],
  }))
}, (app) => {
  for (const name of ["automation_notifications", "automation_runs", "automation_rules", "batch_jobs", "agent_messages", "agent_threads", "app_versions"]) {
    try { app.delete(app.findCollectionByNameOrId(name)) } catch {}
  }
  const members = app.findCollectionByNameOrId("app_members")
  members.fields.getByName("role").values = ["viewer", "editor"]
  members.fields.removeByName("can_batch")
  app.save(members)
  const apps = app.findCollectionByNameOrId("apps")
  apps.fields.removeByName("published_version_id")
  apps.fields.removeByName("draft_version_id")
  apps.fields.removeByName("creator_id")
  app.save(apps)
})
