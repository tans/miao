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

// 切换角色
function switchRole(newRole) {
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

// 页面加载时初始化角色
if (typeof window !== 'undefined') {
  window.addEventListener('DOMContentLoaded', function() {
    const token = localStorage.getItem('token');
    if (token && !localStorage.getItem('current_role')) {
      // 如果没有设置当前角色，初始化为第一个角色
      const roles = getUserRoles();
      if (roles.length > 0) {
        setCurrentRole(roles[0]);
      }
    }
  });
}
