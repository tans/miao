migrate((app) => {
  const apps = app.findCollectionByNameOrId('apps');
  apps.fields.add(new TextField({ name: 'business_context', max: 16000 }));
  apps.fields.add(new NumberField({ name: 'context_revision', min: 0 }));
  app.save(apps);
  // Business attachments must be served through MIAO's authorized proxy.
  for (const collection of app.findAllCollections()) {
    if (!/^app_[a-z0-9]+_/.test(collection.name)) continue;
    let changed = false;
    for (const field of collection.fields) if (field.type() === 'file') { field.protected = true; changed = true; }
    if (changed) app.save(collection);
  }
  const base = [
    { name: 'created', type: 'autodate', onCreate: true, system: true },
    { name: 'updated', type: 'autodate', onCreate: true, onUpdate: true, system: true },
    { name: 'tenant_id', type: 'text', required: true, max: 64 },
  ];
  for (const config of [
    { name: 'agent_sessions', fields: [
      { name: 'user_id', type: 'text', required: true, max: 64 },
      { name: 'scope', type: 'text', required: true, max: 100000 },
      { name: 'revision', type: 'number', min: 0 },
      { name: 'checkpoint', type: 'text', max: 6000000 },
      { name: 'messages', type: 'json', maxSize: 300000 },
    ], indexes: ['CREATE UNIQUE INDEX idx_agent_sessions_owner ON agent_sessions (tenant_id, user_id)'] },
    { name: 'app_files', fields: [
      { name: 'app_id', type: 'text', required: true, max: 64 },
      { name: 'user_id', type: 'text', required: true, max: 64 },
      { name: 'name', type: 'text', required: true, max: 120 },
      { name: 'file', type: 'file', protected: true, required: true, maxSelect: 1, maxSize: 5242880 },
    ] },
    { name: 'miao_record_changes', fields: [
      { name: 'app_id', type: 'text', required: true, max: 64 },
      { name: 'table', type: 'text', required: true, max: 64 },
      { name: 'record_id', type: 'text', required: true, max: 64 },
      { name: 'actor_id', type: 'text', max: 64 },
      { name: 'source', type: 'text', max: 32 },
      { name: 'before', type: 'json' }, { name: 'after', type: 'json' },
    ], indexes: ['CREATE INDEX idx_record_changes_app ON miao_record_changes (tenant_id, app_id, record_id)'] },
  ]) app.save(new Collection({ type: 'base', listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null, ...config, fields: [...base, ...config.fields] }));
}, (app) => {
  for (const name of ['agent_sessions', 'app_files', 'miao_record_changes']) app.delete(app.findCollectionByNameOrId(name));
  const apps = app.findCollectionByNameOrId('apps');
  apps.fields.removeByName('business_context'); apps.fields.removeByName('context_revision'); app.save(apps);
});
