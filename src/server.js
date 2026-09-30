import { start, app } from './routes/index.js';

await start();

let closing = false;
for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, async () => {
  if (closing) return;
  closing = true;
  try { await app.close(); process.exit(0); } catch (error) { app.log.error(error); process.exit(1); }
});
