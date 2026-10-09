#!/usr/bin/env node
// miao-test 本地 API 测试套件运行器。
// 用法：
//   node miao-test/run.mjs [--base http://127.0.0.1:41874] [--keep]
// 依赖本地 miao 服务（npm run server:start），测试账号与数据仅用于本地演练。

import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { ensureAccount, Client } from "./lib.mjs";

const args = process.argv.slice(2);
const flag = (name) => args.includes(name);
const option = (name) => {
  const i = args.indexOf(name);
  return i >= 0 ? args[i + 1] : undefined;
};

const base =
  option("--base") ?? process.env.MIAO_TEST_BASE ?? `http://127.0.0.1:${process.env.MIAO_PORT ?? "41874"}`;
const keep = flag("--keep");

if (flag("--list")) {
  const files = await listCaseFiles();
  for (const file of files) {
    const mod = await import(file);
    for (const item of mod.default) console.log(`  ${item.name}`);
  }
  process.exit(0);
}

async function listCaseFiles() {
  const dir = path.join(path.dirname(fileURLToPath(import.meta.url)), "cases");
  const names = (await readdir(dir)).filter((f) => f.endsWith(".mjs")).sort();
  return names.map((f) => pathToFileURL(join(dir, f)).href);
}
function join(...parts) {
  return path.join(...parts);
}
function pathToFileURL(p) {
  return new URL(`file://${p}`);
}

const accounts = {
  owner: { email: "miao-test-owner@example.test", password: "MiaoTestOwner!01", name: "miao-test Owner" },
  editor: { email: "miao-test-editor@example.test", password: "MiaoTestEditor!02", name: "miao-test Editor" },
  viewer: { email: "miao-test-viewer@example.test", password: "MiaoTestViewer!03", name: "miao-test Viewer" },
  outsider: { email: "miao-test-outsider@example.test", password: "MiaoTestOutsider!04", name: "miao-test Outsider" },
  admin: { email: "miao-test-admin@example.test", password: "MiaoTestAdmin!05", name: "miao-test Admin" },
};

console.log(`miao-test 目标服务: ${base}`);
const health = await fetch(`${base}/api/health`).catch(() => null);
if (!health || health.status !== 200) {
  console.error(`无法访问 ${base}/api/health，请先启动本地服务（npm run server:start）`);
  process.exit(2);
}

// ---- 准备测试账号 ----
const state = { appId: "" };
for (const role of ["owner", "editor", "viewer", "outsider", "admin"]) {
  state[role] = await ensureAccount(base, accounts[role]);
  console.log(`  账号就绪: ${accounts[role].email}`);
}
const outsiderMe = await state.outsider.get("/api/me");
if (outsiderMe.status !== 200) throw new Error(`外部账号登录失败: ${JSON.stringify(outsiderMe.body)}`);
state.outsiderHomeTenantId = outsiderMe.body.tenant.id;

const t = { base, accounts, state, anon: new Client(base, { name: "anon" }) };
t.expectStatus = (res, expected, what) => {
  const allowed = Array.isArray(expected) ? expected : [expected];
  if (!allowed.includes(res.status)) {
    throw new Error(`${what ?? "expected status"}: expected ${allowed.join(" or ")} but got status=${res.status} body=${JSON.stringify(res.body)?.slice(0, 400)}`);
  }
};
t.expectField = (res, p, predicate, what) => {
  let value = res;
  for (const part of p.split(".")) value = value?.[part];
  if (predicate !== undefined && !predicate(value)) {
    throw new Error(`${what ?? `field ${p} mismatch`}: got ${JSON.stringify(value)}`);
  }
  return value;
};

// ---- 顺序执行 ----
const cases = [];
for (const file of await listCaseFiles()) {
  const mod = await import(file);
  for (const item of mod.default) cases.push({ file: path.basename(file), ...item });
}

let passed = 0;
let failed = 0;
const failures = [];
for (const item of cases) {
  const started = Date.now();
  try {
    await item.run(t);
    passed += 1;
    console.log(`  PASS  ${item.name} (${Date.now() - started}ms)`);
  } catch (err) {
    failed += 1;
    failures.push({ name: item.name, error: err?.message ?? String(err) });
    console.log(`  FAIL  ${item.name}\n        ${String(err?.message ?? err).split("\n").slice(0, 3).join("\n        ")}`);
  }
}

// ---- 清理 ----
if (!keep && state.appId) {
  try {
    const res = await state.owner.delete(`/api/apps/${state.appId}`, { confirm: true });
    console.log(res.status < 300 ? "  已清理测试应用" : `  清理测试应用失败: ${res.status} ${JSON.stringify(res.body)}`);
  } catch (err) {
    console.log(`  清理测试应用异常: ${err.message}`);
  }
} else if (keep) {
  console.log("  --keep: 保留测试应用与数据");
}

console.log(`\n结果: ${passed} 通过, ${failed} 失败, 共 ${cases.length} 个案例（${base}）`);
for (const f of failures) console.log(`  - ${f.name}\n      ${f.error}`);
process.exit(failed > 0 ? 1 : 0);
