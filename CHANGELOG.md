# Changelog

All notable changes to MIAO are documented here. Release packages are published on [GitHub Releases](https://github.com/tans/miao/releases).

## [0.2.0] - 2026-10-11

### Added

- Export one application table as CSV, including the authenticated API route and download action.
- Search records across every accessible application and table from the workspace.
- Share a record with a stable hash link that opens and highlights the target record.
- Filter the notification inbox by application, type, and read state, with an all-read action and unread grouping.
- Schedule background tasks with interval, monthly, and cron triggers.
- Send workspace notifications through email, WeCom, DingTalk, and Lark channels.
- Add six business templates and three daily intelligence templates to the template marketplace.

### Changed

- Keep the product interface in Chinese and English.
- Make conversational and JEV-driven interface editing the documented product direction.
- Refresh the generated API contract and operations documentation for the new routes and behavior.

### Fixed

- Isolate panics in the background task worker and scan loop so one failed run does not stop the process.
- Route administrator endpoints through the shared administrator authorization boundary.

### Maintenance

- Split the HTTP API into smaller workspace, application access, public site, usage, UI draft, and backend plan domains without changing their public routes.

## [0.0.1] - 2026-10-10

- Initial self-hosted MIAO release with the embedded PocketBase Go runtime, agent-first application workflow, versioned interfaces, background tasks, public read-only pages, imports, and backup/restore commands.

[0.2.0]: https://github.com/tans/miao/compare/v0.0.1...v0.2.0
[0.0.1]: https://github.com/tans/miao/releases/tag/v0.0.1
