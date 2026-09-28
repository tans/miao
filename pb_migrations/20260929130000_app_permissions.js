migrate((app) => {
  const apps = app.findCollectionByNameOrId("apps")
  if (!apps.fields.getByName("restricted")) {
    apps.fields.add(new BoolField({ name: "restricted" }))
    app.save(apps)
  }

  app.save(new Collection({
    type: "base",
    name: "app_members",
    listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
    fields: [
      { name: "created", type: "autodate", onCreate: true, system: true },
      { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true },
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "app_id", type: "text", required: true, max: 64 },
      { name: "user_id", type: "text", required: true, max: 64 },
      { name: "role", type: "select", required: true, maxSelect: 1, values: ["viewer", "editor"] },
    ],
    indexes: ["CREATE UNIQUE INDEX idx_app_members_app_user ON app_members (app_id, user_id)"],
  }))
}, (app) => {
  try { app.delete(app.findCollectionByNameOrId("app_members")) } catch {}
  const apps = app.findCollectionByNameOrId("apps")
  apps.fields.removeByName("restricted")
  app.save(apps)
})
