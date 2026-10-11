import { t, applyTranslations } from './modules/i18n.js';

applyTranslations();

const slug = location.pathname.split('/').filter(Boolean)[1] || '';
const titleNode = document.querySelector('#site-title');
const navNode = document.querySelector('#site-navigation');
const contentNode = document.querySelector('#site-content');
const esc = (value) => String(value ?? '').replace(/[&<>"']/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]);
let unmount = null;
let pageNumber = 1;
let searchTerm = '';
let visitCounted = false;
let rendererPromise = null;

function loadRenderer() {
  if (!rendererPromise) {
    rendererPromise = import('./modules/ui-renderer.bundle.js').catch((error) => {
      rendererPromise = null;
      throw error;
    });
  }
  return rendererPromise;
}

async function api(path) {
  const response = await fetch(path, { headers: { Accept: 'application/json' }, credentials: 'omit', cache: 'no-store' });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || t('公开页面暂不可用'));
  return data;
}

function currentPage() {
  const parts = location.pathname.split('/').filter(Boolean);
  return parts[2] || new URLSearchParams(location.search).get('page') || '';
}

function navigate(page) {
  history.pushState({}, '', `/s/${encodeURIComponent(slug)}/${encodeURIComponent(page)}`);
  pageNumber = 1;
  searchTerm = '';
  load();
}

function renderPublicForm(form, pageId) {
  const fields = (form.fields || []).filter((field) => ['text', 'email', 'url', 'number', 'date', 'select', 'bool'].includes(field.type));
  if (!fields.length) return;
  const section = document.createElement('section');
  section.className = 'public-form';
  section.innerHTML = `<h2>${esc(form.title || t('提交信息'))}</h2><form>${fields.map((field) => {
    const label = esc(field.label || field.name);
    if (field.type === 'select') return `<label>${label}<select name="${esc(field.name)}" ${field.required ? 'required' : ''}><option value=""></option>${(field.options || []).map((value) => `<option value="${esc(value)}">${esc(value)}</option>`).join('')}</select></label>`;
    if (field.type === 'bool') return `<label>${label}<input name="${esc(field.name)}" type="checkbox" value="true" ${field.required ? 'required' : ''}></label>`;
    return `<label>${label}<input name="${esc(field.name)}" type="${field.type === 'number' ? 'number' : field.type === 'date' ? 'date' : field.type}" ${field.required ? 'required' : ''} ${field.type === 'number' ? 'step="any"' : ''}></label>`;
  }).join('')}<label class="public-form-trap" aria-hidden="true">Website<input name="website" tabindex="-1" autocomplete="off"></label><button class="btn btn-primary" type="submit">${esc(t('提交'))}</button><p role="status"></p></form>`;
  contentNode.append(section);
  section.querySelector('form').addEventListener('submit', async (event) => {
    event.preventDefault();
    const formNode = event.currentTarget;
    const data = {};
    for (const field of fields) {
      const control = formNode.elements.namedItem(field.name);
      if (field.type === 'bool') data[field.name] = control.checked;
      else if (control.value !== '') data[field.name] = field.type === 'number' ? Number(control.value) : control.value;
    }
    const button = formNode.querySelector('button[type="submit"]');
    button.disabled = true;
    try {
      const response = await fetch(`/api/public/${encodeURIComponent(slug)}/forms/${encodeURIComponent(pageId)}`, { method: 'POST', headers: { 'Content-Type': 'application/json', Accept: 'application/json' }, credentials: 'omit', body: JSON.stringify({ data, website: formNode.elements.website.value }) });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.error || t('提交暂时失败，请稍后重试'));
      formNode.innerHTML = `<p role="status">${esc(t('感谢提交，我们会尽快联系你。'))}</p>`;
    } catch (error) { formNode.querySelector('[role="status"]').textContent = error.message; button.disabled = false; }
  });
}

function setMeta(property, content) {
  const element = document.querySelector(`meta[property="${property}"]`);
  if (element) element.content = content;
}

async function load() {
  const parts = location.pathname.split('/').filter(Boolean);
  const requestedPage = currentPage();
  const detail = parts.length === 5 ? { source: parts[3], item: parts[4] } : null;
  try {
    const query = new URLSearchParams();
    if (requestedPage) query.set('page', requestedPage);
    if (pageNumber > 1) query.set('page_number', String(pageNumber));
    if (searchTerm) query.set('search', searchTerm);
    const runtime = await api(`/api/public/${encodeURIComponent(slug)}/runtime?${query}`);
    if (!visitCounted) {
      visitCounted = true;
      fetch(`/api/public/${encodeURIComponent(slug)}/visit`, { method: 'POST' }).catch(() => {});
    }
    titleNode.textContent = runtime.app_title || runtime.title || t('公开页面');
    navNode.innerHTML = (runtime.pages || []).map((page) => `<button class="btn btn-sm ${page.id === runtime.public_page ? 'btn-primary' : 'btn-ghost'}" type="button" data-page="${esc(page.id)}">${esc(page.title || page.id)}</button>`).join('');
    navNode.querySelectorAll('[data-page]').forEach((button) => button.addEventListener('click', () => navigate(button.dataset.page)));
    const page = runtime.definition?.pages?.find((item) => item.id === runtime.public_page);
    if (!page?.spec) throw new Error(t('公开页面配置已失效'));
    let spec = page.spec;
    let detailSEO = null;
    if (detail) {
      const source = runtime.sources?.[detail.source];
      if (!source) throw new Error(t('公开内容不存在'));
      const row = await api(`/api/public/${encodeURIComponent(slug)}/records/${encodeURIComponent(detail.item)}?${new URLSearchParams({page_id: runtime.public_page, table: source.collection, source: detail.source})}`);
      runtime.sources[detail.source] = { ...source, items: [row], total_items: 1, total_pages: 1, page: 1 };
      spec = { root: 'detail', elements: { detail: { type: 'RecordDetail', props: { source: detail.source, title: runtime.page_title }, children: [] } } };
      detailSEO = row.seo;
    }
    unmount?.();
    const { mount } = await loadRenderer();
    unmount = mount(contentNode, spec, {
      sources: runtime.sources,
      readOnly: true,
      search: searchTerm,
      onSearch: detail ? undefined : (_source, value) => { searchTerm = String(value || '').trim(); pageNumber = 1; load(); },
      onPage: detail ? undefined : (_source, value) => { pageNumber = Number(value) || 1; load(); },
    });
    const title = runtime.page_title || runtime.app_title || t('公开页面');
    if (!detail && page.form?.fields?.length) renderPublicForm(page.form, runtime.public_page);
    document.title = detailSEO?.title || title;
    document.querySelector('meta[name="description"]').content = detailSEO?.description || runtime.description || title;
    setMeta('og:title', document.title);
    setMeta('og:description', document.querySelector('meta[name="description"]').content);
    setMeta('og:url', location.href);
    document.querySelector('link[rel="canonical"]').href = location.href;
  } catch (error) {
    unmount?.();
    unmount = null;
    contentNode.innerHTML = `<p class="site-error" role="alert">${esc(error.message)}</p>`;
  }
}

window.addEventListener('popstate', () => { pageNumber = 1; searchTerm = ''; load(); });
load();
