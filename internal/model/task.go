package model

import "time"

// TaskStatus 任务状态
type TaskStatus int

const (
	TaskStatusPending   TaskStatus = 1 // 待审核
	TaskStatusOnline    TaskStatus = 2 // 已上架
	TaskStatusOngoing   TaskStatus = 3 // 进行中
	TaskStatusEnded     TaskStatus = 4 // 已结束
	TaskStatusCancelled TaskStatus = 5 // 已取消
)

// TaskCategory 任务分类
type TaskCategory int

const (
	CategoryCopywriting  TaskCategory = 1 // 文案
	CategoryDesign      TaskCategory = 2 // 设计
	CategoryVideo       TaskCategory = 3 // 视频
	CategoryPhotography TaskCategory = 4 // 摄影
	CategoryMusic      TaskCategory = 5 // 音乐
	CategoryDev         TaskCategory = 6 // 开发
	CategoryOther       TaskCategory = 7 // 其他
)

// Task 任务表
type Task struct {
	ID              int64        `json:"id"`
	BusinessID      int64        `json:"business_id"`
	Title           string       `json:"title"`
	Description     string       `json:"description"`
	Category        TaskCategory `json:"category"` // 1=文案, 2=设计, 3=视频, 4=摄影, 5=音乐, 6=开发, 7=其他

	UnitPrice       float64      `json:"unit_price"`        // 单价
	TotalCount      int          `json:"total_count"`       // 总数量
	RemainingCount  int          `json:"remaining_count"`   // 剩余数量

	Status          TaskStatus   `json:"status"`             // 1=待审核, 2=已上架, 3=进行中, 4=已结束, 5=已取消
	ReviewAt        *time.Time   `json:"review_at,omitempty"` // 审核时间
	PublishAt       *time.Time   `json:"publish_at,omitempty"` // 上架时间
	EndAt           *time.Time   `json:"end_at,omitempty"`   // 结束时间

	// 资金
	TotalBudget     float64      `json:"total_budget"`      // = UnitPrice * TotalCount
	FrozenAmount    float64      `json:"frozen_amount"`     // 已冻结
	PaidAmount      float64      `json:"paid_amount"`      // 已支付

	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

// IsAvailable 检查任务是否可认领
func (t *Task) IsAvailable() bool {
	return t.Status == TaskStatusOnline && t.RemainingCount > 0
}

// TaskCreate 创建任务请求
type TaskCreate struct {
	Title       string        `json:"title" binding:"required"`
	Description string        `json:"description" binding:"required"`
	Category    TaskCategory  `json:"category" binding:"required"`
	UnitPrice   float64       `json:"unit_price" binding:"required,gt=0"`
	TotalCount  int           `json:"total_count" binding:"required,gt=0"`
	Deadline    string        `json:"deadline"` // 截止时间 (RFC3339格式)
}

// TaskUpdate 更新任务请求
type TaskUpdate struct {
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Category    TaskCategory  `json:"category"`
	UnitPrice   float64       `json:"unit_price"`
	TotalCount  int           `json:"total_count"`
}

// TaskQuery 任务查询
type TaskQuery struct {
	Category TaskCategory `form:"category"`
	Status   *int         `form:"status"`
	Keyword  string      `form:"keyword"`
	Page     int         `form:"page,default=1"`
	PageSize int         `form:"page_size,default=20"`
}

// TaskListQuery 商家任务列表查询
type TaskListQuery struct {
	Status   *int `form:"status"`
	Page     int  `form:"page,default=1"`
	PageSize int  `form:"page_size,default=20"`
}
