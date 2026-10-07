// Account-level UI language ("zh-CN" / "en"; empty follows the browser).
migrate((app) => {
  const users = app.findCollectionByNameOrId("users")
  if (users.fields.getByName("language")) return
  users.fields.add(new TextField({ name: "language", max: 10 }))
  app.save(users)
}, (app) => {
  const users = app.findCollectionByNameOrId("users")
  if (!users.fields.getByName("language")) return
  users.fields.removeByName("language")
  app.save(users)
})
