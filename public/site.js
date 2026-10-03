const params = new URLSearchParams(location.search);
const slug = location.pathname.split('/').filter(Boolean)[1] || '';
const titleNode = document.querySelector('#site-title');
const navNode = document.querySelector('#site-navigation');
const contentNode = document.querySelector('#site-content');
const esc = (value) => String(value ?? '').replace(/[&<>"']/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]);
const valueText = (value) => value === null || value === undefined || value === '' ? '—' : typeof value === 'boolean' ? (value ? '是' : '否') : Array.isArray(value) ? value.join('、') : String(value);
const pageFromURL = () => new URLSearchParams(location.search).get('page') || '';
let paginationPage = 1;
let frame = null;
let frameSession = null;

async function api(path) {
  const response = await fetch(path, { headers: { Accept: 'application/json' }, credentials: 'omit', cache: 'no-store' });
  const result = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(result.error || '公开页面暂时无法载入');
  return result;
}

function navigate(id) {
  const next = new URL(location.href);
  next.searchParams.set('page', id);
  history.pushState({}, '', next);
  paginationPage = 1;
  load();
}

function renderNavigation(runtime) {
  const activePage = runtime.public_page || runtime.page;
  navNode.innerHTML = (runtime.pages || []).map((page) => `<button class="btn btn-sm ${page.id === activePage ? 'btn-primary' : 'btn-ghost'}" type="button" data-page="${esc(page.id)}">${esc(page.title || page.id)}</button>`).join('');
  navNode.querySelectorAll('[data-page]').forEach((button) => button.addEventListener('click', () => navigate(button.dataset.page)));
}

function renderSchema(runtime) {
  const fields = runtime.fields || [];
  const rows = runtime.items || [];
  const cards = rows.map((row) => `<article class="card bg-base-100 site-record"><div class="card-body"><dl>${fields.map((field) => `<div><dt>${esc(field.label || field.name)}</dt><dd>${field.type === 'url' && safeURL(row.data[field.name]) ? `<a class="link" href="${esc(safeURL(row.data[field.name]))}" rel="nofollow noopener noreferrer" target="_blank">${esc(row.data[field.name])}</a>` : esc(valueText(row.data[field.name]))}</dd></div>`).join('')}</dl></div></article>`).join('');
  contentNode.innerHTML = `<header class="site-hero"><span class="badge badge-outline">${esc(runtime.page_title || runtime.title || '')}</span><h1>${esc(runtime.title || runtime.app_title)}</h1><p>${esc(runtime.description || '')}</p></header>${rows.length ? `<section class="site-record-grid" aria-label="公开内容">${cards}</section>` : '<div class="alert site-error"><span>目前还没有可展示的公开内容。</span></div>'}<div class="site-pager"><button class="btn btn-sm btn-outline" data-prev ${runtime.page <= 1 ? 'disabled' : ''}>上一页</button><span class="text-sm opacity-70">第 ${runtime.page} / ${Math.max(1, runtime.total_pages)} 页</span><button class="btn btn-sm btn-outline" data-next ${runtime.page >= runtime.total_pages ? 'disabled' : ''}>下一页</button></div>`;
  contentNode.querySelector('[data-prev]')?.addEventListener('click', () => { paginationPage--; load(); });
  contentNode.querySelector('[data-next]')?.addEventListener('click', () => { paginationPage++; load(); });
}

function escapeScript(value) {
  return String(value).replace(/<\/script/gi, '<\\/script').replace(/<!--/g, '<\\!--');
}

function safeURL(value) {
  try {
    const url = new URL(String(value));
    return ['https:', 'http:'].includes(url.protocol) ? url.href : '';
  } catch { return ''; }
}

function sourceDocument(runtime) {
  const nonce = crypto.randomUUID();
  const entry = runtime.source?.['entry.html'] || '';
  const styles = runtime.source?.['styles.css'] ? `<style>${runtime.source['styles.css']}</style>` : '';
  const script = runtime.source?.['app.js'] || '';
  const bridge = `(function(){const nonce=${JSON.stringify(nonce)},page=${JSON.stringify(runtime.page)};let next=0;const pending=new Map();function call(action,args){return new Promise((resolve,reject)=>{const id=++next;pending.set(id,{resolve,reject});parent.postMessage({protocol:'miao-public-v1',nonce,page,id,action,args},'*')})}window.miao={query:(table,options={})=>call('records.read',{table,...options}),navigate:path=>call('navigation.go',{path})};addEventListener('message',event=>{const m=event.data;if(!m||m.protocol!=='miao-public-v1-result'||m.nonce!==nonce)return;const p=pending.get(m.id);if(!p)return;pending.delete(m.id);m.ok?p.resolve(m.result):p.reject(new Error(m.error||'读取失败'))});parent.postMessage({protocol:'miao-public-v1-ready',nonce,page},'*')})();`;
  const headPolicy = `<meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; connect-src 'none'; object-src 'none'; form-action 'none'; base-uri 'none'; frame-src 'none'; navigate-to 'none"><meta name="viewport" content="width=device-width,initial-scale=1">${styles}`;
  let html = entry
    .replace(/<script\b[^>]*src=["'][^"']*app\.js[^"']*["'][^>]*>\s*<\/script>/i, '')
    .replace(/<script\b[^>]*src=["'][^"']*["'][^>]*>\s*<\/script>/gi, '')
    .replace(/<link\b[^>]*href=["'][^"']*["'][^>]*>/gi, '')
    .replace(/<head([^>]*)>/i, `<head$1>${headPolicy}`);
  if (!/<head\b/i.test(html)) html = `<!doctype html><html><head>${headPolicy}</head><body>${html}</body></html>`;
  const inlineScript = `<script>${escapeScript(bridge + script)}</script>`;
  if (/<\/body>/i.test(html)) html = html.replace(/<\/body>/i, `${inlineScript}</body>`);
  else if (/<\/html>/i.test(html)) html = html.replace(/<\/html>/i, `${inlineScript}</html>`);
  else html += inlineScript;
  return { nonce, html };
}

function mountSource(runtime) {
  contentNode.innerHTML = '<iframe class="site-frame" title="公开应用页面" sandbox="allow-scripts" referrerpolicy="no-referrer"></iframe>';
  frame = contentNode.querySelector('iframe');
  const document = sourceDocument(runtime);
  frameSession = { page: runtime.page, nonce: document.nonce, reads: runtime.reads || [], pages: runtime.pages || [] };
  frame.srcdoc = document.html;
}

async function handleSourceMessage(event) {
  if (!frame || event.source !== frame.contentWindow || !frameSession) return;
  const message = event.data;
  if (!message || message.protocol !== 'miao-public-v1' || message.page !== frameSession.page || message.nonce !== frameSession.nonce || !Number.isSafeInteger(message.id)) return;
  if (message.action === 'navigation.go') {
    if ((frameSession.pages || []).some((page) => page.id === message.args?.path)) navigate(message.args.path);
    frame.contentWindow.postMessage({ protocol: 'miao-public-v1-result', nonce: message.nonce, id: message.id, ok: true, result: null }, '*');
    return;
  }
  if (message.action !== 'records.read') return;
  const grant = frameSession.reads.find((item) => item.table === message.args?.table);
  if (!grant) {
    frame.contentWindow.postMessage({ protocol: 'miao-public-v1-result', nonce: message.nonce, id: message.id, ok: false, error: '此页面未获授权读取该数据表' }, '*');
    return;
  }
  try {
    const query = new URLSearchParams({ page_id: frameSession.page, table: grant.table });
    for (const key of ['page', 'perPage', 'search']) if (message.args[key] !== undefined) query.set(key, String(message.args[key]));
    const result = await api(`/api/public/${encodeURIComponent(slug)}/records?${query}`);
    if (!frame || event.source !== frame.contentWindow) return;
    frame.contentWindow.postMessage({ protocol: 'miao-public-v1-result', nonce: message.nonce, id: message.id, ok: true, result }, '*');
  } catch (error) {
    frame.contentWindow.postMessage({ protocol: 'miao-public-v1-result', nonce: message.nonce, id: message.id, ok: false, error: error.message }, '*');
  }
}

window.addEventListener('message', handleSourceMessage);
window.addEventListener('popstate', () => { paginationPage = 1; load(); });

async function load() {
  try {
    const query = new URLSearchParams();
    const selected = pageFromURL();
    if (selected) query.set('page', selected);
    if (paginationPage > 1) query.set('page_number', String(paginationPage));
    const runtime = await api(`/api/public/${encodeURIComponent(slug)}/runtime?${query}`);
    titleNode.textContent = runtime.app_title || runtime.title || '公开页面';
    updateMetadata(runtime);
    renderNavigation(runtime);
    if (runtime.source) mountSource(runtime);
    else renderSchema(runtime);
  } catch (error) {
    contentNode.innerHTML = `<div class="alert alert-warning site-error"><span>${esc(error.message)}</span></div>`;
  }
}

function updateMetadata(runtime) {
  const title = runtime.page_title || runtime.app_title || runtime.title || '公开页面';
  const descriptionText = runtime.description || `${title} · MIAO`;
  document.title = title;
  const description = document.querySelector('meta[name="description"]');
  if (description) description.content = descriptionText;
  const canonical = new URL(`/s/${encodeURIComponent(slug)}`, location.origin);
  const selectedPage = runtime.public_page || runtime.page;
  if (selectedPage) canonical.searchParams.set('page', selectedPage);
  let canonicalLink = document.querySelector('link[rel="canonical"]');
  if (!canonicalLink) {
    canonicalLink = document.createElement('link');
    canonicalLink.rel = 'canonical';
    document.head.append(canonicalLink);
  }
  canonicalLink.href = canonical.href;
  setMetaProperty('og:title', title);
  setMetaProperty('og:description', descriptionText);
  setMetaProperty('og:type', 'website');
  setMetaProperty('og:url', canonical.href);
}

function setMetaProperty(property, content) {
  let node = document.querySelector(`meta[property="${property}"]`);
  if (!node) {
    node = document.createElement('meta');
    node.setAttribute('property', property);
    document.head.append(node);
  }
  node.content = content;
}

load();
