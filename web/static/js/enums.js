// docs/enums.md 对应的 JavaScript 枚举
// 所有状态值与后端 Go 枚举完全一致（1-indexed）

const TaskStatus = {
  PENDING: 1,    // 待审核
  ONLINE: 2,     // 已上架
  ONGOING: 3,    // 进行中
  ENDED: 4,      // 已结束
  CANCELLED: 5,  // 已取消
};

const TaskStatusText = {
  [TaskStatus.PENDING]: '待审核',
  [TaskStatus.ONLINE]: '已上架',
  [TaskStatus.ONGOING]: '进行中',
  [TaskStatus.ENDED]: '已结束',
  [TaskStatus.CANCELLED]: '已取消',
};

const TaskStatusColor = {
  [TaskStatus.PENDING]: 'warning',
  [TaskStatus.ONLINE]: 'success',
  [TaskStatus.ONGOING]: 'primary',
  [TaskStatus.ENDED]: 'secondary',
  [TaskStatus.CANCELLED]: 'danger',
};

const ClaimStatus = {
  PENDING: 1,     // 待提交
  SUBMITTED: 2,   // 待验收
  APPROVED: 3,    // 已完成
  CANCELLED: 4,   // 已取消
  EXPIRED: 5,     // 已超时
};

const ClaimStatusText = {
  [ClaimStatus.PENDING]: '待提交',
  [ClaimStatus.SUBMITTED]: '待验收',
  [ClaimStatus.APPROVED]: '已完成',
  [ClaimStatus.CANCELLED]: '已取消',
  [ClaimStatus.EXPIRED]: '已超时',
};

const ClaimStatusColor = {
  [ClaimStatus.PENDING]: 'secondary',
  [ClaimStatus.SUBMITTED]: 'info',
  [ClaimStatus.APPROVED]: 'success',
  [ClaimStatus.CANCELLED]: 'danger',
  [ClaimStatus.EXPIRED]: 'warning',
};

/**
 * 获取任务状态文本
 */
function getTaskStatusText(status) {
  return TaskStatusText[status] || `未知(${status})`;
}

/**
 * 获取任务状态对应的 Bootstrap 颜色类
 */
function getTaskStatusColor(status) {
  return TaskStatusColor[status] || 'secondary';
}

/**
 * 获取认领状态文本
 * @param status 认领状态 (1=待提交, 2=待验收, 3=已完成, 4=已取消, 5=已超时)
 * @param reviewResult 验收结果 (2=退回/拒绝, 3=举报)
 */
function getClaimStatusText(status, reviewResult) {
  if (reviewResult === 3) return '已被举报';
  if (reviewResult === 2) return '已退回';
  return ClaimStatusText[status] || `未知(${status})`;
}

/**
 * 获取认领状态对应的 Bootstrap 颜色类
 */
function getClaimStatusColor(status, reviewResult) {
  if (reviewResult === 3) return 'danger'; // 举报 - 红色
  if (reviewResult === 2) return 'secondary'; // 退回 - 灰色
  return ClaimStatusColor[status] || 'secondary';
}
