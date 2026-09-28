import { cp, mkdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const source = path.join(root, 'node_modules', 'libfx');
const destination = path.join(root, 'public', 'vendor', 'fx');
await mkdir(destination, { recursive: true });
for (const file of ['browser.js', 'fx-sdk.js', 'wasm-module.js', 'core-output.js', 'fx-core.wasm']) {
  await cp(path.join(source, file), path.join(destination, file));
}
