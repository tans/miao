const addTimestamps = (app, collectionName) => {
  const collection = app.findCollectionByNameOrId(collectionName)
  if (!collection.fields.getByName("created")) {
    collection.fields.add(new AutodateField({ name: "created", system: true, onCreate: true }))
  }
  if (!collection.fields.getByName("updated")) {
    collection.fields.add(new AutodateField({ name: "updated", system: true, onCreate: true, onUpdate: true }))
  }
  app.save(collection)
}

migrate((app) => {
  for (const name of ["tenants", "apps", "app_collections", "tenant_members", "tenant_invites"]) {
    addTimestamps(app, name)
  }

  const metadata = app.findRecordsByFilter("app_collections", "", "", 0, 0)
  for (const row of metadata) {
    addTimestamps(app, row.getString("pb_collection"))
  }
}, (app) => {
  const metadata = app.findRecordsByFilter("app_collections", "", "", 0, 0)
  for (const row of metadata) {
    const collection = app.findCollectionByNameOrId(row.getString("pb_collection"))
    collection.fields.removeByName("updated")
    collection.fields.removeByName("created")
    app.save(collection)
  }

  for (const name of ["tenant_invites", "tenant_members", "app_collections", "apps", "tenants"]) {
    const collection = app.findCollectionByNameOrId(name)
    collection.fields.removeByName("updated")
    collection.fields.removeByName("created")
    app.save(collection)
  }
})
