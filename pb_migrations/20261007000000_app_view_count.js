// Backfill for data directories created before the app view counter existed;
// a no-op when the initial schema already provides the field.
migrate((app) => {
  const apps = app.findCollectionByNameOrId("apps")
  if (apps.fields.getByName("view_count")) return
  apps.fields.add(new NumberField({ name: "view_count", min: 0 }))
  app.save(apps)
}, (app) => {
  const apps = app.findCollectionByNameOrId("apps")
  if (!apps.fields.getByName("view_count")) return
  apps.fields.removeByName("view_count")
  app.save(apps)
})
