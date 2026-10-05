migrate((app) => {
  const allowed = ["image/png", "image/jpeg", "image/gif", "image/webp", "application/pdf", "text/plain"]
  for (const collection of app.findAllCollections()) {
    if (!/^app_[a-z0-9]+_/.test(collection.name)) continue
    let changed = false
    for (const field of collection.fields) {
		if (field.type() === "file" && (field.protected !== true || JSON.stringify(field.mimeTypes || []) !== JSON.stringify(allowed))) {
		  field.protected = true
		  field.mimeTypes = allowed
		  changed = true
		}
    }
    if (changed) app.save(collection)
  }
}, (app) => {})
