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

  const items = navItems[role] || [];
  const currentPath = window.location.pathname;

  return `
    <nav class="navbar navbar-expand-lg navbar-dark bg-primary">
      <div class="container-fluid">
        <a class="navbar-brand" href="/">创意喵</a>
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
          <div class="d-flex align-items-center gap-3 text-white">
            <div id="role-switcher-container"></div>
            <span>欢迎，${username}</span>
            <button class="btn btn-outline-light btn-sm" onclick="logout()">退出</button>
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

    // 渲染角色切换器（如果有多个角色）
    const roleSwitcherContainer = document.getElementById('role-switcher-container');
    if (roleSwitcherContainer && typeof renderRoleSwitcher === 'function') {
      roleSwitcherContainer.innerHTML = renderRoleSwitcher();
    }
  }
});
