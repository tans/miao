export const blockingPublishDiagnostics = (manifest) => (manifest.diagnostics || []).filter((item) => item.level === 'error' || item.code === 'uncompiled_mutation');

export const publishSnapshot = (app, timestamp) => ({
  published_definition: app.draft_definition ?? app.definition,
  published_manifest_json: app.draft_manifest_json ?? app.manifest_json,
  published_version: app.draft_version,
  updated_at: timestamp
});

export const rollbackSnapshot = (version, timestamp) => ({
  published_definition: version.definition,
  published_manifest_json: version.manifest_json,
  published_version: version.version,
  updated_at: timestamp
});
