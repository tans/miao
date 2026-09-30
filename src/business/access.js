export const appPermission = async (pocketbase, { app, tenant, user, membership }) => {
  if (membership.role === 'owner' && tenant.owner_id === user.id) return { role: 'owner', can_batch: true };
  const assigned = await pocketbase.collection('app_members').getFirstListItem(pocketbase.filter(
    'tenant_id = {:tenantId} && app_id = {:appId} && user_id = {:userId}',
    { tenantId: tenant.id, appId: app.id, userId: user.id }
  )).catch((error) => { if (error.status === 404) return null; throw error; });
  if (app.restricted && !assigned) return null;
  return { role: assigned?.role || 'editor', can_batch: Boolean(assigned?.can_batch) && assigned.role !== 'viewer' };
};

export const taskAuthority = async (pocketbase, task) => {
  const [user, tenant, app, membership] = await Promise.all([
    pocketbase.collection('users').getOne(task.created_by),
    pocketbase.collection('tenants').getOne(task.tenant_id),
    pocketbase.collection('apps').getOne(task.app_id),
    pocketbase.collection('tenant_members').getFirstListItem(pocketbase.filter('tenant_id = {:tenantId} && user_id = {:userId}', { tenantId: task.tenant_id, userId: task.created_by }))
  ]).catch((error) => { if (error.status === 404) throw Object.assign(new Error('任务负责人、工作区或应用已失效'), { code: 'AUTH_REVOKED' }); throw error; });
  const permission = await appPermission(pocketbase, { app, tenant, user, membership });
  if (user.disabled || (process.env.MIAO_REQUIRE_EMAIL_VERIFICATION === 'true' && !user.verified) || app.archived || app.tenant_id !== tenant.id || !['owner', 'publisher'].includes(permission?.role)) {
    throw Object.assign(new Error('任务负责人已失去权限或应用已归档'), { code: 'AUTH_REVOKED' });
  }
  return { user, tenant, app, membership, permission };
};
