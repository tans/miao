migrate((app) => {
  for (const collection of app.findAllCollections()) {
    let changed = false
    for (const field of collection.fields) {
      if (field.type() === 'file' && field.protected !== true) {
        field.protected = true
        changed = true
      }
    }
    if (changed) app.save(collection)
  }
  const versions = app.findCollectionByNameOrId("app_versions")
  const definition = versions.fields.getByName("definition")
  if (definition) {
    definition.required = false
  }
  for (const field of ["source", "manifest", "capabilities"]) {
    if (!versions.fields.getByName(field)) versions.fields.add(new JSONField({ name: field }))
  }
  app.save(versions)
}, (app) => {
  const versions = app.findCollectionByNameOrId("app_versions")
  for (const name of ["capabilities", "manifest", "source"]) {
    if (versions.fields.getByName(name)) versions.fields.removeByName(name)
  }
  app.save(versions)
})
