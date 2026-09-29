migrate((app) => {
  const versions = app.findCollectionByNameOrId("app_versions")
  if (!versions.fields.getByName("based_on_version_id")) {
    versions.fields.add(new TextField({ name: "based_on_version_id", max: 64 }))
    app.save(versions)
  }
}, (app) => {
  const versions = app.findCollectionByNameOrId("app_versions")
  if (versions.fields.getByName("based_on_version_id")) {
    versions.fields.removeByName("based_on_version_id")
    app.save(versions)
  }
})
