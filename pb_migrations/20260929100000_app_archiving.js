migrate((app) => {
  const apps = app.findCollectionByNameOrId("apps")
  if (!apps.fields.getByName("archived")) {
    apps.fields.add(new BoolField({ name: "archived" }))
    app.save(apps)
  }
}, (app) => {
  const apps = app.findCollectionByNameOrId("apps")
  apps.fields.removeByName("archived")
  app.save(apps)
})
