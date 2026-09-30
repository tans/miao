const bad = (message) => { throw Object.assign(new Error(message), { statusCode: 400 }); };
const integer = (value, fallback, min, max) => {
  const result = value ?? fallback;
  if (!Number.isInteger(result) || result < min || result > max) bad(`运行限制必须是 ${min}–${max} 的整数`);
  return result;
};
const parts = (date, timezone) => Object.fromEntries(new Intl.DateTimeFormat('en-CA', { timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).formatToParts(date).filter((part) => part.type !== 'literal').map((part) => [part.type, part.value]));

export function nextScheduledRun(trigger, after = new Date()) {
  if (['manual', 'record_created', 'status_changed'].includes(trigger.type)) return '';
  if (trigger.type === 'once') return Date.parse(trigger.at) > after.getTime() ? new Date(trigger.at).toISOString() : '';
  const local = parts(after, trigger.timezone);
  const [hour, minute] = trigger.time.split(':').map(Number);
  for (let day = 0; day < 9; day++) {
    const calendar = new Date(Date.UTC(Number(local.year), Number(local.month) - 1, Number(local.day) + day, hour, minute));
    if (trigger.type === 'weekly' && !trigger.weekdays.includes(calendar.getUTCDay())) continue;
    const desired = calendar.getTime();
    let candidate = desired;
    for (let iteration = 0; iteration < 4; iteration++) {
      const p = parts(new Date(candidate), trigger.timezone);
      const represented = Date.UTC(Number(p.year), Number(p.month) - 1, Number(p.day), Number(p.hour), Number(p.minute));
      candidate += desired - represented;
    }
    // Find the first occurrence of an ambiguous local time; nonexistent times are skipped.
    for (const offset of [-7200000, -3600000, -1800000, 0, 1800000, 3600000, 7200000]) {
      const p = parts(new Date(candidate + offset), trigger.timezone);
      if (Date.UTC(Number(p.year), Number(p.month) - 1, Number(p.day), Number(p.hour), Number(p.minute)) === desired) {
        const first = candidate + offset;
        if (first > after.getTime()) return new Date(first).toISOString();
        break;
      }
    }
  }
  return '';
}

export async function normalizeTaskDefinition(pocketbase, { tenantId, appId }, input) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) bad('任务定义必须是对象');
  const goal = String(input.goal || '').trim();
  if (!goal || goal.length > 6000) bad('请提供不超过 6000 字的任务目标');
  const type = input.trigger?.type || 'manual';
  if (!['manual', 'once', 'daily', 'weekly', 'record_created', 'status_changed'].includes(type)) bad('触发类型无效');
  const timezone = String(input.trigger?.timezone || 'Asia/Shanghai');
  try { parts(new Date(), timezone); } catch { bad('时区无效'); }
  const trigger = { type, timezone };
  if (type === 'once') {
    if (!/([zZ]|[+-]\d{2}:\d{2})$/.test(String(input.trigger.at)) || !Number.isFinite(Date.parse(input.trigger.at))) bad('一次性时间必须带时区');
    trigger.at = new Date(input.trigger.at).toISOString();
  }
  if (['daily', 'weekly'].includes(type)) {
    if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(input.trigger.time || '')) bad('运行时间格式为 HH:mm');
    trigger.time = input.trigger.time;
    if (type === 'weekly') {
      if (!Array.isArray(input.trigger.weekdays) || !input.trigger.weekdays.length || input.trigger.weekdays.some((day) => !Number.isInteger(day) || day < 0 || day > 6)) bad('星期使用 0–6，0 为周日');
      trigger.weekdays = [...new Set(input.trigger.weekdays)];
    }
  }
  const tables = await pocketbase.collection('app_collections').getFullList({ filter: pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId}', { tenantId, appId }) });
  const grants = input.scope?.tables;
  if (!Array.isArray(grants) || !grants.length || grants.length > 12) bad('请明确授权 1–12 张数据表');
  const seen = new Set();
  const scopeTables = grants.map((grant) => {
    if (!grant || typeof grant !== 'object' || Array.isArray(grant)) bad('数据表授权必须是对象');
    const table = tables.find((item) => item.slug === grant.table);
    if (!table || seen.has(grant.table)) bad('授权的数据表不存在或重复');
    seen.add(grant.table);
    const allowed = (table.fields || []).filter((field) => !['file', 'relation'].includes(field.type)).map((field) => field.name);
    const read = grant.read_fields;
    const write = grant.write_fields || [];
    if (!Array.isArray(read) || !read.length || !Array.isArray(write) || [...read, ...write].some((name) => !allowed.includes(name))) bad('需明确授权可读字段；首版后台任务不支持附件或关联字段');
    if (write.some((field) => !read.includes(field))) bad('可写字段也必须授权读取，便于展示修改前后的内容');
    return { table: table.slug, read_fields: [...new Set(read)], write_fields: [...new Set(write)] };
  });
  if (['record_created', 'status_changed'].includes(type)) {
    const table = tables.find((item) => item.slug === input.trigger.table);
    const grant = scopeTables.find((item) => item.table === table?.slug);
    if (!table || !grant) bad('业务事件必须来自获授权的数据表');
    trigger.table = table.slug;
    if (type === 'status_changed') {
      const field = table.fields.find((item) => item.name === input.trigger.field && item.type === 'select');
      if (!field || !grant.read_fields.includes(field.name) || !(field.options?.includes(input.trigger.from) || (!field.required && input.trigger.from === '')) || !(field.options?.includes(input.trigger.to) || (!field.required && input.trigger.to === '')) || input.trigger.from === input.trigger.to) bad('状态变化条件无效');
      Object.assign(trigger, { field: field.name, from: input.trigger.from, to: input.trigger.to });
    }
  }
  const recipients = input.scope?.recipient_ids || [];
  if (!Array.isArray(recipients) || recipients.length > 10 || recipients.some((id) => typeof id !== 'string')) bad('接收人最多 10 位');
  for (const userId of recipients) {
    const member = await pocketbase.collection('tenant_members').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && user_id = {:userId}', { tenantId, userId })).catch((error) => { if (error.status === 404) return null; throw error; });
    if (!member) bad('接收人必须是当前工作区成员');
  }
  const execution = input.execution || 'agent';
  if (!['agent', 'report'].includes(execution)) bad('执行方式为 agent 或 report');
  return { goal, execution, trigger, scope: { tables: scopeTables, recipient_ids: [...new Set(recipients)] }, limits: {
    max_writes: integer(input.limits?.max_writes, 10, 0, 100),
    max_requests: integer(input.limits?.max_requests, 12, 1, 30),
    timeout_seconds: integer(input.limits?.timeout_seconds, 180, 30, 600),
    confirmation_timeout_hours: integer(input.limits?.confirmation_timeout_hours, 72, 1, 720)
  } };
}
