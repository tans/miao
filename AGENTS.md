# Agent instructions

- Run the application under the system PM2 using the project's PM2 configuration. Use `npm run server:start` to start or refresh it, `npm run server:status` to check it, and `npm run server:logs` to inspect logs.
- PM2 watches changes in `src` and `public` and restarts `miao-platform` automatically.
- Do not start or keep a separate long-running application process yourself (for example with `bun run dev`, `bun run start`, `nohup`, or a background shell command). Do not create a separate watcher or process supervisor. Let PM2 own the service lifecycle.
- For a temporary one-shot command, run it in the foreground and allow it to exit when the command is done.
