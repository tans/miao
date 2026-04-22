/* ========== 通用管理界面JS ========== */
/* 提取自各管理页面的通用函数 */

// ========== 认证检查 ==========
function checkAdminAuth() {
  const token = localStorage.getItem('token');
  const currentRole = localStorage.getItem('current_role');
  if (!token || currentRole !== 'admin') {
    window.location.href = '/admin/login.html';
    return false;
  }
  return true;
}

// ========== 设置用户名 ==========
function setAdminUsername() {
  const username = localStorage.getItem('username');
  const el = document.getElementById('admin-username');
  if (username && el) {
    el.textContent = username;
  }
}

// ========== 登出 ==========
function logout() {
  const keys = ['token', 'user_id', 'username', 'role', 'is_admin', 'current_role', 'admin_token'];
  keys.forEach(k => localStorage.removeItem(k));
  window.location.href = '/admin/login.html';
}

// ========== HTML转义 ==========
function escapeHtml(t) {
  const d = document.createElement('div');
  d.textContent = t;
  return d.innerHTML;
}

// ========== Toast通知 ==========
function showToast(msg, type) {
  const c = document.getElementById('toast-container') || createToastContainer();
  const id = 'toast-' + Date.now();
  const bgClass = type === 'success' ? 'bg-success' : type === 'danger' ? 'bg-danger' : type === 'warning' ? 'bg-warning' : 'bg-info';
  c.innerHTML += `<div id="${id}" class="toast align-items-center text-white ${bgClass} border-0" role="alert"><div class="d-flex"><div class="toast-body">${msg}</div><button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast"></button></div></div>`;
  const toastEl = c.lastElementChild;
  new bootstrap.Toast(toastEl).show();
  toastEl.addEventListener('hidden.bs.toast', () => toastEl.remove());
}

function createToastContainer() {
  const c = document.createElement('div');
  c.id = 'toast-container';
  c.className = 'toast-container position-fixed top-0 end-0 p-3';
  document.body.appendChild(c);
  return c;
}

// ========== 分页渲染 ==========
function renderPagination(totalPages, currentPage, loadFn) {
  const c = document.getElementById('pagination-container');
  if (!c) return;
  if (totalPages <= 1) {
    c.innerHTML = '';
    return;
  }
  let h = '<nav><ul class="pagination">';
  h += `<li class="page-item ${currentPage === 1 ? 'disabled' : ''}"><a class="page-link" href="#" onclick="${loadFn}(${currentPage - 1});return false;">上一页</a></li>`;
  for (let i = 1; i <= totalPages; i++) {
    if (i === 1 || i === totalPages || (i >= currentPage - 2 && i <= currentPage + 2)) {
      h += `<li class="page-item ${i === currentPage ? 'active' : ''}"><a class="page-link" href="#" onclick="${loadFn}(${i});return false;">${i}</a></li>`;
    } else if (i === currentPage - 3 || i === currentPage + 3) {
      h += '<li class="page-item disabled"><span class="page-link">...</span></li>';
    }
  }
  h += `<li class="page-item ${currentPage === totalPages ? 'disabled' : ''}"><a class="page-link" href="#" onclick="${loadFn}(${currentPage + 1});return false;">下一页</a></li></ul></nav>`;
  c.innerHTML = h;
}

// ========== 状态徽章类 ==========
function getStatusClass(s) {
  const map = {
    pending: 'bg-warning text-dark',
    approved: 'bg-success',
    published: 'bg-success',
    rejected: 'bg-danger',
    completed: 'bg-info',
    cancelled: 'bg-secondary',
    active: 'bg-success',
    disabled: 'bg-secondary'
  };
  return map[s] || 'bg-secondary';
}

function getStatusText(s) {
  const map = {
    pending: '待审核',
    approved: '已通过',
    published: '已发布',
    rejected: '已拒绝',
    completed: '已完成',
    cancelled: '已取消',
    active: '正常',
    disabled: '已禁用'
  };
  return map[s] || s;
}

function getRoleBadge(role) {
  const badges = { business: 'bg-primary', creator: 'bg-success', admin: 'bg-danger' };
  return badges[role] || 'bg-secondary';
}

function getRoleText(role) {
  const texts = { business: '商家', creator: '创作者', admin: '管理员' };
  return texts[role] || role;
}

function getClaimStatusBadge(status) {
  const map = { 1: 'bg-info', 2: 'bg-warning', 3: 'bg-success', 4: 'bg-secondary', 5: 'bg-danger' };
  return map[status] || 'bg-secondary';
}

function getReviewResultBadge(result) {
  if (result === 1) return 'bg-success';
  if (result === 2) return 'bg-danger';
  return 'bg-secondary';
}

function getTaskStatusBadge(status) {
  const map = { 1: 'bg-warning', 2: 'bg-success', 3: 'bg-secondary', 4: 'bg-danger', 5: 'bg-info' };
  return map[status] || 'bg-secondary';
}

function getTaskStatusText(status) {
  const map = { 1: '待审核', 2: '已上线', 3: '已下线', 4: '已取消', 5: '已结束' };
  return map[status] || '未知';
}

// ========== API请求封装 ==========
function adminFetch(url, options = {}) {
  const token = localStorage.getItem('token');
  const headers = { 'Authorization': `Bearer ${token}` };
  if (options.body && typeof options.body === 'object' && !(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
    options.body = JSON.stringify(options.body);
  }
  return fetch(url, { ...options, headers: { ...headers, ...options.headers } });
}

// ========== 初始化 ==========
document.addEventListener('DOMContentLoaded', function() {
  // 检查登录状态
  if (typeof checkAdminAuth === 'function' && !checkAdminAuth()) return;
  // 设置用户名
  setAdminUsername();
});
