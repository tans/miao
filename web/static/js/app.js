const API_BASE = '/api/v1';

// ========== Toast 通知系统 ==========
function showToast(message, type = 'info') {
  const container = document.getElementById('toast-container') || createToastContainer();
  const toast = document.createElement('div');
  toast.className = `toast align-items-center text-white bg-${type === 'error' ? 'danger' : type === 'success' ? 'success' : 'primary'} border-0`;
  toast.setAttribute('role', 'alert');
  toast.innerHTML = `
    <div class="d-flex">
      <div class="toast-body">${message}</div>
      <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast"></button>
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

// ========== API 请求 ==========
function apiRequest(endpoint, method = 'GET', body = null, showLoadingFlag = true) {
  if (showLoadingFlag) showLoading();

  const token = localStorage.getItem('token');
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  return fetch(`${API_BASE}${endpoint}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : null
  })
    .then(res => {
      if (!res.ok) {
        if (res.status === 401) {
          showError('登录已过期，请重新登录');
          setTimeout(() => logout(), 1500);
          throw new Error('Unauthorized');
        }
        throw new Error(`HTTP ${res.status}`);
      }
      return res.json();
    })
    .finally(() => {
      if (showLoadingFlag) hideLoading();
    });
}

function handleApiError(err) {
  console.error('API Error:', err);
  showError('请求失败，请稍后重试');
}

function isLoggedIn() {
  return !!localStorage.getItem('token');
}

function getUserRole() {
  return localStorage.getItem('role');
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
  localStorage.removeItem('role');
  if (typeof bootstrap !== 'undefined') {
    showInfo('已退出登录');
    setTimeout(() => window.location.href = '/auth/login.html', 500);
  } else {
    window.location.href = '/auth/login.html';
  }
}