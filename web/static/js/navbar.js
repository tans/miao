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
    <nav class="navbar navbar-expand-lg navbar-light sticky-top">
      <div class="container-fluid px-3 px-lg-4">
        <a class="navbar-brand fw-bold" href="/">创意喵</a>
        <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navbarNav">
          <span class="navbar-toggler-icon"></span>
        </button>
        <div class="collapse navbar-collapse" id="navbarNav">
          <ul class="navbar-nav me-auto">
            ${items.map(item => `
              <li class="nav-item">
                <a class="nav-link ${currentPath === item.href ? 'active' : ''}" href="${item.href}">${item.text}</a>
              </li>
            `).join('')}
          </ul>
          <div class="d-flex align-items-center gap-3">
            <div id="role-switcher-container"></div>
            <div class="dropdown">
              <button class="btn btn-outline-secondary dropdown-toggle" type="button" data-bs-toggle="dropdown">
                ${username}
              </button>
              <ul class="dropdown-menu dropdown-menu-end">
                <li><a class="dropdown-item" href="/user/profile.html">个人资料</a></li>
                <li><a class="dropdown-item" href="/user/password.html">修改密码</a></li>
                <li><a class="dropdown-item" href="/messages.html">消息中心</a></li>
                <li><hr class="dropdown-divider"></li>
                <li><a class="dropdown-item" href="#" onclick="logout(); return false;">退出登录</a></li>
              </ul>
            </div>
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
