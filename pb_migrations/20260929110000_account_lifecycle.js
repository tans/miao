migrate((app) => {
  const users = app.findCollectionByNameOrId("users")
  if (!users.fields.getByName("disabled")) {
    users.fields.add(new BoolField({ name: "disabled" }))
    app.save(users)
  }

  app.save(new Collection({
    type: "base",
    name: "account_tokens",
    listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null,
    fields: [
      { name: "created", type: "autodate", onCreate: true, system: true },
      { name: "updated", type: "autodate", onCreate: true, onUpdate: true, system: true },
      { name: "user_id", type: "text", required: true, max: 64 },
      { name: "token_hash", type: "text", required: true, max: 64 },
      { name: "kind", type: "select", required: true, maxSelect: 1, values: ["verify", "reset"] },
      { name: "expires_at", type: "text", required: true, max: 40 },
    ],
    indexes: ["CREATE UNIQUE INDEX idx_account_tokens_hash ON account_tokens (token_hash)"],
  }))
}, (app) => {
  try { app.delete(app.findCollectionByNameOrId("account_tokens")) } catch {}
  const users = app.findCollectionByNameOrId("users")
  users.fields.removeByName("disabled")
  app.save(users)
})
