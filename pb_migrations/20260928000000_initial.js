migrate((app) => {
  const users = new Collection({
    type: "auth",
    name: "users",
    listRule: null,
    viewRule: null,
    createRule: null,
    updateRule: null,
    deleteRule: null,
    fields: [{ name: "name", type: "text", required: true, max: 120 }],
    passwordAuth: { enabled: true },
  })
  app.save(users)

  const definitions = [
    {
      name: "tenants",
      fields: [
        { name: "name", type: "text", required: true, max: 160 },
        { name: "owner_id", type: "text", required: true, max: 64 },
        { name: "slug", type: "text", required: true, max: 160 },
        { name: "created_at", type: "text", max: 64 },
        { name: "updated_at", type: "text", max: 64 },
      ],
    },
    {
      name: "apps",
      fields: [
        { name: "tenant_id", type: "text", required: true, max: 64 },
        { name: "name", type: "text", required: true, max: 160 },
        { name: "description", type: "text", max: 4000 },
        { name: "definition", type: "text" },
        { name: "draft_definition", type: "text" },
        { name: "published_definition", type: "text" },
        { name: "manifest_json", type: "json" },
        { name: "draft_manifest_json", type: "json" },
        { name: "published_manifest_json", type: "json" },
        { name: "published_version", type: "number" },
        { name: "draft_version", type: "number" },
        { name: "created_at", type: "text", max: 64 },
        { name: "updated_at", type: "text", max: 64 },
      ],
    },
    {
      name: "app_versions",
      fields: [
        { name: "app_id", type: "text", required: true, max: 64 },
        { name: "version", type: "number" },
        { name: "definition", type: "text" },
        { name: "manifest_json", type: "json" },
        { name: "status", type: "text", max: 32 },
        { name: "published_at", type: "text", max: 64 },
        { name: "previous_version", type: "number" },
        { name: "created_at", type: "text", max: 64 },
        { name: "updated_at", type: "text", max: 64 },
      ],
    },
    {
      name: "static_resources",
      viewRule: "",
      fields: [
        { name: "tenant_id", type: "text", required: true, max: 64 },
        { name: "app_id", type: "text", required: true, max: 64 },
        { name: "path", type: "text", required: true, max: 240 },
        { name: "version", type: "number" },
        { name: "content", type: "file", maxSelect: 1, maxSize: 26214400 },
        { name: "mime", type: "text", max: 255 },
        { name: "size", type: "number" },
        { name: "deleted_at", type: "text", max: 64 },
        { name: "created_at", type: "text", max: 64 },
        { name: "updated_at", type: "text", max: 64 },
      ],
    },
    {
      name: "events",
      fields: [
        { name: "tenant_id", type: "text", required: true, max: 64 },
        { name: "app_id", type: "text", max: 64 },
        { name: "type", type: "text", max: 120 },
        { name: "message", type: "text", max: 4000 },
        { name: "payload_json", type: "json" },
        { name: "actor", type: "text", max: 120 },
        { name: "created_at", type: "text", max: 64 },
      ],
    },
    {
      name: "traces",
      fields: [
        { name: "tenant_id", type: "text", required: true, max: 64 },
        { name: "app_id", type: "text", max: 64 },
        { name: "session_id", type: "text", max: 64 },
        { name: "user_id", type: "text", max: 64 },
        { name: "agent_id", type: "text", max: 120 },
        { name: "permissions", type: "json" },
        { name: "tool", type: "text", max: 160 },
        { name: "status", type: "text", max: 32 },
        { name: "input_json", type: "json" },
        { name: "output_json", type: "json" },
        { name: "error", type: "text", max: 4000 },
        { name: "duration_ms", type: "number" },
        { name: "created_at", type: "text", max: 64 },
      ],
    },
  ]

  for (const definition of definitions) {
    app.save(new Collection({
      type: "base",
      name: definition.name,
      listRule: null,
      viewRule: definition.viewRule ?? null,
      createRule: null,
      updateRule: null,
      deleteRule: null,
      fields: definition.fields,
    }))
  }

  const email = $os.getenv("POCKETBASE_SUPERUSER_EMAIL")
  const password = $os.getenv("POCKETBASE_SUPERUSER_PASSWORD")
  if (email && password) {
    const superusers = app.findCollectionByNameOrId("_superusers")
    const record = new Record(superusers)
    record.set("email", email)
    record.set("password", password)
    app.save(record)
  }
}, (app) => {
  for (const name of ["traces", "events", "static_resources", "app_versions", "apps", "tenants", "users"]) {
    try { app.delete(app.findCollectionByNameOrId(name)) } catch {}
  }
})
