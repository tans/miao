// ========== 全局组件库 ==========

// 数据边界处理：确保返回数组
function ensureArray(data) {
  if (Array.isArray(data)) return data;
  if (data === null || data === undefined) return [];
  if (typeof data === 'object') return [data];
  return [];
}

// Loading 遮罩
function showLoading(text = '加载中...') {
  let overlay = document.getElementById('loading-overlay');
  if (!overlay) {
    overlay = document.createElement('div');
    overlay.id = 'loading-overlay';
    overlay.className = 'position-fixed top-0 start-0 w-100 h-100 d-flex justify-content-center align-items-center';
    overlay.style.background = 'rgba(0,0,0,0.5)';
    overlay.style.zIndex = '9998';
    document.body.appendChild(overlay);
  }
  overlay.innerHTML = `
    <div class="text-center text-white">
      <div class="spinner-border mb-2" role="status">
        <span class="visually-hidden">Loading...</span>
      </div>
      <div>${text}</div>
    </div>
  `;
  overlay.style.display = 'flex';
}

function hideLoading() {
  const overlay = document.getElementById('loading-overlay');
  if (overlay) {
    overlay.style.display = 'none';
    overlay.classList.remove('d-flex');
  }
}

// Toast 提示
function showToast(message, type = 'info') {
  const container = document.getElementById('toast-container') || createToastContainer();
  const bgClass = type === 'error' ? 'danger' : type === 'success' ? 'success' : type === 'warning' ? 'warning' : 'primary';

  const toast = document.createElement('div');
  toast.className = `toast align-items-center text-white bg-${bgClass} border-0`;
  toast.setAttribute('role', 'alert');
  toast.setAttribute('aria-live', 'assertive');
  toast.setAttribute('aria-atomic', 'true');

  toast.innerHTML = `
    <div class="d-flex">
      <div class="toast-body">${message}</div>
      <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast" aria-label="Close"></button>
    </div>
  `;

  container.appendChild(toast);
  const bsToast = new bootstrap.Toast(toast, { delay: 3000 });
  bsToast.show();
  toast.addEventListener('hidden.bs.toast', () => toast.remove());
}

function createToastContainer() {
  const container = document.createElement('div');
  container.id = 'toast-container';
  container.className = 'toast-container position-fixed top-0 end-0 p-3';
  container.style.zIndex = '9999';
  document.body.appendChild(container);
  return container;
}

function showSuccess(message) { showToast(message, 'success'); }
function showError(message) { showToast(message, 'error'); }
function showInfo(message) { showToast(message, 'info'); }
function showWarning(message) { showToast(message, 'warning'); }

// 确认对话框
function confirmDialog(message, callback, title = '确认操作') {
  const modalId = 'confirm-modal-' + Date.now();
  const modalHtml = `
    <div class="modal fade" id="${modalId}" tabindex="-1" aria-hidden="true">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">${title}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
          </div>
          <div class="modal-body">
            <p>${message}</p>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
            <button type="button" class="btn btn-primary" id="${modalId}-confirm">确认</button>
          </div>
        </div>
      </div>
    </div>
  `;

  document.body.insertAdjacentHTML('beforeend', modalHtml);
  const modalEl = document.getElementById(modalId);
  const modal = new bootstrap.Modal(modalEl);

  document.getElementById(`${modalId}-confirm`).addEventListener('click', () => {
    modal.hide();
    if (callback) callback();
  });

  modalEl.addEventListener('hidden.bs.modal', () => {
    modalEl.remove();
  });

  modal.show();
}

// 提示对话框
function alertDialog(message, title = '提示') {
  const modalId = 'alert-modal-' + Date.now();
  const modalHtml = `
    <div class="modal fade" id="${modalId}" tabindex="-1" aria-hidden="true">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">${title}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
          </div>
          <div class="modal-body">
            <p>${message}</p>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-primary" data-bs-dismiss="modal">确定</button>
          </div>
        </div>
      </div>
    </div>
  `;

  document.body.insertAdjacentHTML('beforeend', modalHtml);
  const modalEl = document.getElementById(modalId);
  const modal = new bootstrap.Modal(modalEl);

  modalEl.addEventListener('hidden.bs.modal', () => {
    modalEl.remove();
  });

  modal.show();
}

// ========== API 请求 ==========
const API_BASE = '/api/v1';

// Support both legacy relative endpoints and already-prefixed API paths.
function buildApiUrl(endpoint) {
  if (endpoint.startsWith('http://') || endpoint.startsWith('https://')) {
    return endpoint;
  }

  if (endpoint === API_BASE || endpoint.startsWith(`${API_BASE}/`)) {
    return endpoint;
  }

  if (endpoint.startsWith('/')) {
    return `${API_BASE}${endpoint}`;
  }

  return `${API_BASE}/${endpoint}`;
}

function apiRequest(endpoint, method = 'GET', body = null, showLoadingFlag = true) {
  if (showLoadingFlag) showLoading();

  const token = localStorage.getItem('token');
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  return fetch(buildApiUrl(endpoint), {
    method,
    headers,
    body: body ? JSON.stringify(body) : null
  })
    .then(res => {
      // 先解析 JSON，无论状态码如何
      return res.json().then(data => {
        if (!res.ok) {
          if (res.status === 401) {
            // 在登录页面，返回错误信息而不是抛出异常
            if (window.location.pathname.includes('/auth/login.html')) {
              return data; // 返回包含错误信息的 JSON
            }

            // 非登录页面，清除过期token并跳转
            localStorage.removeItem('token');
            localStorage.removeItem('user_id');
            localStorage.removeItem('username');
            localStorage.removeItem('role');
            showError('登录已过期，请重新登录');
            setTimeout(() => window.location.href = '/auth/login.html', 1500);
            throw new Error('Unauthorized');
          }
          // 其他错误状态码（409等）也返回data让调用者处理
          return data;
        }
        return data;
      });
    })
    .finally(() => {
      if (showLoadingFlag) hideLoading();
    });
}

function handleApiError(err) {
  console.error('API Error:', err);
  showError('请求失败，请稍后重试');
}

function storeAuthSession(authData, selectedRole = 'business') {
  const user = authData && authData.user ? authData.user : {};
  const roles = user.is_admin ? 'business,creator,admin' : 'business,creator';

  localStorage.setItem('token', authData.token);
  localStorage.setItem('user_id', user.id);
  localStorage.setItem('username', user.username);
  localStorage.setItem('role', selectedRole);
  localStorage.setItem('roles', roles);
  localStorage.setItem('is_admin', String(!!user.is_admin));
  localStorage.setItem('current_role', selectedRole);

  if (typeof initializeRole === 'function') {
    initializeRole({ roles, role: selectedRole });
  }
}

function redirectToDashboard(role) {
  if (role === 'creator') {
    window.location.href = '/creator/dashboard.html';
    return;
  }
  if (role === 'admin') {
    window.location.href = '/admin/dashboard.html';
    return;
  }
  window.location.href = '/business/dashboard.html';
}

function isLoggedIn() {
  return !!localStorage.getItem('token');
}

function getCurrentRole() {
  // All users have both business and creator capabilities
  // Return 'business' by default for UI navigation
  const current = localStorage.getItem('current_role');
  if (current) return current;

  // Check if admin
  const isAdmin = localStorage.getItem('is_admin') === 'true';
  if (isAdmin) return 'admin';

  // Default to business for regular users
  return 'business';
}

function getUserRole() {
  return getCurrentRole();
}

function isAdmin() {
  return localStorage.getItem('is_admin') === 'true';
}

function requireAuth() {
  if (!isLoggedIn()) {
    window.location.href = '/auth/login.html';
    return false;
  }
  return true;
}

function logout() {
  localStorage.removeItem('token');
  localStorage.removeItem('user_id');
  localStorage.removeItem('username');
  localStorage.removeItem('is_admin');
  localStorage.removeItem('current_role');
  localStorage.removeItem('role');
  localStorage.removeItem('roles');
  showInfo('已退出登录');
  setTimeout(() => window.location.href = '/auth/login.html', 500);
}
