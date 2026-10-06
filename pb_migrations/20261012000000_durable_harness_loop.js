migrate((app) => {
  const runs = app.findCollectionByNameOrId('miao_harness_runs')
  runs.fields.add(new JSONField({ name: 'loop', maxSize: 6500000 }))
  runs.fields.add(new NumberField({ name: 'storage_revision', min: 0 }))
  runs.fields.add(new TextField({ name: 'lease_owner', max: 80 }))
  runs.fields.add(new TextField({ name: 'lease_expires_at', max: 40 }))
  runs.fields.add(new TextField({ name: 'active_started_at', max: 40 }))
  app.save(runs)

  const tasks = app.findCollectionByNameOrId('miao_runs')
  tasks.fields.add(new JSONField({ name: 'harness_loop', maxSize: 6500000 }))
  tasks.fields.add(new NumberField({ name: 'harness_storage_revision', min: 0 }))
  tasks.fields.add(new TextField({ name: 'harness_lease_owner', max: 80 }))
  tasks.fields.add(new TextField({ name: 'harness_lease_expires_at', max: 40 }))
  tasks.fields.add(new TextField({ name: 'harness_active_started_at', max: 40 }))
  app.save(tasks)

  const versions = app.findCollectionByNameOrId('app_versions')
  versions.fields.add(new TextField({ name: 'harness_step_id', max: 80 }))
  versions.indexes = [...versions.indexes, "CREATE UNIQUE INDEX idx_harness_ui_step ON app_versions (harness_step_id) WHERE harness_step_id != ''"]
  app.save(versions)
}, (app) => {
  const versions = app.findCollectionByNameOrId('app_versions')
  versions.indexes = versions.indexes.filter((index) => !index.includes('idx_harness_ui_step'))
  versions.fields.removeByName('harness_step_id')
  app.save(versions)
  const runs = app.findCollectionByNameOrId('miao_harness_runs')
  for (const name of ['active_started_at', 'lease_expires_at', 'lease_owner', 'storage_revision', 'loop']) runs.fields.removeByName(name)
  app.save(runs)

  const tasks = app.findCollectionByNameOrId('miao_runs')
  for (const name of ['harness_active_started_at', 'harness_lease_expires_at', 'harness_lease_owner', 'harness_storage_revision', 'harness_loop']) tasks.fields.removeByName(name)
  app.save(tasks)
})
