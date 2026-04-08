// 角色切换功能

// 获取用户的所有角色
function getUserRoles() {
  const roles = localStorage.getItem('roles') || localStorage.getItem('role') || '';
  return roles.split(',').filter(r => r);
}

// 获取当前激活的角色
function getCurrentRole() {
  return localStorage.getItem('current_role') || getUserRoles()[0] || 'creator';
}

// 设置当前角色
function setCurrentRole(role) {
  localStorage.setItem('current_role', role);
}

// 检查用户是否有某个角色
function hasRole(role) {
  return getUserRoles().includes(role);
}

// 切换角色
function switchRole(newRole) {
  const roles = getUserRoles();

  if (!roles.includes(newRole)) {
    showError('您没有该角色权限');
    return false;
  }

  setCurrentRole(newRole);
  showSuccess('已切换到' + (newRole === 'creator' ? '创作者' : '商家') + '身份');

  // 跳转到对应的工作台
  setTimeout(() => {
    if (newRole === 'creator') {
      window.location.href = '/creator/dashboard.html';
    } else if (newRole === 'business') {
      window.location.href = '/business/dashboard.html';
    }
  }, 500);

  return true;
}

// 渲染角色切换器（在导航栏中显示）
function renderRoleSwitcher() {
  const roles = getUserRoles();
  const currentRole = getCurrentRole();

  // 如果只有一个角色，不显示切换器
  if (roles.length <= 1) {
    return '';
  }

  const roleNames = {
    'creator': '创作者',
    'business': '商家',
    'admin': '管理员'
  };

  let html = `
    <div class="dropdown">
      <button class="btn btn-outline-primary btn-sm dropdown-toggle" type="button" id="roleSwitcher" data-bs-toggle="dropdown" aria-expanded="false">
        <i class="bi bi-person-circle"></i> ${roleNames[currentRole] || '当前身份'}
      </button>
      <ul class="dropdown-menu" aria-labelledby="roleSwitcher">
  `;

  roles.forEach(role => {
    const isActive = role === currentRole;
    html += `
      <li>
        <a class="dropdown-item ${isActive ? 'active' : ''}" href="#" onclick="switchRole('${role}'); return false;">
          ${isActive ? '<i class="bi bi-check-circle-fill"></i> ' : ''}
          ${roleNames[role] || role}
        </a>
      </li>
    `;
  });

  html += `
      </ul>
    </div>
  `;

  return html;
}

// 初始化角色（在登录后调用）
function initializeRole(userData) {
  // 保存所有角色
  localStorage.setItem('roles', userData.roles || userData.role);

  // 如果没有设置当前角色，使用第一个角色
  if (!localStorage.getItem('current_role')) {
    const roles = getUserRoles();
    if (roles.length > 0) {
      setCurrentRole(roles[0]);
    }
  }
}

// 检查当前页面是否匹配当前角色
function checkRoleAccess() {
  const currentRole = getCurrentRole();
  const path = window.location.pathname;

  // 检查是否在错误的角色页面
  if (path.includes('/creator/') && currentRole !== 'creator') {
    if (hasRole('creator')) {
      // 有创作者角色但当前不是，提示切换
      if (confirm('当前是' + (currentRole === 'business' ? '商家' : '管理员') + '身份，是否切换到创作者身份？')) {
        switchRole('creator');
      } else {
        window.location.href = '/' + currentRole + '/dashboard.html';
      }
    } else {
      // 没有创作者角色，跳转到当前角色的工作台
      showError('您没有创作者权限');
      window.location.href = '/' + currentRole + '/dashboard.html';
    }
  } else if (path.includes('/business/') && currentRole !== 'business') {
    if (hasRole('business')) {
      if (confirm('当前是' + (currentRole === 'creator' ? '创作者' : '管理员') + '身份，是否切换到商家身份？')) {
        switchRole('business');
      } else {
        window.location.href = '/' + currentRole + '/dashboard.html';
      }
    } else {
      showError('您没有商家权限');
      window.location.href = '/' + currentRole + '/dashboard.html';
    }
  }
}

// 页面加载时检查角色访问权限
if (typeof window !== 'undefined') {
  window.addEventListener('DOMContentLoaded', function() {
    // 只在需要认证的页面检查
    const path = window.location.pathname;
    if (path.includes('/creator/') || path.includes('/business/') || path.includes('/admin/')) {
      const token = localStorage.getItem('token');
      if (token) {
        checkRoleAccess();
      }
    }
  });
}
