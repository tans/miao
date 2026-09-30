migrate((app) => {
  const text = (name, max = 64, required = false) => ({ name, type: 'text', max, required })
  const json = (name, required = false, maxSize = 2000000) => ({ name, type: 'json', required, maxSize })
  const common = [text('tenant_id', 64, true), text('app_id', 64, true)]
  const definitions = [
    { name: 'miao_tasks', fields: [...common, text('created_by', 64, true), text('name', 160, true), { name: 'revision', type: 'number', min: 1 }, json('definition', true), text('status', 24, true), text('pause_reason', 300), text('next_run_at', 40)] },
    { name: 'miao_runs', fields: [...common, text('task_id', 64, true), text('created_by', 64, true), text('event_key', 160, true), json('snapshot', true), text('status', 24, true), text('started_at', 40), text('finished_at', 40), text('output', 30000), text('error', 1000), json('pending'), json('checkpoint', false, 6500000), { name: 'attempts', type: 'number' }, { name: 'model_requests', type: 'number' }, text('delivery_status', 24), { name: 'cancel_requested', type: 'bool' }], indexes: ['CREATE UNIQUE INDEX idx_miao_run_event ON miao_runs (task_id, event_key)', 'CREATE INDEX idx_miao_run_status ON miao_runs (status, created)'] },
    { name: 'miao_actions', fields: [...common, text('run_id', 64, true), text('action_key', 64, true), text('tool', 80, true), json('input', true), text('status', 24, true), json('result'), json('evidence'), text('approved_by', 64), text('approved_at', 40)], indexes: ['CREATE UNIQUE INDEX idx_miao_action_key ON miao_actions (run_id, action_key)'] },
    { name: 'miao_runtime_locks', fields: [text('name', 64, true), text('owner', 64, true), text('expires_at', 40, true)], indexes: ['CREATE UNIQUE INDEX idx_miao_runtime_lock ON miao_runtime_locks (name)'] }
  ]
  for (const definition of definitions) app.save(new Collection({ type: 'base', ...definition,
    listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
    fields: [{ name: 'created', type: 'autodate', onCreate: true, system: true }, { name: 'updated', type: 'autodate', onCreate: true, onUpdate: true, system: true }, ...definition.fields]
  }))
}, (app) => {
  for (const name of ['miao_runtime_locks', 'miao_actions', 'miao_runs', 'miao_tasks']) app.delete(app.findCollectionByNameOrId(name))
})
