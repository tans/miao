migrate((app) => {
  app.save(new Collection({
    type: "base",
    name: "tenant_members",
    listRule: null,
    viewRule: null,
    createRule: null,
    updateRule: null,
    deleteRule: null,
    fields: [
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "user_id", type: "text", required: true, max: 64 },
      { name: "role", type: "text", required: true, max: 16 },
    ],
    indexes: ["CREATE UNIQUE INDEX idx_tenant_members_tenant_user ON tenant_members (tenant_id, user_id)"],
  }))

  app.save(new Collection({
    type: "base",
    name: "tenant_invites",
    listRule: null,
    viewRule: null,
    createRule: null,
    updateRule: null,
    deleteRule: null,
    fields: [
      { name: "tenant_id", type: "text", required: true, max: 64 },
      { name: "email", type: "email", required: true },
      { name: "token_hash", type: "text", required: true, max: 64 },
      { name: "expires_at", type: "text", required: true, max: 40 },
      { name: "invited_by", type: "text", required: true, max: 64 },
      { name: "status", type: "text", required: true, max: 16 },
    ],
    indexes: ["CREATE UNIQUE INDEX idx_tenant_invites_token_hash ON tenant_invites (token_hash)"],
  }))

  const tenants = app.findRecordsByFilter("tenants", "", "", 0, 0)
  const members = app.findCollectionByNameOrId("tenant_members")
  for (const tenant of tenants) {
    const record = new Record(members)
    record.set("tenant_id", tenant.id)
    record.set("user_id", tenant.getString("owner_id"))
    record.set("role", "owner")
    app.save(record)
  }
}, (app) => {
  for (const name of ["tenant_invites", "tenant_members"]) {
    try { app.delete(app.findCollectionByNameOrId(name)) } catch {}
  }
})
