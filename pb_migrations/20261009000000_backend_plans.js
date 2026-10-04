migrate((app) => {
  const text = (name, max = 180, required = false) => ({ name, type: 'text', max, required })
  const json = (name, required = false, maxSize = 2000000) => ({ name, type: 'json', required, maxSize })
  app.save(new Collection({
    type: 'base',
    name: 'app_backend_plans',
    listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
    fields: [
      { name: 'created', type: 'autodate', onCreate: true, system: true },
      { name: 'updated', type: 'autodate', onCreate: true, onUpdate: true, system: true },
      text('tenant_id', 64, true), text('app_id', 64, true), text('user_id', 64, true),
      text('status', 24, true), { name: 'revision', type: 'number', min: 1 },
      text('baseline_hash', 64, true), text('expires_at', 40, true),
      json('operations', true, 1000000), json('dependency_order', true, 100000),
      json('impact', true, 100000), json('receipt', true, 1000000),
    ],
    indexes: [
      'CREATE INDEX idx_backend_plans_app ON app_backend_plans (tenant_id, app_id, created)',
      'CREATE INDEX idx_backend_plans_owner ON app_backend_plans (tenant_id, user_id, status)',
    ],
  }))
}, (app) => {
  const collection = app.findCollectionByNameOrId('app_backend_plans')
  if (collection) app.delete(collection)
})
