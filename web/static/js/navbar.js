// 导航栏组件
function renderNavbar(role) {
  const username = localStorage.getItem('username') || '用户';

  const navItems = {
    creator: [
      { href: '/creator/dashboard.html', text: '工作台' },
      { href: '/creator/task_hall.html', text: '任务大厅' },
      { href: '/creator/claim_list.html', text: '我的认领' },
      { href: '/creator/wallet.html', text: '钱包' },
    ],
    business: [
      { href: '/business/dashboard.html', text: '工作台' },
      { href: '/business/task_create.html', text: '发布任务' },
      { href: '/business/task_list.html', text: '我的任务' },
      { href: '/business/claim_review.html', text: '审核认领' },
      { href: '/business/recharge.html', text: '充值' },
    ],
    admin: [
      { href: '/admin/dashboard.html', text: '工作台' },
      { href: '/admin/user_list.html', text: '用户管理' },
      { href: '/admin/task_list.html', text: '任务管理' },
      { href: '/admin/appeal_list.html', text: '申诉管理' },
    ]
  };

  const roleLabels = {
    creator: '创作者版',
    business: '商家版',
    admin: '管理后台'
  };

  const items = navItems[role] || [];
  const currentPath = window.location.pathname;
  const roleLabel = roleLabels[role] || '平台';

  return `
    <nav class="navbar navbar-expand-lg sticky-top">
      <div class="container-fluid px-3 px-lg-4">
        <a class="navbar-brand d-flex flex-column align-items-start" href="/">
          <span style="font-size: 0.72rem; letter-spacing: 0.18em; color: var(--ink-faint);">CREATIVE MARKET</span>
          <span style="font-size: 1.15rem;">创意喵</span>
        </a>
        <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navbarNav">
          <span class="navbar-toggler-icon"></span>
        </button>
        <div class="collapse navbar-collapse" id="navbarNav">
          <div class="ms-lg-4 me-auto d-flex align-items-center flex-wrap gap-2 gap-lg-3 py-3 py-lg-0">
            <span class="page-eyebrow mb-0">${roleLabel}</span>
            ${items.map(item => `
              <a class="nav-link ${currentPath === item.href ? 'active' : ''}" href="${item.href}">${item.text}</a>
            `).join('')}
          </div>
          <div class="d-flex align-items-center flex-column flex-lg-row gap-2 gap-lg-3 py-3 py-lg-0">
            <div id="role-switcher-container"></div>
            <div class="text-lg-end">
              <div style="font-size: 0.72rem; letter-spacing: 0.12em; color: var(--ink-faint); text-transform: uppercase;">当前账号</div>
              <div style="font-weight: 700; color: var(--ink);">${username}</div>
            </div>
            <button class="btn btn-outline-secondary btn-sm" onclick="logout()">退出</button>
          </div>
        </div>
      </div>
    </nav>
  `;
}

// 页面加载时渲染导航栏
document.addEventListener('DOMContentLoaded', function() {
  const role = localStorage.getItem('current_role') || localStorage.getItem('role');
  const navbarContainer = document.getElementById('navbar-container');

  if (navbarContainer && role) {
    navbarContainer.innerHTML = renderNavbar(role);

    const roleSwitcherContainer = document.getElementById('role-switcher-container');
    if (roleSwitcherContainer && typeof renderRoleSwitcher === 'function') {
      roleSwitcherContainer.innerHTML = renderRoleSwitcher();
    }
  }
});
