migrate((app) => {
  const apps = app.findCollectionByNameOrId("apps")
  if (!apps.fields.getByName("published_version_id")) {
    apps.fields.add(new TextField({ name: "published_version_id", max: 64 }))
    app.save(apps)
  }

  app.save(new Collection({
    type: "base",
    name: "app_versions",
    listRule: null,
    viewRule: null,
    createRule: null,
    updateRule: null,
    deleteRule: null,
    fields: [
      { name: "created", type: "autodate", onCreate: true, system: true },
      { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true },
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "app_id", type: "text", required: true, max: 64 },
      { name: "version", type: "number", required: true, min: 1 },
      { name: "definition", type: "json", required: true },
      { name: "summary", type: "text", max: 1000 },
      { name: "created_by", type: "text", required: true, max: 64 },
      { name: "published_at", type: "date" },
    ],
    indexes: [
      "CREATE UNIQUE INDEX idx_app_versions_app_version ON app_versions (app_id, version)",
      "CREATE INDEX idx_app_versions_tenant_app ON app_versions (tenant_id, app_id, version)",
    ],
  }))
}, (app) => {
  try { app.delete(app.findCollectionByNameOrId("app_versions")) } catch {}
  const apps = app.findCollectionByNameOrId("apps")
  apps.fields.removeByName("published_version_id")
  app.save(apps)
})
