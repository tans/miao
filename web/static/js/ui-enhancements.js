// ========== UI增强组件库 ==========

// ========== 骨架屏组件 ==========
const SkeletonLoader = {
  // 创建卡片骨架屏
  createCardSkeleton(count = 3) {
    let html = '';
    for (let i = 0; i < count; i++) {
      html += `
        <div class="col-md-4 mb-3">
          <div class="card skeleton-card">
            <div class="card-body">
              <div class="skeleton-line skeleton-title"></div>
              <div class="skeleton-line skeleton-text"></div>
              <div class="skeleton-line skeleton-text" style="width: 80%;"></div>
              <div class="skeleton-line skeleton-text" style="width: 60%;"></div>
            </div>
          </div>
        </div>
      `;
    }
    return html;
  },

  // 创建表格骨架屏
  createTableSkeleton(rows = 5, cols = 4) {
    let html = '<div class="skeleton-table"><table class="table"><tbody>';
    for (let i = 0; i < rows; i++) {
      html += '<tr>';
      for (let j = 0; j < cols; j++) {
        html += '<td><div class="skeleton-line"></div></td>';
      }
      html += '</tr>';
    }
    html += '</tbody></table></div>';
    return html;
  },

  // 创建列表骨架屏
  createListSkeleton(count = 5) {
    let html = '<div class="skeleton-list">';
    for (let i = 0; i < count; i++) {
      html += `
        <div class="skeleton-list-item">
          <div class="skeleton-line skeleton-title"></div>
          <div class="skeleton-line skeleton-text"></div>
        </div>
      `;
    }
    html += '</div>';
    return html;
  },

  // 显示骨架屏
  show(containerId, type = 'card', count = 3) {
    const container = document.getElementById(containerId);
    if (!container) return;

    let skeletonHtml = '';
    switch (type) {
      case 'card':
        skeletonHtml = this.createCardSkeleton(count);
        break;
      case 'table':
        skeletonHtml = this.createTableSkeleton(count);
        break;
      case 'list':
        skeletonHtml = this.createListSkeleton(count);
        break;
    }

    container.innerHTML = skeletonHtml;
  },

  // 隐藏骨架屏
  hide(containerId) {
    const container = document.getElementById(containerId);
    if (!container) return;
    container.innerHTML = '';
  }
};

// ========== 加载状态管理 ==========
const LoadingState = {
  // 显示按钮加载状态
  showButtonLoading(buttonId, loadingText = '处理中...') {
    const button = document.getElementById(buttonId);
    if (!button) return;

    button.disabled = true;
    button.dataset.originalText = button.innerHTML;
    button.innerHTML = `
      <span class="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>
      ${loadingText}
    `;
  },

  // 隐藏按钮加载状态
  hideButtonLoading(buttonId) {
    const button = document.getElementById(buttonId);
    if (!button) return;

    button.disabled = false;
    if (button.dataset.originalText) {
      button.innerHTML = button.dataset.originalText;
      delete button.dataset.originalText;
    }
  },

  // 显示内联加载指示器
  showInlineLoading(containerId, message = '加载中...') {
    const container = document.getElementById(containerId);
    if (!container) return;

    container.innerHTML = `
      <div class="text-center py-5">
        <div class="spinner-border text-primary mb-3" role="status">
          <span class="visually-hidden">Loading...</span>
        </div>
        <p class="text-muted">${message}</p>
      </div>
    `;
  }
};

// ========== 操作反馈动画 ==========
const FeedbackAnimation = {
  // 成功动画
  showSuccess(message, duration = 2000) {
    const toast = this.createToast('success', message);
    document.body.appendChild(toast);

    setTimeout(() => {
      toast.classList.add('show');
    }, 10);

    setTimeout(() => {
      toast.classList.remove('show');
      setTimeout(() => toast.remove(), 300);
    }, duration);
  },

  // 错误动画
  showError(message, duration = 3000) {
    const toast = this.createToast('danger', message);
    document.body.appendChild(toast);

    setTimeout(() => {
      toast.classList.add('show');
    }, 10);

    setTimeout(() => {
      toast.classList.remove('show');
      setTimeout(() => toast.remove(), 300);
    }, duration);
  },

  // 警告动画
  showWarning(message, duration = 2500) {
    const toast = this.createToast('warning', message);
    document.body.appendChild(toast);

    setTimeout(() => {
      toast.classList.add('show');
    }, 10);

    setTimeout(() => {
      toast.classList.remove('show');
      setTimeout(() => toast.remove(), 300);
    }, duration);
  },

  // 信息动画
  showInfo(message, duration = 2000) {
    const toast = this.createToast('info', message);
    document.body.appendChild(toast);

    setTimeout(() => {
      toast.classList.add('show');
    }, 10);

    setTimeout(() => {
      toast.classList.remove('show');
      setTimeout(() => toast.remove(), 300);
    }, duration);
  },

  // 创建Toast元素
  createToast(type, message) {
    const toast = document.createElement('div');
    toast.className = `toast-notification toast-${type}`;

    const icons = {
      success: '✓',
      danger: '✕',
      warning: '⚠',
      info: 'ℹ'
    };

    toast.innerHTML = `
      <span class="toast-icon">${icons[type]}</span>
      <span class="toast-message">${message}</span>
    `;

    return toast;
  },

  // 确认对话框
  confirm(message, onConfirm, onCancel) {
    const modal = document.createElement('div');
    modal.className = 'modal fade';
    modal.innerHTML = `
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">确认操作</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <p>${message}</p>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
            <button type="button" class="btn btn-primary" id="confirm-btn">确认</button>
          </div>
        </div>
      </div>
    `;

    document.body.appendChild(modal);
    const bsModal = new bootstrap.Modal(modal);
    bsModal.show();

    modal.querySelector('#confirm-btn').addEventListener('click', () => {
      bsModal.hide();
      if (onConfirm) onConfirm();
    });

    modal.addEventListener('hidden.bs.modal', () => {
      modal.remove();
      if (onCancel) onCancel();
    });
  }
};

// ========== 数据筛选和排序 ==========
const DataFilter = {
  // 创建筛选器UI
  createFilterUI(containerId, filters) {
    const container = document.getElementById(containerId);
    if (!container) return;

    let html = '<div class="filter-bar mb-3"><div class="row g-2">';

    filters.forEach(filter => {
      html += `
        <div class="col-md-${filter.width || 3}">
          <label class="form-label small">${filter.label}</label>
      `;

      if (filter.type === 'select') {
        html += `<select class="form-select form-select-sm" id="${filter.id}">`;
        html += '<option value="">全部</option>';
        filter.options.forEach(opt => {
          html += `<option value="${opt.value}">${opt.label}</option>`;
        });
        html += '</select>';
      } else if (filter.type === 'search') {
        html += `<input type="text" class="form-control form-control-sm" id="${filter.id}" placeholder="${filter.placeholder || '搜索...'}">`;
      } else if (filter.type === 'date') {
        html += `<input type="date" class="form-control form-control-sm" id="${filter.id}">`;
      }

      html += '</div>';
    });

    html += `
        <div class="col-md-auto d-flex align-items-end">
          <button class="btn btn-primary btn-sm me-2" id="apply-filter">筛选</button>
          <button class="btn btn-outline-secondary btn-sm" id="reset-filter">重置</button>
        </div>
      </div></div>
    `;

    container.innerHTML = html;
  },

  // 创建排序UI
  createSortUI(containerId, sortOptions) {
    const container = document.getElementById(containerId);
    if (!container) return;

    let html = `
      <div class="sort-bar mb-3">
        <div class="d-flex align-items-center gap-2">
          <label class="form-label small mb-0">排序：</label>
          <select class="form-select form-select-sm" id="sort-field" style="width: auto;">
    `;

    sortOptions.forEach(opt => {
      html += `<option value="${opt.value}">${opt.label}</option>`;
    });

    html += `
          </select>
          <select class="form-select form-select-sm" id="sort-order" style="width: auto;">
            <option value="asc">升序</option>
            <option value="desc">降序</option>
          </select>
        </div>
      </div>
    `;

    container.innerHTML = html;
  },

  // 应用筛选
  applyFilter(data, filters) {
    return data.filter(item => {
      for (const [key, value] of Object.entries(filters)) {
        if (!value) continue;

        if (typeof value === 'string') {
          const itemValue = String(item[key] || '').toLowerCase();
          const filterValue = value.toLowerCase();
          if (!itemValue.includes(filterValue)) return false;
        } else {
          if (item[key] !== value) return false;
        }
      }
      return true;
    });
  },

  // 应用排序
  applySort(data, field, order = 'asc') {
    return [...data].sort((a, b) => {
      const aVal = a[field];
      const bVal = b[field];

      if (aVal === bVal) return 0;

      const comparison = aVal > bVal ? 1 : -1;
      return order === 'asc' ? comparison : -comparison;
    });
  }
};

// ========== 分页组件 ==========
const Pagination = {
  // 创建分页UI
  create(containerId, currentPage, totalPages, onPageChange) {
    const container = document.getElementById(containerId);
    if (!container) return;

    let html = '<nav><ul class="pagination pagination-sm justify-content-center mb-0">';

    // 上一页
    html += `
      <li class="page-item ${currentPage === 1 ? 'disabled' : ''}">
        <a class="page-link" href="#" data-page="${currentPage - 1}">上一页</a>
      </li>
    `;

    // 页码
    const maxVisible = 5;
    let startPage = Math.max(1, currentPage - Math.floor(maxVisible / 2));
    let endPage = Math.min(totalPages, startPage + maxVisible - 1);

    if (endPage - startPage < maxVisible - 1) {
      startPage = Math.max(1, endPage - maxVisible + 1);
    }

    if (startPage > 1) {
      html += `<li class="page-item"><a class="page-link" href="#" data-page="1">1</a></li>`;
      if (startPage > 2) {
        html += `<li class="page-item disabled"><span class="page-link">...</span></li>`;
      }
    }

    for (let i = startPage; i <= endPage; i++) {
      html += `
        <li class="page-item ${i === currentPage ? 'active' : ''}">
          <a class="page-link" href="#" data-page="${i}">${i}</a>
        </li>
      `;
    }

    if (endPage < totalPages) {
      if (endPage < totalPages - 1) {
        html += `<li class="page-item disabled"><span class="page-link">...</span></li>`;
      }
      html += `<li class="page-item"><a class="page-link" href="#" data-page="${totalPages}">${totalPages}</a></li>`;
    }

    // 下一页
    html += `
      <li class="page-item ${currentPage === totalPages ? 'disabled' : ''}">
        <a class="page-link" href="#" data-page="${currentPage + 1}">下一页</a>
      </li>
    `;

    html += '</ul></nav>';
    container.innerHTML = html;

    // 绑定事件
    container.querySelectorAll('.page-link').forEach(link => {
      link.addEventListener('click', (e) => {
        e.preventDefault();
        const page = parseInt(link.dataset.page);
        if (page && page !== currentPage && page >= 1 && page <= totalPages) {
          onPageChange(page);
        }
      });
    });
  }
};

// ========== 空状态组件 ==========
const EmptyState = {
  show(containerId, message = '暂无数据', icon = '📭') {
    const container = document.getElementById(containerId);
    if (!container) return;

    container.innerHTML = `
      <div class="empty-state text-center py-5">
        <div class="empty-icon mb-3" style="font-size: 4rem;">${icon}</div>
        <p class="text-muted">${message}</p>
      </div>
    `;
  }
};

// 向后兼容：保持原有的全局函数
window.SkeletonLoader = SkeletonLoader;
window.LoadingState = LoadingState;
window.FeedbackAnimation = FeedbackAnimation;
window.DataFilter = DataFilter;
window.Pagination = Pagination;
window.EmptyState = EmptyState;
