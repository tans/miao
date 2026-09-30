import { taskAuthority } from '../business/access.js';
import { buildRecordFilter, validateRecordData, publicRecord, updateBusinessRecord } from '../business/records.js';
import { actionKey, missing, serialized } from './repository.js';

export function createRunTools({ pocketbase, run, stop, assertActive }) {
  const definition = run.snapshot;
  const failures = [];
  const authority = () => taskAuthority(pocketbase, run);
  const scopedTable = async (slug) => {
    await assertActive();
    await authority();
    const grant = definition.scope.tables.find((item) => item.table === slug);
    if (!grant) throw new Error('此数据表未获任务读取授权');
    const table = await pocketbase.collection('app_collections').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && app_id = {:appId} && slug = {:slug}', { tenantId: run.tenant_id, appId: run.app_id, slug }));
    if (grant.read_fields.some((name) => !table.fields.some((field) => field.name === name && !['file', 'relation'].includes(field.type)))) throw new Error('授权字段已失效，请重新配置任务');
    return { table, grant };
  };
  const visible = (row, grant) => ({ ...publicRecord(row), data: Object.fromEntries(grant.read_fields.map((name) => [name, row[name]])) });
  const query = async ({ table: slug, conditions = [], page = 1 }) => {
    const { table, grant } = await scopedTable(slug);
    if (!Number.isInteger(page) || page < 1 || page > 10000) throw new Error('页码无效');
    const filter = buildRecordFilter({ tenant: { id: run.tenant_id }, params: { id: run.app_id } }, { ...table, fields: table.fields.filter((field) => grant.read_fields.includes(field.name)) }, conditions, pocketbase);
    const data = await pocketbase.collection(table.pb_collection).getList(page, 25, { filter, sort: '-created' });
    return { items: data.items.map((row) => visible(row, grant)), totalItems: data.totalItems, page: data.page, totalPages: data.totalPages };
  };
  const suspend = async (pending) => {
    await pocketbase.collection('miao_runs').update(run.id, { status: 'waiting', pending, error: pending.reason, delivery_status: 'pending' });
    stop();
    throw Object.assign(new Error(pending.reason), { code: 'WAITING' });
  };
  const update = async (input) => serialized(`effect:${run.id}`, async () => {
    const { table, grant } = await scopedTable(input.table);
    if (!input.data || typeof input.data !== 'object' || Array.isArray(input.data) || !Object.keys(input.data).length || Object.keys(input.data).some((name) => !grant.read_fields.includes(name))) throw new Error('只能修改已授权读取的具体业务字段');
    const validation = validateRecordData(input.data, table.fields, { partial: true });
    if (validation) throw new Error(validation);
    const key = actionKey('update_record', input);
    let action = await pocketbase.collection('miao_actions').getFirstListItem(pocketbase.filter('run_id = {:runId} && action_key = {:key}', { runId: run.id, key })).catch(missing);
    if (action?.status === 'done') return action.result;
    const collection = pocketbase.collection(table.pb_collection);
    const row = await collection.getOne(String(input.record_id));
    if (row.app_id !== run.app_id || row.tenant_id !== run.tenant_id) throw new Error('记录不属于当前应用');
    if (!input.expected_updated_at || row.updated !== input.expected_updated_at) throw new Error('记录已变化，请重新查询后提交准确变更');
    const evidence = { before: visible(row, grant), after: { ...visible(row, grant).data, ...input.data } };
    if (!action) action = await pocketbase.collection('miao_actions').create({ tenant_id: run.tenant_id, app_id: run.app_id, run_id: run.id, action_key: key, tool: 'update_record', input, evidence, status: 'planned' });
    if (action.status === 'executing' || action.status === 'unknown') return suspend({ kind: 'uncertain', action_id: action.id, reason: '上一次写入结果不确定，请核实当前记录后再处理', input, evidence });
    const completed = await pocketbase.collection('miao_actions').getList(1, 1, { filter: pocketbase.filter('run_id = {:runId} && status = "done"', { runId: run.id }) });
    if (completed.totalItems >= definition.limits.max_writes) throw new Error('本次运行已达到写入数量限制');
    const autoAllowed = Object.keys(input.data).every((name) => grant.write_fields.includes(name));
    if (!autoAllowed && action.status !== 'approved') {
      await pocketbase.collection('miao_actions').update(action.id, { status: 'waiting' });
      return suspend({ kind: 'approval', action_id: action.id, reason: '此具体变更超出任务预先授权，等待负责人确认', input, evidence });
    }
    await assertActive();
    await authority();
    await pocketbase.collection('miao_actions').update(action.id, { status: 'executing' });
    try {
      const saved = await updateBusinessRecord({ pocketbase, table, tenantId: run.tenant_id, appId: run.app_id, recordId: row.id, data: input.data, expectedUpdated: input.expected_updated_at, authorize: async () => { await assertActive(); await authority(); } });
      const result = visible(saved, grant);
      await pocketbase.collection('miao_actions').update(action.id, { status: 'done', result });
      // Background writes do not re-enter record triggers, preventing self-trigger loops.
      return result;
    } catch (error) {
      if (error.statusCode || error.code === 'AUTH_REVOKED' || [400, 403, 404].includes(error.status)) {
        await pocketbase.collection('miao_actions').update(action.id, { status: 'rejected' });
        throw error;
      }
      await pocketbase.collection('miao_actions').update(action.id, { status: 'unknown' });
      return suspend({ kind: 'uncertain', action_id: action.id, reason: '写入结果待核实，系统不会盲目重试', input, evidence });
    }
  });
  const tools = [
    { name: 'list_tables', description: '读取本任务明确授权的数据表与字段。', inputSchema: { type: 'object', properties: {}, additionalProperties: false }, async execute() {
      const result = [];
      for (const grant of definition.scope.tables) { const { table } = await scopedTable(grant.table); result.push({ table: table.slug, name: table.name, fields: table.fields.filter((field) => grant.read_fields.includes(field.name)), write_fields: grant.write_fields }); }
      return JSON.stringify(result);
    } },
    { name: 'query_records', description: '查询授权数据，返回记录 ID 和 updated_at。每页 25 条，可继续分页；不要将一页当作全部结果。', inputSchema: { type: 'object', required: ['table'], properties: { table: { type: 'string' }, page: { type: 'integer', minimum: 1 }, conditions: { type: 'array', maxItems: 8, items: { type: 'object', required: ['field', 'op'], properties: { field: { type: 'string' }, op: { type: 'string', enum: ['eq', 'contains', 'before', 'after', 'empty'] }, value: {} } } } } }, async execute(input) { return JSON.stringify(await query(input)); } },
    { name: 'update_record', description: '设置单条记录的具体字段值，必须提供刚查询得到的 updated_at。超出预先授权会暂停运行等待具体确认；不可声称已执行。', inputSchema: { type: 'object', required: ['table', 'record_id', 'expected_updated_at', 'data'], properties: { table: { type: 'string' }, record_id: { type: 'string' }, expected_updated_at: { type: 'string' }, data: { type: 'object', additionalProperties: true } } }, async execute(input) { return JSON.stringify(await update(input)); } },
    { name: 'request_information', description: '缺少必要事实时请求负责人补充信息，停止本次执行，不要猜测。', inputSchema: { type: 'object', required: ['question'], properties: { question: { type: 'string', maxLength: 1000 } } }, async execute({ question }) { await assertActive(); await authority(); return suspend({ kind: 'information', reason: String(question).slice(0, 1000) }); } }
  ];
  return { tools: tools.map((tool) => ({ ...tool, async execute(input) {
    try { return await tool.execute(input); }
    catch (error) { if (error.code !== 'WAITING') failures.push(error.message); throw error; }
  } })), query, update, authority, failures };
}
