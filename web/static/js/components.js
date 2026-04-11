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
    overlay.style.background = 'rgba(255,255,255,0.95)';
    overlay.style.zIndex = '1090';
    document.body.appendChild(overlay);
  }
  overlay.innerHTML = `
    <div class="text-center">
      <div class="spinner-paw mb-3">
        <div class="spinner-css"></div>
      </div>
      <p class="text-muted small mb-0">${text}</p>
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

  // 自定义图标
  const icons = {
    success: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" fill="currentColor" viewBox="0 0 16 16"><path d="M16 8A8 8 0 1 1 0 8a8 8 0 0 1 16 0zm-3.97-3.03a.75.75 0 0 0-1.08.022L7.477 9.417 5.384 7.323a.75.75 0 0 0-1.06 1.06L6.97 11.03a.75.75 0 0 0 1.079-.02l3.992-4.99a.75.75 0 0 0-.01-1.05z"/></svg>',
    error: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" fill="currentColor" viewBox="0 0 16 16"><path d="M16 8A8 8 0 1 1 0 8a8 8 0 0 1 16 0zM5.354 4.646a.5.5 0 1 0-.708.708L7.293 8l-2.647 2.646a.5.5 0 0 0 .708.708L8 8.707l2.646 2.647a.5.5 0 0 0 .708-.708L8.707 8l2.647-2.646a.5.5 0 0 0-.708-.708L8 7.293 5.354 4.646z"/></svg>',
    warning: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" fill="currentColor" viewBox="0 0 16 16"><path d="M8.982 1.566a1.13 1.13 0 0 0-1.96 0L.165 13.233c-.457.778.091 1.767.98 1.767h13.713c.889 0 1.438-.99.98-1.767L8.982 1.566zM8 5c.535 0 .954.462.9.995l-.35 3.507a.552.552 0 0 1-1.1 0L7.1 5.995A.905.905 0 0 1 8 5zm.002 6a1 1 0 1 1 0 2 1 1 0 0 1 0-2z"/></svg>',
    info: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" fill="currentColor" viewBox="0 0 16 16"><path d="M8 16A8 8 0 1 0 8 0a8 8 0 0 0 0 16zm.93-9.412-1 4.705c-.07.34.029.533.304.533.194 0 .487-.07.686-.246l-.088.416c-.287.346-.92.598-1.465.598-.703 0-1.002-.422-.808-1.319l.738-3.468c.064-.293.006-.399-.287-.47l-.451-.081.082-.381 2.29-.287zM8 5.5a1 1 0 1 1 0-2 1 1 0 0 1 0 2z"/></svg>'
  };

  const colors = {
    success: { bg: '#d4edda', border: '#28a745', text: '#155724', icon: '#28a745' },
    error: { bg: '#f8d7da', border: '#dc3545', text: '#721c24', icon: '#dc3545' },
    warning: { bg: '#fff3cd', border: '#ffc107', text: '#856404', icon: '#ffc107' },
    info: { bg: '#d1ecf1', border: '#17a2b8', text: '#0c5460', icon: '#17a2b8' }
  };

  const color = colors[type] || colors.info;
  const icon = icons[type] || icons.info;

  const toast = document.createElement('div');
  toast.style.cssText = `
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px 20px;
    background: ${color.bg};
    border-left: 4px solid ${color.border};
    border-radius: 8px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.15);
    margin-bottom: 8px;
    animation: slideInRight 0.3s ease-out;
    max-width: 350px;
  `;
  toast.setAttribute('role', 'alert');
  toast.setAttribute('aria-live', 'assertive');

  toast.innerHTML = `
    <span style="color: ${color.icon}; flex-shrink: 0;">${icon}</span>
    <span style="color: ${color.text}; flex: 1; font-size: 14px; line-height: 1.4;">${message}</span>
    <button type="button" style="background: none; border: none; color: ${color.text}; opacity: 0.5; cursor: pointer; padding: 4px;" data-bs-dismiss="toast" aria-label="Close">
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" fill="currentColor" viewBox="0 0 16 16"><path d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"/></svg>
    </button>
  `;

  container.appendChild(toast);

  // 自动消失
  setTimeout(() => {
    toast.style.animation = 'fadeOut 0.3s ease-out forwards';
    setTimeout(() => { if (toast.parentNode) toast.remove(); }, 300);
  }, 3000);
}

function createToastContainer() {
  const container = document.createElement('div');
  container.id = 'toast-container';
  container.className = 'toast-container position-fixed top-0 end-0 p-3';
  container.style.zIndex = '1090';
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

  const token = localStorage.getItem('admin_token') || localStorage.getItem('token');
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

  // 设置 cookie（用于服务端渲染页面认证）
  document.cookie = `token=${authData.token}; path=/; max-age=${7*24*60*60}`;

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
