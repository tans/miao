import { mount } from './modules/ui-renderer.bundle.js';

const slug = location.pathname.split('/').filter(Boolean)[1] || '';
const titleNode = document.querySelector('#site-title');
const navNode = document.querySelector('#site-navigation');
const contentNode = document.querySelector('#site-content');
const esc = (value) => String(value ?? '').replace(/[&<>"']/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]);
let unmount = null;
let pageNumber = 1;
let searchTerm = '';

async function api(path) {
  const response = await fetch(path, { headers: { Accept: 'application/json' }, credentials: 'omit', cache: 'no-store' });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || '公开页面暂不可用');
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
    titleNode.textContent = runtime.app_title || runtime.title || '公开页面';
    navNode.innerHTML = (runtime.pages || []).map((page) => `<button class="btn btn-sm ${page.id === runtime.public_page ? 'btn-primary' : 'btn-ghost'}" type="button" data-page="${esc(page.id)}">${esc(page.title || page.id)}</button>`).join('');
    navNode.querySelectorAll('[data-page]').forEach((button) => button.addEventListener('click', () => navigate(button.dataset.page)));
    const page = runtime.definition?.pages?.find((item) => item.id === runtime.public_page);
    if (!page?.spec) throw new Error('公开页面配置已失效');
    if (detail) {
      const source = runtime.sources?.[detail.source];
      if (!source) throw new Error('公开内容不存在');
      const row = await api(`/api/public/${encodeURIComponent(slug)}/records/${encodeURIComponent(detail.item)}?${new URLSearchParams({page_id: runtime.public_page, table: source.collection, source: detail.source})}`);
      runtime.sources[detail.source] = { ...source, items: [row], total_items: 1, total_pages: 1, page: 1 };
    }
    unmount?.();
    unmount = mount(contentNode, page.spec, {
      sources: runtime.sources,
      readOnly: true,
      search: searchTerm,
      onSearch: (_source, value) => { searchTerm = String(value || '').trim(); pageNumber = 1; load(); },
      onPage: (_source, value) => { pageNumber = Number(value) || 1; load(); },
    });
    const title = runtime.page_title || runtime.app_title || '公开页面';
    if (!detail) document.title = title;
    if (!detail) document.querySelector('meta[name="description"]').content = runtime.description || title;
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
