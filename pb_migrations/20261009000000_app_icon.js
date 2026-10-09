// Add the optional visual identity used by the workspace app cards.
migrate((app) => {
  const apps = app.findCollectionByNameOrId("apps")
  if (apps.fields.getByName("icon")) return
  apps.fields.add(new TextField({ name: "icon", max: 512000 }))
  app.save(apps)
}, (app) => {
  const apps = app.findCollectionByNameOrId("apps")
  if (!apps.fields.getByName("icon")) return
  apps.fields.removeByName("icon")
  app.save(apps)
})
