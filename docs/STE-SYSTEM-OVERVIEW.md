# MIAO System Overview (STE-flavored)

The skill rewrites the text in STE-flavored mode. The linter reports 0 hard violations and 0 advisory findings. The numbers and limits copy from README.md. When they conflict, docs/OPERATIONS.md wins.

## How it runs

MIAO is one Go process. It embeds PocketBase 0.40.4, the browser UI, database migrations, and backup/restore commands. The MIAO API sits in front of PocketBase and enforces workspace and app permissions. AI credentials stay on the server. The browser assistant is only a client of the server API. The installer writes a configuration file with the data directory, listen address, encryption key, and the first admin email. A user edits the other business settings in the admin console. PM2 runs the service. The npm scripts start, stop, and inspect it.

A user describes the work. The server-backed assistant shapes a focused application around the data, screens, and actions the team needs. The user previews a UI version. The user publishes it. The user restores a compatible version later.

## What it does

- A user creates apps and tables through agent conversation.
- State workflows run on any user-defined table without case-specific modules.
- Agent and collection scripts read external HTTP(S) sources inside bounded runs with budgets and idempotent receipts.
- Background tasks start from a time, a record change, or an authenticated external event.
- A user shares a read-only public link. The link exposes selected pages and fields. It never exposes workspace access or writes.
- A user reviews CSV and XLSX imports as plans before they write rows.
- Members join a workspace and hold app roles. Batch updates need their own permission.

## Limits

- Published interfaces hold at most 12 pages.
- File uploads cap at 5 MB.
- Table reads and each import cap at 100 rows.
- Restoration returns record changes. It does not return attachments, deletions, or schema.
- Model-dependent composition needs server credentials. Deterministic CRUD works without a model.
