// Allow each user to keep multiple workspace- or app-scoped conversations.
migrate((app) => {
  const sessions = app.findCollectionByNameOrId("agent_sessions")
  if (!sessions.fields.getByName("app_id")) sessions.fields.add(new TextField({ name: "app_id", max: 64 }))
  if (!sessions.fields.getByName("title")) sessions.fields.add(new TextField({ name: "title", max: 160 }))
  if ((sessions.indexes || []).some(sql => /INDEX `?idx_agent_sessions_owner`?/i.test(sql))) {
    sessions.removeIndex("idx_agent_sessions_owner")
  }
  if (!(sessions.indexes || []).some(sql => /INDEX `?idx_agent_sessions_owner_app`?/i.test(sql))) {
    sessions.addIndex("idx_agent_sessions_owner_app", false, "tenant_id, user_id, app_id, updated")
  }
  app.save(sessions)
}, (app) => {
  const sessions = app.findCollectionByNameOrId("agent_sessions")
  if ((sessions.indexes || []).some(sql => /INDEX `?idx_agent_sessions_owner_app`?/i.test(sql))) {
    sessions.removeIndex("idx_agent_sessions_owner_app")
  }
  if (!(sessions.indexes || []).some(sql => /INDEX `?idx_agent_sessions_owner`?/i.test(sql))) {
    sessions.addIndex("idx_agent_sessions_owner", true, "tenant_id, user_id")
  }
  if (sessions.fields.getByName("title")) sessions.fields.removeByName("title")
  if (sessions.fields.getByName("app_id")) sessions.fields.removeByName("app_id")
  app.save(sessions)
})
