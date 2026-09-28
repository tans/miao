migrate((app) => {
  // PocketBase already creates the default `users` auth collection.
  // Keep it and use its built-in name field instead of trying to create a duplicate.

  const definitions = [
    {
      name: "tenants",
      fields: [
        { name: "name", type: "text", required: true, max: 160 },
        { name: "owner_id", type: "text", required: true, max: 64 },
        { name: "slug", type: "text", required: true, max: 160 },
      ],
    },
    {
      name: "apps",
      fields: [
        { name: "tenant_id", type: "text", required: true, max: 64 },
        { name: "name", type: "text", required: true, max: 160 },
        { name: "description", type: "text", max: 4000 },
      ],
    },
    {
      name: "app_collections",
      fields: [
        { name: "tenant_id", type: "text", required: true, max: 64 },
        { name: "app_id", type: "text", required: true, max: 64 },
        { name: "name", type: "text", required: true, max: 160 },
        { name: "slug", type: "text", required: true, max: 40 },
        { name: "pb_collection", type: "text", required: true, max: 80 },
        { name: "fields", type: "json" },
      ],
    },
  ]

  for (const definition of definitions) {
    app.save(new Collection({
      type: "base",
      name: definition.name,
      listRule: null,
      viewRule: null,
      createRule: null,
      updateRule: null,
      deleteRule: null,
      fields: [
        { name: "created", type: "autodate", onCreate: true, system: true },
        { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true },
        ...definition.fields,
      ],
    }))
  }

  const email = $os.getenv("POCKETBASE_SUPERUSER_EMAIL")
  const password = $os.getenv("POCKETBASE_SUPERUSER_PASSWORD")
  if (email && password) {
    const record = new Record(app.findCollectionByNameOrId("_superusers"))
    record.set("email", email)
    record.set("password", password)
    app.save(record)
  }
}, (app) => {
  for (const name of ["app_collections", "apps", "tenants"]) {
    try { app.delete(app.findCollectionByNameOrId(name)) } catch {}
  }
})
