export const blockingPublishDiagnostics = (manifest) => (manifest.diagnostics || []).filter((item) => item.level === 'error' || item.code === 'uncompiled_mutation');

export const publishSnapshot = (app, timestamp) => ({
  published_source: app.draft_source ?? app.source,
  published_manifest_json: app.draft_manifest_json ?? app.manifest_json,
  published_version: app.draft_version,
  updated_at: timestamp
});

export const rollbackSnapshot = (version, timestamp) => ({
  published_source: version.source,
  published_manifest_json: version.manifest_json,
  published_version: version.version,
  updated_at: timestamp
});
