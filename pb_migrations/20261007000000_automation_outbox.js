migrate((app) => {
  const changes = app.findCollectionByNameOrId("miao_record_changes")
  if (!changes.fields.getByName("event")) {
    changes.fields.add(new TextField({ name: "event", max: 16 }))
  }
  if (!changes.fields.getByName("automation_processed")) {
    changes.fields.add(new BoolField({ name: "automation_processed" }))
  }
  app.save(changes)
  if (!changes.indexes.some((index) => index.includes("idx_record_changes_automation"))) {
    changes.indexes.push("CREATE INDEX idx_record_changes_automation ON miao_record_changes (automation_processed, created)")
    app.save(changes)
  }
}, (app) => {
  const changes = app.findCollectionByNameOrId("miao_record_changes")
  changes.fields.removeByName("event")
  changes.fields.removeByName("automation_processed")
  changes.indexes = changes.indexes.filter((index) => !index.includes("idx_record_changes_automation"))
  app.save(changes)
})
