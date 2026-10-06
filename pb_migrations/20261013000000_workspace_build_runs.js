migrate((app) => {
  for (const name of ['miao_harness_runs', 'miao_harness_events']) {
    const collection = app.findCollectionByNameOrId(name)
    collection.fields.getByName('app_id').required = false
    app.save(collection)
  }
  for (const name of ['apps', 'app_backend_plans']) {
    const collection = app.findCollectionByNameOrId(name)
    collection.fields.add(new TextField({ name: 'harness_step_id', max: 80 }))
    collection.indexes = [...collection.indexes, `CREATE UNIQUE INDEX idx_${name}_harness_step ON ${name} (harness_step_id) WHERE harness_step_id != ''`]
    app.save(collection)
  }
}, (app) => {
  for (const name of ['apps', 'app_backend_plans']) {
    const collection = app.findCollectionByNameOrId(name)
    collection.indexes = collection.indexes.filter((index) => !index.includes(`idx_${name}_harness_step`))
    collection.fields.removeByName('harness_step_id')
    app.save(collection)
  }
  // Workspace runs remain valid historical records when rolling this migration back.
})
