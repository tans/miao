// Add the automation cursor to databases created before the field was present.
migrate((app) => {
  const changes = app.findCollectionByNameOrId("miao_record_changes")
  if (!changes || changes.fields.getByName("automation_processed")) return
  changes.fields.add(new BoolField({ name: "automation_processed" }))
  app.save(changes)
}, (app) => {
  const changes = app.findCollectionByNameOrId("miao_record_changes")
  if (!changes || !changes.fields.getByName("automation_processed")) return
  changes.fields.removeByName("automation_processed")
  app.save(changes)
})
