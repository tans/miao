import React from 'react';
import { createRoot } from 'react-dom/client';
import { Renderer, StateProvider, VisibilityProvider } from '@json-render/react';

function Page({ props, children }) { return <main className="jr-page"><h2>{props.title || ''}</h2>{children}</main>; }
function Section({ props, children }) { return <section className="jr-section">{props.title && <h3>{props.title}</h3>}{children}</section>; }
function Text({ props }) { return <p className="jr-text">{props.text ?? ''}</p>; }
function Metric({ props }) { return <div className="jr-metric"><span>{props.label}</span><strong>{props.value ?? '—'}</strong></div>; }
function valueOf(source, row, field) { return source.relation_labels?.[field.name]?.[row.data?.[field.name]] ?? row.data?.[field.name] ?? '—'; }
function SourceView({ props, mode, onAction }) {
  const source = mode.sources?.[props.source] || {};
  const fields = source.fields || [], rows = source.items || [];
  if (props.variant === 'detail') {
    const row = rows[0];
    return <section className="jr-source"><h3>{props.title || source.collection || ''}</h3>{row ? <dl className="jr-detail">{fields.map((field) => <div key={field.name}><dt>{field.label || field.name}</dt><dd>{String(valueOf(source, row, field))}</dd></div>)}</dl> : <p className="jr-empty">暂无记录</p>}</section>;
  }
  if (props.variant === 'form') return <section className="jr-source"><h3>{props.title || '新增记录'}</h3><form onSubmit={(event) => { event.preventDefault(); onAction?.(source, { id: 'records.create', label: '新增记录' }, { id: '', data: Object.fromEntries(new FormData(event.currentTarget)) }); }}>{(source.create_form_fields || fields).map((field) => <label key={field.name}>{field.label || field.name}<input className="input input-sm" name={field.name} required={field.required} type={field.type === 'number' ? 'number' : field.type === 'email' ? 'email' : 'text'} /></label>)}<button className="btn btn-primary btn-sm" type="submit">保存</button></form></section>;
  const cards = props.variant === 'cards';
  return <section className="jr-source"><h3>{props.title || source.collection || ''}</h3>{cards ? <div className="jr-cards">{rows.map((row) => <article className="jr-card" key={row.id}>{fields.map((field) => <div key={field.name}><strong>{field.label || field.name}</strong><span>{String(valueOf(source, row, field))}</span></div>)}{source.actions?.map((action) => <button className="btn btn-ghost btn-xs" key={action.id} onClick={() => onAction?.(source, action, row)}>{action.label}</button>)}</article>)}</div> : <div className="jr-table-scroll"><table className="table table-sm"><thead><tr>{fields.map((f) => <th key={f.name}>{f.label || f.name}</th>)}</tr></thead><tbody>{rows.map((row) => <tr key={row.id}>{fields.map((field) => <td key={field.name}>{String(valueOf(source, row, field))}</td>)}</tr>)}</tbody></table></div>}{!rows.length && <p className="jr-empty">暂无记录</p>}<small>{source.total_items ?? rows.length} 条记录</small></section>;
}
const registry = { Page, Section, Text, Metric,
  RecordTable: (context) => <SourceView {...context} />,
  RecordCards: (context) => <SourceView {...context} props={{ ...context.props, variant: 'cards' }} />,
  RecordDetail: (context) => <SourceView {...context} props={{ ...context.props, variant: 'detail' }} />,
  RecordForm: (context) => <SourceView {...context} props={{ ...context.props, variant: 'form' }} />,
};

export function mount(root, spec, { sources = {}, readOnly = false, onAction } = {}) {
  const reactRoot = createRoot(root);
  const scopedRegistry = Object.fromEntries(Object.entries(registry).map(([key, Component]) => [key, (context) => <Component {...context} mode={{ sources }} onAction={readOnly ? undefined : onAction} />]));
  reactRoot.render(<StateProvider initialState={{ sources }}><VisibilityProvider><Renderer spec={spec} registry={scopedRegistry} /></VisibilityProvider></StateProvider>);
  return () => reactRoot.unmount();
}
