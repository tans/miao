import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Renderer, StateProvider, VisibilityProvider } from '@json-render/react';

function Page({ props, children }) { return <main className="jr-page"><h2>{props.title || ''}</h2>{children}</main>; }
function Section({ props, children }) { return <section className="jr-section">{props.title && <h3>{props.title}</h3>}{children}</section>; }
function Text({ props }) { return <p className="jr-text">{props.text ?? ''}</p>; }
function Metric({ props }) { return <div className="jr-metric"><span>{props.label}</span><strong>{props.value ?? '—'}</strong></div>; }
function valueOf(source, row, field) { return source.relation_labels?.[field.name]?.[row.data?.[field.name]] ?? row.data?.[field.name] ?? '—'; }
function displayValue(source, row, field) {
  const value = valueOf(source, row, field);
  if (value === '—') return value;
  if (field.type === 'file' && value) return String(value).startsWith('/api/public/') ? <img className="jr-image" src={String(value)} alt={field.label || field.name} loading="lazy" /> : <a href={String(value)} target="_blank" rel="noreferrer">查看附件</a>;
  if (field.type === 'url' && value) return <a href={String(value)} target="_blank" rel="noreferrer">{String(value)}</a>;
  return String(value);
}
function Input({ field, members }) {
  const common = { name: field.name, required: field.required, className: 'input input-sm' };
  if (field.type === 'bool') return <select {...common} className="select select-bordered select-sm"><option value="">请选择</option><option value="true">是</option><option value="false">否</option></select>;
  if (field.type === 'select') return <select {...common} className="select select-bordered select-sm"><option value="">请选择</option>{(field.options || []).map((item) => <option key={item} value={item}>{item}</option>)}</select>;
  if (field.type === 'member') return <select {...common} className="select select-bordered select-sm"><option value="">请选择成员</option>{members.map((member) => <option key={member.id} value={member.id}>{member.label}</option>)}</select>;
  if (field.type === 'file') return <input {...common} type="file" accept="image/*,application/pdf,text/plain" />;
  return <input {...common} type={field.type === 'number' ? 'number' : ['date', 'email', 'url'].includes(field.type) ? field.type : 'text'} />;
}
function formDataFor(form, fields) {
  const data = {};
  for (const field of fields) {
    const control = form.elements[field.name];
    if (!control || control.value === '') continue;
    data[field.name] = field.type === 'number' ? Number(control.value) : field.type === 'bool' ? control.value === 'true' : control.value;
  }
  return data;
}
function SourceView({ props, mode, onAction, onCreate, onUpdate, onDelete }) {
  const source = mode.sources?.[props.source] || {};
  const fields = source.fields || [], rows = source.items || [], writable = !mode.readOnly;
  const [editing, setEditing] = useState(null);
  const createFields = source.create_form_fields || fields;
  const submit = (event) => { event.preventDefault(); const files = Object.fromEntries(createFields.filter((field) => field.type === 'file').map((field) => [field.name, event.currentTarget.elements[field.name]?.files?.[0]]).filter(([, file]) => file)); onCreate?.(source, formDataFor(event.currentTarget, createFields), files); };
  const saveEdit = (event) => { event.preventDefault(); onUpdate?.(source, editing, formDataFor(event.currentTarget, fields)); setEditing(null); };
  const form = writable ? <details><summary className="btn btn-primary btn-sm">新增记录</summary><form onSubmit={submit}>{createFields.map((field) => <label key={field.name}>{field.label || field.name}<Input field={field} members={mode.members || []} /></label>)}<button className="btn btn-primary btn-sm" type="submit">保存</button></form></details> : null;
  const editForm = editing ? <details open><summary className="btn btn-ghost btn-xs">编辑记录</summary><form onSubmit={saveEdit}>{fields.map((field) => <label key={field.name}>{field.label || field.name}<Input field={{ ...field, required: false }} members={mode.members || []} /></label>)}<button className="btn btn-primary btn-xs" type="submit">保存修改</button></form></details> : null;
  if (props.variant === 'detail') { const row = rows[0]; return <section className="jr-source"><h3>{props.title || source.collection || ''}</h3>{row ? <dl className="jr-detail">{fields.map((field) => <div key={field.name}><dt>{field.label || field.name}</dt><dd>{displayValue(source, row, field)}</dd></div>)}</dl> : <p className="jr-empty">暂无记录</p>}</section>; }
  if (props.variant === 'form') return <section className="jr-source"><h3>{props.title || '新增记录'}</h3>{form || <p className="jr-empty">只读界面不可新增记录</p>}</section>;
  const cards = props.variant === 'cards';
  const rowActions = (row) => writable ? <div className="jr-actions"><button className="btn btn-ghost btn-xs" type="button" onClick={() => setEditing(row)}>编辑</button><button className="btn btn-error btn-outline btn-xs" type="button" onClick={() => onDelete?.(source, row)}>删除</button>{source.actions?.map((action) => <button className="btn btn-ghost btn-xs" key={action.id} type="button" onClick={() => onAction?.(source, action, row)}>{action.label}</button>)}</div> : null;
  return <section className="jr-source"><h3>{props.title || source.collection || ''}</h3>{editForm}{cards ? <div className="jr-cards">{rows.map((row) => <article className="jr-card" key={row.id}>{fields.map((field) => <div key={field.name}><strong>{field.label || field.name}</strong><span>{displayValue(source, row, field)}</span></div>)}{rowActions(row)}</article>)}</div> : <div className="jr-table-scroll"><table className="table table-sm"><thead><tr>{fields.map((field) => <th key={field.name}>{field.label || field.name}</th>)}{writable && <th>操作</th>}</tr></thead><tbody>{rows.map((row) => <tr key={row.id}>{fields.map((field) => <td key={field.name}>{displayValue(source, row, field)}</td>)}{writable && <td>{rowActions(row)}</td>}</tr>)}</tbody></table></div>}{!rows.length && <p className="jr-empty">暂无记录</p>}{form}<small>{source.total_items ?? rows.length} 条记录</small></section>;
}
const registry = { Page, Section, Text, Metric, RecordTable: (context) => <SourceView {...context} />, RecordCards: (context) => <SourceView {...context} props={{ ...context.props, variant: 'cards' }} />, RecordDetail: (context) => <SourceView {...context} props={{ ...context.props, variant: 'detail' }} />, RecordForm: (context) => <SourceView {...context} props={{ ...context.props, variant: 'form' }} /> };
export function mount(root, spec, { sources = {}, members = [], readOnly = false, onAction, onCreate, onUpdate, onDelete } = {}) {
  const reactRoot = createRoot(root);
  const scopedRegistry = Object.fromEntries(Object.entries(registry).map(([key, Component]) => [key, (context) => <Component {...context} mode={{ sources, members, readOnly }} onAction={readOnly ? undefined : onAction} onCreate={readOnly ? undefined : onCreate} onUpdate={readOnly ? undefined : onUpdate} onDelete={readOnly ? undefined : onDelete} />]));
  reactRoot.render(<StateProvider initialState={{ sources }}><VisibilityProvider><Renderer spec={spec} registry={scopedRegistry} /></VisibilityProvider></StateProvider>);
  return () => reactRoot.unmount();
}
