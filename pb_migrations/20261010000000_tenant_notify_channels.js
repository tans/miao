// Per-workspace external notification channels: email recipients plus one
// webhook each for WeCom, DingTalk, and Feishu bots.
migrate((app) => {
  const tenants = app.findCollectionByNameOrId("tenants")
  if (!tenants.fields.getByName("notify_channels")) {
    tenants.fields.add(new TextField({ name: "notify_channels", max: 4000 }))
  }
  app.save(tenants)
}, (app) => {
  const tenants = app.findCollectionByNameOrId("tenants")
  if (!tenants.fields.getByName("notify_channels")) return
  tenants.fields.removeByName("notify_channels")
  app.save(tenants)
})
