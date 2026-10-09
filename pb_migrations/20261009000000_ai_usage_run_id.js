// Link provider usage receipts to the durable assistant/background run.
const collection = (app) => {
  try { return app.findCollectionByNameOrId("ai_usage") } catch (_) { return null }
}

migrate((app) => {
  const existing = collection(app)
  if (!existing || existing.fields.find((field) => field.name === "run_id")) return
  existing.fields.push({ name: "run_id", type: "text", max: 64 })
  app.save(existing)
}, (app) => {
  const existing = collection(app)
  const field = existing?.fields.find((item) => item.name === "run_id")
  if (existing && field) {
    existing.fields = existing.fields.filter((item) => item !== field)
    app.save(existing)
  }
})
