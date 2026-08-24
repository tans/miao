import { Liquid } from 'liquidjs';

// LiquidJS is deliberately used without a file system loader. Templates can
// only render the data object supplied by the Runtime and cannot execute JS.
const engine = new Liquid({
  strictVariables: false,
  ownPropertyOnly: true,
  dynamicPartials: false,
  jsTruthy: false
});

const blockedTags = /{%\s*(?:include|render|layout|extends)\b/i;

export const validateTemplateSource = (source) => {
  if (typeof source !== 'string' || !source.trim()) throw new Error('template.source 必须是非空字符串');
  if (source.length > 200_000) throw new Error('模板不能超过 200KB');
  if (blockedTags.test(source)) throw new Error('模板不允许 include、render、layout 或 extends');
  return source;
};

export const renderTemplate = async (source, data = {}) => {
  const template = validateTemplateSource(source);
  if (!data || typeof data !== 'object' || Array.isArray(data)) throw new Error('模板数据必须是对象');
  return engine.parseAndRender(template, data);
};

export const templatePublic = (row) => ({
  id: row.id,
  app_id: row.app_id,
  name: row.name,
  object_type: row.object_type,
  version: row.version,
  status: row.status,
  source: row.source,
  created_at: row.created_at,
  updated_at: row.updated_at,
  provenance: row.provenance_json || null
});
