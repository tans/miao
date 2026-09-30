onRecordCreate((e) => require(__hooks + '/business-events.js').commit(e, 'created'));
onRecordUpdate((e) => require(__hooks + '/business-events.js').commit(e, 'updated'));
onRecordUpdateRequest((e) => {
  if (/^app_[a-z0-9]+_/.test(e.record.collection().name) && e.hasSuperuserAuth()) {
    e.record.set('__miao_skip_events', e.request.header.get('X-Miao-Event-Source') === 'background');
    e.record.set('__miao_expected_updated', e.request.header.get('X-Miao-Expected-Updated'));
  }
  e.next();
});
routerAdd('GET', '/api/miao/runtime', (e) => e.json(200, { atomic_record_events: true, record_version_check: true }), $apis.requireSuperuserAuth());
