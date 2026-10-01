# MIAO

**Agent-first internal business tools for teams.** Describe the work you need to do, shape a focused application with the fx agent, review its interface, then use it with your team.

MIAO combines [PocketBase](https://pocketbase.io/) for identity and application data with an in-browser [fx](https://fx.sh/) agent. The MIAO API enforces workspace and app permissions; AI credentials stay on the server.

## Features

- Create internal apps and data structures through an agent conversation.
- Preview, version, publish, and restore compatible business interfaces.
- Use multipage published apps with relations, attachments, details, and confirmed business actions.
- Share application business notes and continue private conversations across devices.
- Review CSV/XLSX imports and inspect or restore record changes.
- Run authorized background tasks triggered by time, records, or authenticated external events.
- Invite workspace members and assign app roles with separate batch-update permission.
- Query records, review batch-update plans before execution, and create basic reminders.
- Self-host with PocketBase and PM2; back up application data and attachments.

## Product boundaries

MIAO is designed around an agent-led workflow rather than a drag-and-drop app builder. Published interfaces support up to 12 validated pages and fixed field actions. File uploads are limited to 5 MB, table reading and each import to 100 rows. Record restoration excludes attachments, deletion, and schema. The embedded agent requires a browser with WebAssembly JSPI support.

## Self-hosting

The Go service serves the existing UI and keeps PocketBase as its data store. Install scripts support Linux and macOS on x64 and ARM64. Building requires Go 1.22+, Node.js/npm (to vendor the browser Agent assets and run backup helpers), PM2, `curl`, `unzip`, and `openssl`.

```sh
npm run server:install
npm run server:start
npm run server:status
```

The installer prints the location of the server configuration it creates. Set the PocketBase administrator password and configure an AI provider before exposing MIAO. For ports, email, HTTPS proxy, upgrades, and recovery, see the [deployment and operations guide](docs/OPERATIONS.md).

## Development

```sh
npm install
npm run build
npm test
```

Run the managed application through PM2 using `npm run server:start`; see [AGENTS.md](AGENTS.md) for repository workflow notes.

## Documentation

- [Product, API, deployment, and operations guide](docs/OPERATIONS.md)
- [Repository and contribution notes](AGENTS.md)

## License

This repository does not currently contain a `LICENSE` file. Reuse and redistribution terms have not been specified.
