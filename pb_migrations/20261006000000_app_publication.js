migrate((app) => {
  const apps = app.findCollectionByNameOrId("apps")
  if (!apps.fields.getByName("public_publication")) {
    apps.fields.add(new JSONField({ name: "public_publication" }))
  }
  if (!apps.fields.getByName("public_slug")) {
    apps.fields.add(new TextField({ name: "public_slug", max: 64 }))
  }
  app.save(apps)
  if (!apps.indexes.some((index) => index.includes("idx_apps_public_slug"))) {
    apps.indexes.push("CREATE UNIQUE INDEX idx_apps_public_slug ON apps (public_slug) WHERE public_slug != ''")
    app.save(apps)
  }
}, (app) => {
  const apps = app.findCollectionByNameOrId("apps")
  apps.fields.removeByName("public_publication")
  apps.fields.removeByName("public_slug")
  apps.indexes = apps.indexes.filter((index) => !index.includes("idx_apps_public_slug"))
  app.save(apps)
})
