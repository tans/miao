import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { JSONUIProvider, Renderer, defineRegistry } from '@json-render/react';
import { schema } from '@json-render/react/schema';
import { defineCatalog } from '@json-render/core';
import { shadcnComponentDefinitions } from '@json-render/shadcn/catalog';
import { shadcnComponents } from '@json-render/shadcn';

function childTitles(children) {
  return React.Children.toArray(children)
    .map((child) => child?.props?.element?.props?.title || child?.props?.props?.title)
    .filter(Boolean);
}
// Generic UI comes from the shadcn catalog (skill's technique). Metric and the
// record views are MIAO-specific: Metric shows a KPI, and the record views
// carry the data CRUD logic that shadcn's static Table does not.
const catalog = defineCatalog(schema, {
  components: {
    Text: shadcnComponentDefinitions.Text,
  },
  actions: {},
});
const { registry: shadcnRegistry } = defineRegistry(catalog, {
  components: {
    Text: shadcnComponents.Text,
  },
});

function Page({ props, children }) {
  const title = props.title || '';
  return <main className="jr-page">{title && !childTitles(children).includes(title) && <h2 className="text-foreground">{title}</h2>}{children}</main>;
}
function Section({ props, children }) {
  const title = props.title || '';
  return <section className="jr-section">{title && !childTitles(children).includes(title) && <h3 className="text-foreground">{title}</h3>}{children}</section>;
}
function Metric({ props }) {
  return <div className="jr-metric"><span className="text-muted-foreground">{props.label}</span><strong className="text-foreground">{props.value ?? '—'}</strong></div>;
}
function valueOf(source, row, field) { return source.relation_labels?.[field.name]?.[row.data?.[field.name]] ?? row.data?.[field.name] ?? '—'; }
function displayValue(source, row, field) {
  const value = valueOf(source, row, field);
  if (value === '—') return value;
  if (source.sanitized_markup === true && field.format === 'sanitized_html') return <div className="jr-rich-text" dangerouslySetInnerHTML={{ __html: String(value) }} />;
  if (field.type === 'file' && value) return String(value).startsWith('/api/public/') ? <img className="jr-image" src={String(value)} alt={field.label || field.name} loading="lazy" /> : <a className="text-primary underline" href={String(value)} target="_blank" rel="noreferrer">查看附件</a>;
  if (field.type === 'url' && value) return <a className="text-primary underline" href={String(value)} target="_blank" rel="noreferrer">{String(value)}</a>;
  return String(value);
}
// MIAO record-view inputs: native controls on purpose (SourceView's formDataFor
// reads form.elements[name].value, which a Radix Select does not populate),
// styled with the same shadcn token utilities as the rest of the island.
const inputCls = 'h-8 w-full rounded-md border border-input bg-background px-3 text-[13px] leading-[20px] text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:opacity-50';
const btnCls = (variant) => {
  const base = 'inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 text-[13px] leading-[20px] font-medium focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50';
  if (variant === 'primary') return `${base} bg-primary text-primary-foreground hover:bg-primary/90`;
  if (variant === 'danger') return `${base} border border-destructive bg-transparent text-destructive hover:bg-destructive/10`;
  return `${base} bg-transparent text-foreground hover:bg-accent hover:text-accent-foreground`;
};
function Input({ field, members, value }) {
  const common = { name: field.name, required: field.required, className: inputCls };
  if (field.type === 'bool') return <select {...common} defaultValue={value ?? ''}><option value="">请选择</option><option value="true">是</option><option value="false">否</option></select>;
  if (field.type === 'select') return <select {...common} defaultValue={value ?? ''}><option value="">请选择</option>{(field.options || []).map((item) => <option key={item} value={item}>{item}</option>)}</select>;
  if (field.type === 'member') return <select {...common} defaultValue={value ?? ''}><option value="">请选择成员</option>{members.map((member) => <option key={member.id} value={member.id}>{member.label}</option>)}</select>;
  if (field.type === 'relation') return <select {...common} defaultValue={value ?? ''}><option value="">请选择关联记录</option>{(field.relation_options || []).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}</select>;
  if (field.type === 'file') return <input {...common} type="file" accept="image/*,application/pdf,text/plain" />;
  return <input {...common} defaultValue={value ?? ''} step={field.type === 'number' ? 'any' : undefined} type={field.type === 'number' ? 'number' : ['date', 'email', 'url'].includes(field.type) ? field.type : 'text'} />;
}
function formDataFor(form, fields) {
  const data = {};
  for (const field of fields) {
    const control = form.elements[field.name];
    if (!control || field.type === 'file' || control.value === '') continue;
    data[field.name] = field.type === 'number' ? Number(control.value) : field.type === 'bool' ? control.value === 'true' : control.value;
  }
  return data;
}
function fileDataFor(form, fields) {
  return Object.fromEntries(fields.filter((field) => field.type === 'file').map((field) => [field.name, form.elements[field.name]?.files?.[0]]).filter(([, file]) => file));
}
function SourceView({ element, mode, onAction, onCreate, onUpdate, onDelete, onOpenRecord, onSearch, onPage }) {
  const props = element?.props ?? {};
  const source = mode.sources?.[props.source] || {};
  const fields = source.fields || [], rows = source.items || [], writable = !mode.readOnly;
  const [editing, setEditing] = useState(null);
  const [actionRequest, setActionRequest] = useState(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const perform = async (operation) => {
    if (pending) return;
    setError(''); setPending(true);
    try { await operation(); } catch (failure) { setError(failure.message || '操作失败，请重试。'); } finally { setPending(false); }
  };
  const feedback = error ? <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-[13px] leading-[20px] text-destructive" role="alert">{error}</div> : null;
  const createFields = source.create_form_fields || fields;
  const submit = (event) => { event.preventDefault(); const form = event.currentTarget; const data = { ...(source.create_defaults || {}), ...formDataFor(form, createFields) }, files = fileDataFor(form, createFields); perform(async () => { await onCreate?.(source, data, files); form.reset(); }); };
  const saveEdit = (event) => { event.preventDefault(); const form = event.currentTarget; const data = formDataFor(form, fields), files = fileDataFor(form, fields); perform(async () => { await onUpdate?.(source, editing, data, files); setEditing(null); }); };
  const form = writable && source.create_form_available !== false && (props.variant === 'form' || !mode.dedicatedFormSources.has(props.source)) ? <details><summary className={btnCls('primary')}>新增记录</summary><form onSubmit={submit}>{createFields.map((field) => <label key={field.name}>{field.label || field.name}<Input field={field} value={source.create_defaults?.[field.name]} members={mode.members || []} /></label>)}<button className={btnCls('primary')} type="submit" disabled={pending}>保存</button></form></details> : null;
  const editForm = editing ? <details open><summary className={btnCls('ghost')}>编辑记录</summary><form onSubmit={saveEdit}>{fields.map((field) => <label key={field.name}>{field.label || field.name}<Input field={{ ...field, required: false }} value={editing.data?.[field.name]} members={mode.members || []} /></label>)}<button className={btnCls('primary')} type="submit" disabled={pending}>保存修改</button></form></details> : null;
  const actionButtons = (row) => writable && (source.actions || []).filter((action) => !action.state_field || row.data?.[action.state_field] === undefined || row.data[action.state_field] === action.from).map((action) => <button className={btnCls('ghost')} key={action.id} type="button" disabled={pending || action.available === false} title={action.available === false ? '动作暂停或修订已变化，请管理员重新绑定发布。' : undefined} onClick={() => {
    if (action.inputs?.length) { setActionRequest({ action, row }); return; }
    if ((action.action_id || action.workflow_id) && !window.confirm(`确认对当前记录执行「${action.label}」？`)) return;
    perform(() => onAction?.(source, action, row, {}));
  }}>{action.label}{action.available === false ? '（不可用）' : ''}</button>);
  const actionForm = actionRequest ? <details open><summary>{actionRequest.action.label}</summary><form onSubmit={(event) => {
    event.preventDefault();
    const values = formDataFor(event.currentTarget, actionRequest.action.inputs);
    if (!window.confirm(`确认使用以上输入执行「${actionRequest.action.label}」？`)) return;
    perform(async () => { await onAction?.(source, actionRequest.action, actionRequest.row, values); setActionRequest(null); });
  }}>{actionRequest.action.inputs.map((field) => <label key={field.name}>{field.label || field.name}<Input field={field} members={mode.members || []} /></label>)}<div className="jr-actions"><button className={btnCls('primary')} type="submit" disabled={pending}>确认执行</button><button className={btnCls('ghost')} type="button" disabled={pending} onClick={() => setActionRequest(null)}>取消</button></div></form></details> : null;
  if (props.variant === 'detail') { const row = rows[0]; return <section className="jr-source" aria-busy={pending}>{feedback}<h3 className="text-foreground">{props.title || source.collection || ''}</h3>{row ? <><dl className="jr-detail">{fields.map((field) => <div key={field.name}><dt className="text-muted-foreground">{field.label || field.name}</dt><dd className="text-foreground">{displayValue(source, row, field)}</dd></div>)}</dl><div className="jr-actions">{actionButtons(row)}</div>{actionForm}{writable && <details><summary className={btnCls('ghost')}>编辑此记录</summary><form onSubmit={(event) => { event.preventDefault(); const data = formDataFor(event.currentTarget, fields), files = fileDataFor(event.currentTarget, fields); perform(() => onUpdate?.(source, row, data, files)); }}><div className="jr-detail-form">{fields.map((field) => <label key={field.name}>{field.label || field.name}<Input field={{ ...field, required: false }} value={row.data?.[field.name]} members={mode.members || []} /></label>)}</div><button className={btnCls('primary')} type="submit" disabled={pending}>保存修改</button></form></details>}</> : <p className="text-muted-foreground">暂无记录</p>}</section>; }
  if (props.variant === 'form' && mode.readOnly) return null;
  if (props.variant === 'form') return <section className="jr-source" aria-busy={pending}>{feedback}<h3 className="text-foreground">{props.title || '新增记录'}</h3>{form || <div className="jr-readonly-notice" role="status"><strong>当前界面为只读</strong><p>此应用版本暂不支持在这里新增记录。请进入“查看数据表”管理数据，或联系工作区管理员确认权限。</p></div>}</section>;
  const cards = props.variant === 'cards';
  const detailActionFor = (row) => onOpenRecord ? <button className={btnCls('ghost')} type="button" onClick={() => onOpenRecord(source, row)}>详情</button> : row.url ? <a className={btnCls('ghost')} href={row.url}>详情</a> : null;
  const rowActions = (row) => <div className="jr-actions">{detailActionFor(row)}{writable && <><button className={btnCls('ghost')} type="button" disabled={pending} onClick={() => setEditing(row)}>编辑</button><button className={btnCls('danger')} type="button" disabled={pending} onClick={() => perform(() => onDelete?.(source, row))}>删除</button>{actionButtons(row)}</>}</div>;
  const searchable = Boolean(onSearch) && fields.some((field) => ['text', 'email', 'url'].includes(field.type));
  return <section className="jr-source" aria-busy={pending}>{feedback}<h3 className="text-foreground">{props.title || source.collection || ''}</h3>{searchable && <form className="jr-search" onSubmit={(event) => { event.preventDefault(); onSearch?.(source, event.currentTarget.elements.search.value); }}><input className={inputCls} name="search" defaultValue={mode.search || ''} placeholder="搜索记录" aria-label="搜索记录"/><button className={btnCls('primary')} type="submit">搜索</button></form>}{actionForm}{editForm}{cards ? <div className="jr-cards">{rows.map((row) => <article className="rounded-lg border border-border bg-card p-4 text-card-foreground" key={row.id}>{fields.map((field) => <div key={field.name}><strong className="text-foreground">{field.label || field.name}</strong><div className="jr-card-value">{displayValue(source, row, field)}</div></div>)}{rowActions(row)}</article>)}</div> : <div className="jr-table-scroll"><table className="w-full text-[13px] leading-[20px] text-foreground"><thead><tr>{fields.map((field) => <th key={field.name} className="text-muted-foreground">{field.label || field.name}</th>)}<th>操作</th></tr></thead><tbody>{rows.map((row) => <tr key={row.id} className="border-b border-border">{fields.map((field) => <td key={field.name}>{displayValue(source, row, field)}</td>)}<td>{rowActions(row)}</td></tr>)}</tbody></table></div>}{!rows.length && <p className="text-muted-foreground">暂无记录</p>}{onPage && Number(source.total_pages) > 1 && <nav className="jr-pagination" aria-label="记录分页"><span className="text-muted-foreground">第 {source.page} / {source.total_pages} 页 · 共 {source.total_items} 条</span><div><button className={btnCls('ghost')} type="button" disabled={source.page <= 1} onClick={() => onPage?.(source, source.page - 1)}>上一页</button><button className={btnCls('ghost')} type="button" disabled={source.page >= source.total_pages} onClick={() => onPage?.(source, source.page + 1)}>下一页</button></div></nav>}{form}<small className="text-muted-foreground">{source.total_items ?? rows.length} 条记录</small></section>;
}
// @json-render/react 0.21 invokes registry components with {element, children};
// these adapters keep the component signatures below stable.
const adapt = (Component) => ({ element, children }) => <Component props={element?.props ?? {}}>{children}</Component>;
const registry = { Page: adapt(Page), Section: adapt(Section), Metric: adapt(Metric), RecordTable: (context) => <SourceView {...context} />, RecordCards: (context) => <SourceView {...context} element={{ ...(context.element ?? {}), props: { ...(context.element?.props ?? {}), variant: 'cards' } }} />, RecordDetail: (context) => <SourceView {...context} element={{ ...(context.element ?? {}), props: { ...(context.element?.props ?? {}), variant: 'detail' } }} />, RecordForm: (context) => <SourceView {...context} element={{ ...(context.element ?? {}), props: { ...(context.element?.props ?? {}), variant: 'form' } }} /> };
export function mount(root, spec, { sources = {}, members = [], readOnly = false, search = '', onAction, onCreate, onUpdate, onDelete, onOpenRecord, onSearch, onPage } = {}) {
  const reactRoot = createRoot(root);
  const dedicatedFormSources = new Set(Object.values(spec?.elements || {}).filter((element) => element?.type === 'RecordForm' && element.props?.source).map((element) => element.props.source));
  const scopedRegistry = Object.fromEntries(Object.entries(registry).map(([key, Component]) => [key, (context) => <Component {...context} mode={{ sources, members, readOnly, search, dedicatedFormSources }} onAction={readOnly ? undefined : onAction} onCreate={readOnly ? undefined : onCreate} onUpdate={readOnly ? undefined : onUpdate} onDelete={readOnly ? undefined : onDelete} onOpenRecord={onOpenRecord} onSearch={onSearch} onPage={onPage} />]));
  const mergedRegistry = { ...shadcnRegistry, ...scopedRegistry };
  reactRoot.render(<JSONUIProvider initialState={{ sources }}><Renderer spec={spec} registry={mergedRegistry} /></JSONUIProvider>);
  return () => reactRoot.unmount();
}
