// ========== 全局组件库 ==========

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
  if (overlay) overlay.style.display = 'none';
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
