// Link provider usage receipts to the durable assistant/background run.
const collection = (app) => {
  try { return app.findCollectionByNameOrId("ai_usage") } catch (_) { return null }
}

migrate((app) => {
  const existing = collection(app)
  if (!existing || existing.fields.find((field) => field.name === "run_id")) return
  existing.fields.add(new TextField({ name: "run_id", max: 64 }))
  app.save(existing)
}, (app) => {
  const existing = collection(app)
  if (!existing || !existing.fields.find((field) => field.name === "run_id")) return
  existing.fields.removeByName("run_id")
  app.save(existing)
})
