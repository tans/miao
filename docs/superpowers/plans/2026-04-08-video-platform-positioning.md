# Video Platform Positioning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align 创意喵 to a single video-task platform by removing multi-category UI and forcing backend task creation to save as video.

**Architecture:** Keep the existing `category` storage field for compatibility, but normalize all new writes to the video category and stop exposing category choice or category labels in user-facing templates. Update docs and regression checks so product language, UI, backend behavior, and tests all describe the same single-platform model.

**Tech Stack:** Go 1.21, Gin, Go templates, shell-based regression checks

---

### File Map

**Modify**
- `README.md`
- `docs/V1_0_PRD.md`
- `internal/model/task.go`
- `internal/handler/business.go`
- `web/templates/business/task_create.html`
- `web/templates/creator/task_hall.html`
- `web/templates/tasks.html`
- `web/templates/creator/task_detail.html`
- `web/templates/business/task_list.html`
- `test/test_task_filters.sh`

**Create**
- `internal/model/task_test.go`
- `.omx/plans/prd-video-platform-positioning.md`
- `.omx/plans/test-spec-video-platform-positioning.md`

### Task 1: Lock Video-Only Backend Semantics

**Files:**
- Modify: `internal/model/task.go`
- Modify: `internal/handler/business.go`
- Test: `internal/model/task_test.go`

- [ ] **Step 1: Write the failing unit test**

```go
package model

import "testing"

func TestNormalizeTaskCategoryToVideoOnly(t *testing.T) {
	cases := []TaskCategory{
		CategoryCopywriting,
		CategoryDesign,
		CategoryVideo,
		CategoryPhotography,
		CategoryMusic,
		CategoryDev,
		CategoryOther,
		0,
		999,
	}

	for _, input := range cases {
		if got := NormalizeTaskCategory(input); got != CategoryVideo {
			t.Fatalf("NormalizeTaskCategory(%d) = %d, want %d", input, got, CategoryVideo)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model -run TestNormalizeTaskCategoryToVideoOnly -v`
Expected: FAIL with `undefined: NormalizeTaskCategory`

- [ ] **Step 3: Add the minimal normalization helper**

```go
const (
	CategoryCopywriting  TaskCategory = 1 // legacy compatibility only
	CategoryDesign       TaskCategory = 2 // legacy compatibility only
	CategoryVideo        TaskCategory = 3 // only supported category
	CategoryPhotography  TaskCategory = 4 // legacy compatibility only
	CategoryMusic        TaskCategory = 5 // legacy compatibility only
	CategoryDev          TaskCategory = 6 // legacy compatibility only
	CategoryOther        TaskCategory = 7 // legacy compatibility only
)

func NormalizeTaskCategory(_ TaskCategory) TaskCategory {
	return CategoryVideo
}
```

- [ ] **Step 4: Use the helper in task creation**

```go
task := &model.Task{
	BusinessID:      userID,
	Title:           req.Title,
	Description:     req.Description,
	Category:        model.NormalizeTaskCategory(req.Category),
	UnitPrice:       req.UnitPrice,
	TotalCount:      req.TotalCount,
	RemainingCount:  req.TotalCount,
	Status:          model.TaskStatusPending,
	TotalBudget:     totalBudget,
	FrozenAmount:    0,
	PaidAmount:      0,
	CreatedAt:       time.Now(),
	UpdatedAt:       time.Now(),
}
```

- [ ] **Step 5: Re-run the focused test**

Run: `go test ./internal/model -run TestNormalizeTaskCategoryToVideoOnly -v`
Expected: PASS

### Task 2: Rewrite Product Docs To Video-Only Language

**Files:**
- Modify: `README.md`
- Modify: `docs/V1_0_PRD.md`

- [ ] **Step 1: Replace multi-category positioning in README**

```md
- ✅ 发布视频创意任务
+ ✅ 发布视频任务
```

```md
- 创意喵是一个专注于**视频创意**的任务平台
+ 创意喵是一个专注于**视频内容生产与交付**的任务平台
```

- [ ] **Step 2: Replace multi-category task model language in PRD**

```go
- Category       int       // 1=文案, 2=设计, 3=视频, 4=摄影, 5=音乐, 6=开发, 7=其他
+ Category       int       // 兼容保留字段，平台当前固定为 3=视频
```

- [ ] **Step 3: Run a conflict scan on docs**

Run: `rg -n "文案创作|设计创意|摄影拍照|音乐音频|程序开发|多分类创意" README.md docs/V1_0_PRD.md`
Expected: no matches that still describe the platform as a multi-category market

### Task 3: Remove Category UX From Merchant And Creator Templates

**Files:**
- Modify: `web/templates/business/task_create.html`
- Modify: `web/templates/creator/task_hall.html`
- Modify: `web/templates/tasks.html`
- Modify: `web/templates/creator/task_detail.html`
- Modify: `web/templates/business/task_list.html`

- [ ] **Step 1: Remove category selection from merchant task creation**

```html
- <label for="category" class="form-label">任务分类 <span class="text-danger">*</span></label>
- <select class="form-select" id="category" required>...</select>
+ <div class="form-text">当前平台仅支持视频任务，请在任务描述中写清脚本、镜头、剪辑和交付要求。</div>
```

- [ ] **Step 2: Force the submit payload to send the video category**

```js
const formData = {
	title: document.getElementById('title').value.trim(),
	description: document.getElementById('description').value.trim(),
	category: 3,
	unit_price: parseFloat(document.getElementById('unit_price').value),
	total_count: parseInt(document.getElementById('quantity').value),
	deadline: new Date(document.getElementById('deadline').value).toISOString(),
};
```

- [ ] **Step 3: Remove category filters and category labels from task halls**

```html
- <label class="form-label">任务分类</label>
- <select class="form-select" id="category-filter" onchange="loadTasks()">...</select>
+ <!-- category filter removed for single video platform -->
```

```js
- if (category) params.append('category', category);
- filters.push({ label: `分类: ${categoryMap[category]}` ... })
+ // category is no longer exposed in the UI
```

- [ ] **Step 4: Remove category badge rendering from detail/list pages**

```html
- <span class="badge bg-light text-dark ms-2">${categoryMap[task.category] || '其他'}</span>
+ <span class="badge bg-light text-dark ms-2">视频任务</span>
```

- [ ] **Step 5: Run a static template scan**

Run: `rg -n "任务分类|文案创作|设计创意|摄影拍照|音乐音频|程序开发" web/templates/business/task_create.html web/templates/creator/task_hall.html web/templates/tasks.html web/templates/creator/task_detail.html web/templates/business/task_list.html`
Expected: no matches in user-facing copy

### Task 4: Rewrite Regression Script For Video-Only Behavior

**Files:**
- Modify: `test/test_task_filters.sh`

- [ ] **Step 1: Replace category-specific test cases with video-only checks**

```bash
- test_api "筛选文案创作任务" "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&category=1" "200"
- test_api "筛选设计创意任务" "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&category=2" "200"
- test_api "筛选视频制作任务" "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&category=3" "200"
+ test_api "获取视频任务列表" "$BASE_URL/api/v1/creator/tasks?page=1&limit=20" "200"
+ test_api "按关键词搜索视频任务" "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&keyword=视频" "200"
+ test_api "按价格排序视频任务" "$BASE_URL/api/v1/creator/tasks?page=1&limit=20&sort=price_desc" "200"
```

- [ ] **Step 2: Validate script syntax**

Run: `bash -n test/test_task_filters.sh`
Expected: exit code 0

### Task 5: Run Full Verification

**Files:**
- Verify only

- [ ] **Step 1: Run focused model test**

Run: `go test ./internal/model -run TestNormalizeTaskCategoryToVideoOnly -v`
Expected: PASS

- [ ] **Step 2: Run full Go test suite**

Run: `go test ./...`
Expected: PASS, or capture pre-existing unrelated failures explicitly

- [ ] **Step 3: Re-run template and docs scans**

Run: `rg -n "任务分类|文案创作|设计创意|摄影拍照|音乐音频|程序开发" web/templates/business/task_create.html web/templates/creator/task_hall.html web/templates/tasks.html web/templates/creator/task_detail.html web/templates/business/task_list.html README.md docs/V1_0_PRD.md`
Expected: no conflicting multi-category UI copy remains

- [ ] **Step 4: Commit with lore trailers**

```bash
git add README.md docs/V1_0_PRD.md internal/model/task.go internal/model/task_test.go internal/handler/business.go web/templates/business/task_create.html web/templates/creator/task_hall.html web/templates/tasks.html web/templates/creator/task_detail.html web/templates/business/task_list.html test/test_task_filters.sh .omx/plans/prd-video-platform-positioning.md .omx/plans/test-spec-video-platform-positioning.md docs/superpowers/specs/2026-04-08-video-platform-positioning-design.md docs/superpowers/plans/2026-04-08-video-platform-positioning.md
git commit -m "Align platform semantics to video-only tasks

Constraint: Existing schema still stores category as an integer
Rejected: Full category-field migration | unnecessary risk for this pass
Confidence: high
Scope-risk: moderate
Directive: Do not reintroduce public multi-category UI without revisiting product positioning
Tested: go test ./..., bash -n test/test_task_filters.sh, static copy scans
Not-tested: Historical non-video task data migration"
```
