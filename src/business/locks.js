const locks = new Map();
export async function serialized(key, work) {
  const prior = locks.get(key) || Promise.resolve();
  let release;
  const next = new Promise((resolve) => { release = resolve; });
  locks.set(key, next);
  await prior;
  try { return await work(); } finally { release(); if (locks.get(key) === next) locks.delete(key); }
}
