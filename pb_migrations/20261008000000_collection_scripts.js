migrate((app) => {
  const text = (name, max = 64, required = false) => ({ name, type: 'text', max, required })
  const json = (name, required = false, maxSize = 2000000) => ({ name, type: 'json', required, maxSize })
  const common = [text('tenant_id', 64, true), text('app_id', 64, true)]
  const definitions = [
    { name: 'collection_scripts', fields: [...common, text('created_by', 64, true), text('name', 160, true), { name: 'revision', type: 'number', min: 1 }, json('definition', true, 500000), text('status', 24, true), text('pause_reason', 300), text('next_run_at', 40)], indexes: ['CREATE INDEX idx_collection_script_app ON collection_scripts (tenant_id, app_id, status)'] },
    { name: 'collection_script_versions', fields: [...common, text('script_id', 64, true), { name: 'version', type: 'number', min: 1 }, text('created_by', 64, true), json('definition', true, 500000)], indexes: ['CREATE UNIQUE INDEX idx_collection_script_version ON collection_script_versions (script_id, version)'] },
    { name: 'collection_script_runs', fields: [...common, text('script_id', 64, true), { name: 'version', type: 'number', min: 1 }, text('created_by', 64, true), text('event_key', 180, true), text('mode', 16, true), text('status', 24, true), json('snapshot', true, 500000), text('started_at', 40), text('finished_at', 40), json('counts'), json('errors', false, 100000), json('result', false, 600000), text('error', 1000)], indexes: ['CREATE UNIQUE INDEX idx_collection_script_run_event ON collection_script_runs (script_id, event_key)', 'CREATE INDEX idx_collection_script_run_status ON collection_script_runs (tenant_id, app_id, status, created)'] },
    { name: 'collection_script_items', fields: [...common, text('script_id', 64, true), text('dedup_key', 180, true), text('status', 24, true), text('target_record_id', 64), text('last_run_id', 64), json('source'), text('error', 1000)], indexes: ['CREATE UNIQUE INDEX idx_collection_script_item_key ON collection_script_items (script_id, dedup_key)', 'CREATE INDEX idx_collection_script_item_status ON collection_script_items (script_id, status)'] },
    { name: 'collection_script_notifications', fields: [...common, text('script_id', 64, true), text('run_id', 64, true), text('item_id', 64, true), text('recipient_id', 64, true), text('event_key', 220, true), text('message', 1000, true), text('status', 24, true), text('error', 1000)], indexes: ['CREATE UNIQUE INDEX idx_collection_script_notification_key ON collection_script_notifications (script_id, item_id, recipient_id)', 'CREATE INDEX idx_collection_script_notification_pending ON collection_script_notifications (status, created)'] },
  ]
  for (const definition of definitions) app.save(new Collection({ type: 'base', ...definition,
    listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
    fields: [{ name: 'created', type: 'autodate', onCreate: true, system: true }, { name: 'updated', type: 'autodate', onCreate: true, onUpdate: true, system: true }, ...definition.fields]
  }))
}, (app) => {
  for (const name of ['collection_script_notifications', 'collection_script_items', 'collection_script_runs', 'collection_script_versions', 'collection_scripts']) app.delete(app.findCollectionByNameOrId(name))
})
