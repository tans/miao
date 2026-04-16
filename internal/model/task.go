package model

import "time"

// TaskStatus 任务状态
// 枚举定义参考: docs/enums.md
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
	CategoryCopywriting TaskCategory = 1 // 兼容保留
	CategoryDesign      TaskCategory = 2 // 兼容保留
	CategoryVideo       TaskCategory = 3 // 当前平台唯一支持的视频任务分类
	CategoryPhotography TaskCategory = 4 // 兼容保留
	CategoryMusic       TaskCategory = 5 // 兼容保留
	CategoryDev         TaskCategory = 6 // 兼容保留
	CategoryOther       TaskCategory = 7 // 兼容保留
)

// NormalizeTaskCategory forces all new tasks to the only supported category.
func NormalizeTaskCategory(_ TaskCategory) TaskCategory {
	return CategoryVideo
}

// Task 任务表
type Task struct {
	ID          int64        `json:"id" db:"id"`
	BusinessID  int64        `json:"business_id" db:"business_id"`
	Title       string       `json:"title" db:"title"`
	Description string       `json:"description" db:"description"`
	Category    TaskCategory `json:"category" db:"category"` // 兼容保留字段，当前平台固定为 3=视频

	UnitPrice      float64 `json:"unit_price" db:"unit_price"`           // 参与奖励（合格投稿均可获得）
	TotalCount     int     `json:"total_count" db:"total_count"`         // 报名人数上限
	RemainingCount int     `json:"remaining_count" db:"remaining_count"` // 剩余数量

	// v1.md 规范新增字段
	Industries      string  `json:"industries" db:"industries"`                     // 行业选项（多选，逗号分隔）
	VideoDuration   string  `json:"video_duration" db:"video_duration"`             // 视频时长：15秒内/30秒/60秒/1-3分钟/不限制
	VideoAspect     string  `json:"video_aspect" db:"video_aspect"`                 // 视频尺寸：9:16/16:9/1:1
	VideoResolution string  `json:"video_resolution" db:"video_resolution"`         // 分辨率：720P/1080P
	CreativeStyle   string  `json:"creative_style" db:"creative_style"`             // 创作风格：口语化/商务正式/种草安利/搞笑轻松/温情故事/科普专业/其他
	AwardPrice      float64 `json:"award_price" db:"award_price"`                   // 入围奖励（入围即中标）
	AwardCount      int     `json:"award_count" db:"award_count"`                   // 入围数量

	Status    TaskStatus `json:"status" db:"status"`                   // 1=待审核, 2=已上架, 3=进行中, 4=已结束, 5=已取消
	ReviewAt  *time.Time `json:"review_at,omitempty" db:"review_at"`   // 审核时间
	PublishAt *time.Time `json:"publish_at,omitempty" db:"publish_at"` // 上架时间
	EndAt     *time.Time `json:"end_at,omitempty" db:"end_at"`         // 结束时间

	// 资金
	TotalBudget  float64 `json:"total_budget" db:"total_budget"`   // = UnitPrice * TotalCount + AwardPrice * AwardCount
	FrozenAmount float64 `json:"frozen_amount" db:"frozen_amount"` // 已冻结
	PaidAmount   float64 `json:"paid_amount" db:"paid_amount"`     // 已支付

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// 任务素材（查询时填充，不存储在 tasks 表）
	Materials []TaskMaterial `json:"materials,omitempty" db:"-"`
}

// IsAvailable 检查任务是否可认领
func (t *Task) IsAvailable() bool {
	return t.Status == TaskStatusOnline && t.RemainingCount > 0
}

// TaskCreate 创建任务请求
type TaskCreate struct {
	Title       string       `json:"title" binding:"required"`
	Description string       `json:"description" binding:"required"`
	Category    TaskCategory `json:"category"` // 兼容保留，缺省时也会被归一为视频
	UnitPrice   float64      `json:"unit_price" binding:"required,gt=0"` // 参与奖励（≥2元)
	TotalCount  int          `json:"total_count" binding:"required,gt=0"` // 报名人数上限（≥10）
	Deadline    string       `json:"deadline" binding:"required"`             // 截止时间 (RFC3339格式)

	// v1.md 规范新增字段
	Industries      []string `json:"industries"`       // 行业选项（多选）
	VideoDuration   string   `json:"video_duration"`   // 视频时长
	VideoAspect     string   `json:"video_aspect"`     // 视频尺寸
	VideoResolution string   `json:"video_resolution"` // 分辨率
	CreativeStyle   string   `json:"creative_style"`   // 创作风格
	AwardPrice      float64  `json:"award_price"`      // 入围奖励（≥8元）
	AwardCount      int      `json:"award_count"`      // 入围数量（≥1）

	// 任务素材（必填，第一个必须是图片）
	Materials []TaskMaterialInput `json:"materials"`
}

// TaskMaterial 任务素材文件
type TaskMaterial struct {
	ID        int64     `json:"id" db:"id"`
	TaskID    int64     `json:"task_id" db:"task_id"`
	FileName  string    `json:"file_name" db:"file_name"`
	FilePath  string    `json:"file_path" db:"file_path"`
	FileSize  int64     `json:"file_size" db:"file_size"`
	FileType  string    `json:"file_type" db:"file_type"` // "image" or "video"
	SortOrder int       `json:"sort_order" db:"sort_order"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// TaskMaterialInput 创建任务时提交的素材
type TaskMaterialInput struct {
	FileName  string `json:"file_name" binding:"required"`
	FilePath  string `json:"file_path" binding:"required"`
	FileSize  int64  `json:"file_size"`
	FileType  string `json:"file_type" binding:"required"` // "image" or "video"
	SortOrder int    `json:"sort_order"`
}

// TaskUpdate 更新任务请求
type TaskUpdate struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Category    TaskCategory `json:"category"`
	UnitPrice   float64      `json:"unit_price"`
	TotalCount  int          `json:"total_count"`

	// v1.md 规范新增字段
	Industries      []string `json:"industries"`
	VideoDuration   string   `json:"video_duration"`
	VideoAspect     string   `json:"video_aspect"`
	VideoResolution string   `json:"video_resolution"`
	CreativeStyle   string   `json:"creative_style"`
	AwardPrice      float64  `json:"award_price"`
}

// TaskQuery 任务查询
type TaskQuery struct {
	Category TaskCategory `form:"category"`
	Status   *int         `form:"status"`
	Keyword  string       `form:"keyword"`
	Page     int          `form:"page,default=1"`
	PageSize int          `form:"page_size,default=20"`
}

// TaskListQuery 商家任务列表查询
type TaskListQuery struct {
	Status   *int `form:"status"`
	Page     int  `form:"page,default=1"`
	PageSize int  `form:"page_size,default=20"`
}
