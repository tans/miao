migrate((app) => {
  const runs = app.findCollectionByNameOrId('miao_runs')
  runs.fields.add(new NumberField({ name: 'retry_count', min: 0, max: 2 }))
  app.save(runs)
  const notifications = app.findCollectionByNameOrId('automation_notifications')
  notifications.fields.add(new TextField({ name: 'run_id', max: 64 }))
  app.save(notifications)
  app.save(new Collection({ name: 'miao_run_attempts', type: 'base', listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
    fields: [
      { name: 'created', type: 'autodate', onCreate: true, system: true },
      { name: 'updated', type: 'autodate', onCreate: true, onUpdate: true, system: true },
      ...['tenant_id', 'app_id', 'run_id'].map((name) => ({ name, type: 'text', required: true, max: 64 })),
      { name: 'sequence', type: 'number', required: true, min: 1 },
      { name: 'status', type: 'text', required: true, max: 24 },
      { name: 'started_at', type: 'text', max: 40 },
      { name: 'finished_at', type: 'text', max: 40 },
      { name: 'output', type: 'text', max: 30000 },
      { name: 'error', type: 'text', max: 1000 },
      { name: 'model_requests', type: 'number' }
    ], indexes: ['CREATE UNIQUE INDEX idx_miao_attempt_sequence ON miao_run_attempts (run_id, sequence)']
  }))
}, (app) => {
  const notifications = app.findCollectionByNameOrId('automation_notifications')
  notifications.fields.removeByName('run_id')
  app.save(notifications)
  app.delete(app.findCollectionByNameOrId('miao_run_attempts'))
  const runs = app.findCollectionByNameOrId('miao_runs')
  runs.fields.removeByName('retry_count')
  app.save(runs)
})
