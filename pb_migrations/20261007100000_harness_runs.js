migrate((app) => {
  const text = (name, max = 160, required = false) => ({ name, type: 'text', max, required })
  const json = (name, required = false, maxSize = 1000000) => ({ name, type: 'json', required, maxSize })
  const common = [text('tenant_id', 64, true), text('app_id', 64, true), text('user_id', 64, true)]
  app.save(new Collection({ type: 'base', name: 'miao_harness_runs', fields: [
    ...common, text('prompt', 12000, true), json('input'), text('state', 40, true), text('phase', 40, true),
    { name: 'sequence', type: 'number', min: 0 }, { name: 'version', type: 'number', min: 0 },
    json('candidate'), json('authority'), json('result'), text('error', 2000), { name: 'cancel_requested', type: 'bool' },
  ], indexes: ['CREATE INDEX idx_harness_runs_owner ON miao_harness_runs (tenant_id, user_id, created)', 'CREATE INDEX idx_harness_runs_state ON miao_harness_runs (state, updated)'], listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null }))
  app.save(new Collection({ type: 'base', name: 'miao_harness_events', fields: [
    text('tenant_id', 64, true), text('app_id', 64, true), text('user_id', 64, true), text('run_id', 64, true),
    { name: 'sequence', type: 'number', min: 1 }, text('event_type', 80, true), json('data'),
  ], indexes: ['CREATE UNIQUE INDEX idx_harness_event_sequence ON miao_harness_events (run_id, sequence)', 'CREATE INDEX idx_harness_events_run ON miao_harness_events (run_id, sequence)'], listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null }))
}, (app) => {
  for (const name of ['miao_harness_events', 'miao_harness_runs']) {
    const collection = app.findCollectionByNameOrId(name)
    if (collection) app.delete(collection)
  }
})
