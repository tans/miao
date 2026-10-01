// Runs inside PocketBase's SQLite transaction, sharing e.app with the save chain.
exports.commit = function (e, event) {
  var name = e.record.collection().name;
  if (!/^app_[a-z0-9]+_/.test(name)) return e.next();
  var originalApp = e.app;
  try {
    e.app.runInTransaction(function (tx) {
      e.app = tx;
      var metadata = tx.findFirstRecordByData('app_collections', 'pb_collection', name);
      var before = event === 'updated' ? tx.findRecordById(name, e.record.id) : null;
      var expected = e.record.getString('__miao_expected_updated');
      if (expected && before && before.getString('updated') !== expected) throw new ApiError(409, '记录已变化，请重新读取后操作');
      if (before && (before.getString('tenant_id') !== e.record.getString('tenant_id') || before.getString('app_id') !== e.record.getString('app_id'))) throw new ApiError(403, '不能改变记录归属');
      if (metadata.getString('tenant_id') !== e.record.getString('tenant_id') || metadata.getString('app_id') !== e.record.getString('app_id')) throw new ApiError(403, '记录归属无效');
      e.next();
      if (before) {
        // JSON arrays unmarshal directly through get; JSON.parse preserves arbitrary field names.
        var definitions = JSON.parse(metadata.getString('fields'));
        var previous = {}, next = {};
        for (var f = 0; f < definitions.length; f++) {
          var field = definitions[f];
          if (field.type === 'file') continue;
          previous[field.name] = before.get(field.name);
          next[field.name] = e.record.get(field.name);
        }
        if (JSON.stringify(previous) !== JSON.stringify(next)) {
          var change = new Record(tx.findCollectionByNameOrId('miao_record_changes'));
          change.set('tenant_id', metadata.getString('tenant_id')); change.set('app_id', metadata.getString('app_id'));
          change.set('table', metadata.getString('slug')); change.set('record_id', e.record.id);
          change.set('actor_id', e.record.getString('__miao_actor_id')); change.set('source', e.record.getString('__miao_source') || 'interactive');
          change.set('before', previous); change.set('after', next); tx.save(change);
        }
      }
      if (e.record.getBool('__miao_skip_events')) return;
      var params = { tenant: metadata.getString('tenant_id'), app: metadata.getString('app_id') };
      for (var offset = 0; ; offset += 200) {
        var tasks = tx.findRecordsByFilter('miao_tasks', 'tenant_id = {:tenant} && app_id = {:app} && status = "enabled"', 'id', 200, offset, params);
        for (var i = 0; i < tasks.length; i++) {
          var task = tasks[i];
          var definition = new DynamicModel({ goal: '', execution: '', trigger: {}, scope: {}, limits: {} });
          task.unmarshalJSONField('definition', definition);
          var trigger = definition.trigger;
          if (trigger.table !== metadata.getString('slug')) continue;
          var matches = (trigger.type === 'record_created' && event === 'created') || (trigger.type === 'status_changed' && event === 'updated' && trigger.from !== trigger.to && before.getString(trigger.field) === trigger.from && e.record.getString(trigger.field) === trigger.to);
          if (!matches) continue;
          var key = 'record:' + task.getInt('revision') + ':' + event + ':' + e.record.id + ':' + e.record.getString('updated');
          var duplicate = tx.findRecordsByFilter('miao_runs', 'task_id = {:task} && event_key = {:key}', '', 1, 0, { task: task.id, key: key });
          if (duplicate.length) continue;
          var run = new Record(tx.findCollectionByNameOrId('miao_runs'));
          run.set('tenant_id', params.tenant);
          run.set('app_id', params.app);
          run.set('task_id', task.id);
          run.set('created_by', task.getString('created_by'));
          run.set('event_key', key);
          run.set('snapshot', { goal: definition.goal, execution: definition.execution, trigger: definition.trigger, scope: definition.scope, limits: definition.limits, name: task.getString('name'), revision: task.getInt('revision'), input: { table: metadata.getString('slug'), record_id: e.record.id } });
          run.set('status', 'queued');
          run.set('delivery_status', 'pending');
          tx.save(run);
        }
        if (tasks.length < 200) break;
      }
    });
  } finally { e.app = originalApp; }
};
