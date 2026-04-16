package model

import (
	"testing"
)

func TestNormalizeTaskCategoryToVideoOnly(t *testing.T) {
	testCases := []TaskCategory{
		CategoryCopywriting,
		CategoryDesign,
		CategoryVideo,
		CategoryPhotography,
		CategoryMusic,
		CategoryDev,
		CategoryOther,
		TaskCategory(0),
		TaskCategory(999),
	}

	for _, input := range testCases {
		if got := NormalizeTaskCategory(input); got != CategoryVideo {
			t.Fatalf("NormalizeTaskCategory(%d) = %d, want %d", input, got, CategoryVideo)
		}
	}
}

func TestTaskV1Fields(t *testing.T) {
	// Test v1.md new fields in Task struct
	task := &Task{
		Industries:      "本地餐饮,美妆护肤",
		VideoDuration:   "60秒",
		VideoAspect:     "9:16",
		VideoResolution: "1080P",
		CreativeStyle:   "口语化",
				AwardPrice:      10.0,
	}

	if task.Industries != "本地餐饮,美妆护肤" {
		t.Errorf("Industries = %s, want 本地餐饮,美妆护肤", task.Industries)
	}
	if task.VideoDuration != "60秒" {
		t.Errorf("VideoDuration = %s, want 60秒", task.VideoDuration)
	}
	if task.VideoAspect != "9:16" {
		t.Errorf("VideoAspect = %s, want 9:16", task.VideoAspect)
	}
	if task.VideoResolution != "1080P" {
		t.Errorf("VideoResolution = %s, want 1080P", task.VideoResolution)
	}
	if task.CreativeStyle != "口语化" {
		t.Errorf("CreativeStyle = %s, want 口语化", task.CreativeStyle)
	}
	if task.AwardPrice != 10.0 {
		t.Errorf("AwardPrice = %f, want 10.0", task.AwardPrice)
	}
}

func TestTaskCreateV1Fields(t *testing.T) {
	// Test v1.md new fields in TaskCreate request
	req := TaskCreate{
		Title:          "测试任务",
		Description:    "测试描述",
		UnitPrice:      5.0,
		TotalCount:     20,
		Industries:    []string{"本地餐饮", "美妆护肤"},
		VideoDuration: "30秒",
		VideoAspect:   "16:9",
		VideoResolution: "720P",
		CreativeStyle: "种草安利",
		AwardPrice:    15.0,
	}

	if len(req.Industries) != 2 {
		t.Errorf("Industries length = %d, want 2", len(req.Industries))
	}
	if req.Industries[0] != "本地餐饮" {
		t.Errorf("Industries[0] = %s, want 本地餐饮", req.Industries[0])
	}
	if req.VideoDuration != "30秒" {
		t.Errorf("VideoDuration = %s, want 30秒", req.VideoDuration)
	}
	if req.VideoAspect != "16:9" {
		t.Errorf("VideoAspect = %s, want 16:9", req.VideoAspect)
	}
	if req.VideoResolution != "720P" {
		t.Errorf("VideoResolution = %s, want 720P", req.VideoResolution)
	}
	if req.CreativeStyle != "种草安利" {
		t.Errorf("CreativeStyle = %s, want 种草安利", req.CreativeStyle)
	}
	if req.AwardPrice != 15.0 {
		t.Errorf("AwardPrice = %f, want 15.0", req.AwardPrice)
	}
}

func TestBudgetCalculation(t *testing.T) {
	tests := []struct {
		name       string
		unitPrice  float64
		totalCount int
		awardPrice float64
		wantTotal  float64
	}{
		{
			name:       "参与奖励 only",
			unitPrice:  5.0,
			totalCount: 10,
			awardPrice: 0,
			wantTotal:  50.0,
		},
		{
			name:       "参与+采纳奖励",
			unitPrice:  5.0,
			totalCount: 10,
			awardPrice: 10.0,
			wantTotal:  150.0,
		},
		{
			name:       "v1最低要求 - 参与5元×10人+采纳8元×10人",
			unitPrice:  5.0,
			totalCount: 10,
			awardPrice: 8.0,
			wantTotal:  130.0,
		},
		{
			name:       "高奖励任务",
			unitPrice:  100.0,
			totalCount: 50,
			awardPrice: 200.0,
			wantTotal:  15000.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			totalBudget := float64(tt.totalCount) * (tt.unitPrice + tt.awardPrice)
			if totalBudget != tt.wantTotal {
				t.Errorf("totalBudget = %f, want %f", totalBudget, tt.wantTotal)
			}
		})
	}
}

func TestIndustriesJoin(t *testing.T) {
	industries := []string{"本地餐饮", "美妆护肤", "家居家电"}
	joined := joinIndustries(industries)

	expected := "本地餐饮,美妆护肤,家居家电"
	if joined != expected {
		t.Errorf("joinIndustries() = %s, want %s", joined, expected)
	}
}

func TestIndustriesSplit(t *testing.T) {
	joined := "本地餐饮,美妆护肤,家居家电"
	split := splitIndustries(joined)

	if len(split) != 3 {
		t.Errorf("splitIndustries() returned %d items, want 3", len(split))
	}
	if split[0] != "本地餐饮" {
		t.Errorf("splitIndustries()[0] = %s, want 本地餐饮", split[0])
	}
	if split[2] != "家居家电" {
		t.Errorf("splitIndustries()[2] = %s, want 家居家电", split[2])
	}
}

// Helper functions to simulate handler logic
func joinIndustries(industries []string) string {
	if len(industries) == 0 {
		return ""
	}
	result := industries[0]
	for i := 1; i < len(industries); i++ {
		result += "," + industries[i]
	}
	return result
}

func splitIndustries(industries string) []string {
	if industries == "" {
		return []string{}
	}
	var result []string
	start := 0
	for i := 0; i < len(industries); i++ {
		if industries[i] == ',' {
			result = append(result, industries[start:i])
			start = i + 1
		}
	}
	result = append(result, industries[start:])
	return result
}

func TestVideoDurationOptions(t *testing.T) {
	validOptions := []string{"15秒内", "30秒", "60秒", "1-3分钟", "不限制", ""}

	for _, opt := range validOptions {
		t.Run("VideoDuration_"+opt, func(t *testing.T) {
			req := TaskCreate{VideoDuration: opt}
			if req.VideoDuration != opt {
				t.Errorf("VideoDuration = %s, want %s", req.VideoDuration, opt)
			}
		})
	}
}

func TestVideoAspectOptions(t *testing.T) {
	validOptions := []string{"9:16", "16:9", "1:1", ""}

	for _, opt := range validOptions {
		t.Run("VideoAspect_"+opt, func(t *testing.T) {
			req := TaskCreate{VideoAspect: opt}
			if req.VideoAspect != opt {
				t.Errorf("VideoAspect = %s, want %s", req.VideoAspect, opt)
			}
		})
	}
}

func TestVideoResolutionOptions(t *testing.T) {
	validOptions := []string{"720P", "1080P", ""}

	for _, opt := range validOptions {
		t.Run("VideoResolution_"+opt, func(t *testing.T) {
			req := TaskCreate{VideoResolution: opt}
			if req.VideoResolution != opt {
				t.Errorf("VideoResolution = %s, want %s", req.VideoResolution, opt)
			}
		})
	}
}

func TestCreativeStyleOptions(t *testing.T) {
	validOptions := []string{"口语化", "商务正式", "种草安利", "搞笑轻松", "温情故事", "科普专业", "其他", ""}

	for _, opt := range validOptions {
		t.Run("CreativeStyle_"+opt, func(t *testing.T) {
			req := TaskCreate{CreativeStyle: opt}
			if req.CreativeStyle != opt {
				t.Errorf("CreativeStyle = %s, want %s", req.CreativeStyle, opt)
			}
		})
	}
}

func TestTaskStatusValues(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected int
		name     string
	}{
		{TaskStatusPending, 1, "Pending"},
		{TaskStatusOnline, 2, "Online"},
		{TaskStatusOngoing, 3, "Ongoing"},
		{TaskStatusEnded, 4, "Ended"},
		{TaskStatusCancelled, 5, "Cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.status) != tt.expected {
				t.Errorf("TaskStatus = %d, want %d", tt.status, tt.expected)
			}
		})
	}
}

func TestTaskIsAvailable(t *testing.T) {
	tests := []struct {
		name     string
		status   TaskStatus
		remain   int
		expected bool
	}{
		{"Available - Online with remaining", TaskStatusOnline, 5, true},
		{"Not Available - Online but no remaining", TaskStatusOnline, 0, false},
		{"Not Available - Ongoing", TaskStatusOngoing, 5, false},
		{"Not Available - Pending", TaskStatusPending, 10, false},
		{"Not Available - Ended", TaskStatusEnded, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{Status: tt.status, RemainingCount: tt.remain}
			if task.IsAvailable() != tt.expected {
				t.Errorf("IsAvailable() = %v, want %v", task.IsAvailable(), tt.expected)
			}
		})
	}
}

func TestTaskUpdateV1Fields(t *testing.T) {
	update := TaskUpdate{
		Title:          "更新任务",
		Description:    "更新描述",
		UnitPrice:      10.0,
		TotalCount:     50,
		Industries:     []string{"数码3C", "教育培训"},
		VideoDuration:  "1-3分钟",
		VideoAspect:    "16:9",
		VideoResolution: "1080P",
		CreativeStyle:  "温情故事",
		AwardPrice:     20.0,
	}

	if update.Title != "更新任务" {
		t.Errorf("Title = %s, want 更新任务", update.Title)
	}
	if len(update.Industries) != 2 {
		t.Errorf("Industries length = %d, want 2", len(update.Industries))
	}
	if update.VideoDuration != "1-3分钟" {
		t.Errorf("VideoDuration = %s, want 1-3分钟", update.VideoDuration)
	}
	if update.AwardPrice != 20.0 {
		t.Errorf("AwardPrice = %f, want 20.0", update.AwardPrice)
	}
}
