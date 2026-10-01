onRecordCreate((e) => require(__hooks + '/business-events.js').commit(e, 'created'));
onRecordUpdate((e) => require(__hooks + '/business-events.js').commit(e, 'updated'));
onRecordUpdateRequest((e) => {
  if (/^app_[a-z0-9]+_/.test(e.record.collection().name) && e.hasSuperuserAuth()) {
    e.record.set('__miao_actor_id', (e.requestInfo().headers['x_miao_actor_id'] || ''));
    e.record.set('__miao_source', (e.requestInfo().headers['x_miao_event_source'] || '') || 'interactive');
    e.record.set('__miao_skip_events', (e.requestInfo().headers['x_miao_event_source'] || '') === 'background');
    e.record.set('__miao_expected_updated', (e.requestInfo().headers['x_miao_expected_updated'] || ''));
  }
  e.next();
});
routerAdd('GET', '/api/miao/runtime', (e) => e.json(200, { atomic_record_events: true, record_version_check: true }), $apis.requireSuperuserAuth());
