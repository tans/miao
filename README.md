# MIAO

**Agent-first internal business tools for teams.** Describe the work you need to do, shape a focused application with the fx agent, review its interface, then use it with your team.

MIAO combines [PocketBase](https://pocketbase.io/) for identity and application data with an in-browser [fx](https://fx.sh/) agent. The MIAO API enforces workspace and app permissions; AI credentials stay on the server.

## Features

- Create internal apps and data structures through an agent conversation.
- Preview, version, publish, and restore compatible business interfaces.
- Use multipage published apps with relations, attachments, details, and confirmed business actions.
- Configure reusable state workflows for any user-defined table without adding case-specific platform modules.
- Share application business notes and continue private conversations across devices.
- Review CSV/XLSX imports and inspect or restore record changes.
- Run authorized background tasks triggered by time, records, or authenticated external events.
- Invite workspace members and assign app roles with separate batch-update permission.
- Query records, review batch-update plans before execution, and create basic reminders.
- Self-host one Go binary with embedded PocketBase, UI, migrations, and backup/restore commands.

## Product boundaries

MIAO is designed around an agent-led workflow rather than a drag-and-drop app builder. Published interfaces support up to 12 validated pages and fixed field actions. File uploads are limited to 5 MB, table reading and each import to 100 rows. Record restoration excludes attachments, deletion, and schema. The embedded agent requires a browser with WebAssembly JSPI support.

## Self-hosting

MIAO runs as one Go process with PocketBase 0.40.4 embedded. The same binary contains the browser UI, database migrations, and backup/restore commands. Fresh installation is the delivery baseline. Install scripts support Linux and macOS on x64 and ARM64.

Building uses the Go 1.27.1 toolchain pinned in `go.mod` and Node.js/npm to bundle the browser Agent assets. The installation scripts use PM2, `curl`, and `openssl`; the compiled server itself needs no Node.js or separate PocketBase executable.

```sh
npm run server:install
npm run server:start
npm run server:status
```

`npm run package` creates a versioned binary archive and SHA-256 checksum. A `v*` tag runs the same checks and publishes Linux/macOS x64/ARM64 packages to [GitHub Releases](https://github.com/tans/miao/releases). Extract a package and run `bash scripts/install.sh`; binary installation needs no Go toolchain or npm dependency download.

The installer prints the location of the server configuration it creates. Configure the registration policy, platform admin email, and AI provider before exposing MIAO. For binary packages, ports, email, HTTPS proxy, and recovery, see the [deployment and operations guide](docs/OPERATIONS.md).

## Development

```sh
npm ci
npm run build
npm test
npm run test:race
go vet ./...
```

Run the managed application through PM2 using `npm run server:start`; see [AGENTS.md](AGENTS.md) for repository workflow notes.

## Documentation

- [Product, API, deployment, and operations guide](docs/OPERATIONS.md)
- [Repository and contribution notes](AGENTS.md)

## License

This repository does not currently contain a `LICENSE` file. Reuse and redistribution terms have not been specified.
