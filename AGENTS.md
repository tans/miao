# Repository guide

- Develop and iterate only on `main`; do not create development branches.
- Commit every change and push it when a remote is configured.
- Do not write test code unless explicitly instructed.
- Keep the root [README.md](README.md) focused on the project overview and quick start.
- Keep detailed product behavior, API references, deployment, backup, and operational procedures in [docs/OPERATIONS.md](docs/OPERATIONS.md). Update that file when those behaviors change; avoid duplicate product or API documents.
- Keep [PRODUCT.md](PRODUCT.md) limited to machine-readable product metadata. Update it when product positioning or constraints change.
- Do not describe planned work as available functionality. CSV/XLSX imports are bounded and require reviewed plans; do not claim unlimited import or OCR.
- Run `npm test` and `git diff --check` for relevant changes. Validate PocketBase migrations against a clean data directory when migrations change.
- For multi-phase GitHub issue work, create or identify a public tracking issue before implementation; link it from the parent issue. After each meaningful phase or PR, update the tracker with merged commits/PRs, verified evidence, remaining blockers, and the next concrete step. Keep the parent issue's completion checklist truthful; local todos and docs do not replace GitHub progress updates. If GitHub write access is unavailable, record that blocker and do not claim remote synchronization.
- Use the system PM2 and project scripts for service lifecycle: `npm run server:start`, `npm run server:status`, and `npm run server:logs`. Do not start a separate long-running service or watcher.
