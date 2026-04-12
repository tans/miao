# 数据库清理实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除三张废弃表（submissions/submission_materials、accounts、messages），清理对应 Go 代码，并新增 claim_materials 表支持作品媒体文件，重建 /works 灵感画廊 API。

**Architecture:** 分四个任务依次执行：先加 claim_materials 表和媒体提交支持，再重建 /works 读 claims+claim_materials，最后清理废弃表和孤立代码。执行顺序保证在任何时刻都有可工作的系统。

**Tech Stack:** Go/Gin，SQLite（database/sql 直接操作，无 ORM），WeChat 小程序

---

## 文件修改映射

| 文件 | 操作 | 说明 |
|------|------|------|
| `miao/internal/database/migration.go` | 修改 | 新增 migration v9（claim_materials）；修改 schemaSQL 移除 submissions/submission_materials/messages；修改 v7 migration 移除 submissions |
| `miao/internal/model/claim.go` | 修改 | 新增 `ClaimMaterial` struct 和 `ClaimSubmitWithMedia` |
| `miao/internal/repository/creator.go` | 修改 | 新增 `CreateClaimMaterial`、`GetClaimMaterials` 方法 |
| `miao/internal/handler/creator.go` | 修改 | 更新 `SubmitClaim` 支持媒体文件列表 |
| `miao/internal/handler/work.go` | 修改 | `ListApprovedWorks`/`GetWork` 补充返回 `materials` 字段 |
| `miao/internal/model/account.go` | 删除 | 移除 Account struct（RechargeRequest/PrepayRequest 迁移到其他 model） |
| `miao/internal/repository/account.go` | 修改 | 删除 accounts 表相关方法，只保留 transaction 方法；文件重命名逻辑不变 |
| `miao-mini/utils/api.js` | 修改 | 更新 `submitClaim` 支持传媒体列表；新增 `getWorks()`（如不存在） |

---

## Task 1: 新增 claim_materials 表（DB migration）

**Files:**
- Modify: `miao/internal/database/migration.go`
- Modify: `miao/internal/model/claim.go`

- [ ] **Step 1: 在 migration.go 新增 v9 migration**

在 `miao/internal/database/migration.go` 的 `migrations` 数组末尾（当前最后一项是 Version:8），在 `}` 闭合前追加：

```go
{
    Version: 9,
    Name:    "claim_materials",
    SQL: `
CREATE TABLE IF NOT EXISTS claim_materials (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    claim_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER DEFAULT 0,
    file_type TEXT NOT NULL,
    thumbnail_path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (claim_id) REFERENCES claims(id)
);

CREATE INDEX IF NOT EXISTS idx_claim_materials_claim_id ON claim_materials(claim_id);
`,
},
```

- [ ] **Step 2: 在 model/claim.go 新增 ClaimMaterial struct**

在 `miao/internal/model/claim.go` 末尾追加：

```go
// ClaimMaterial 认领媒体文件
type ClaimMaterial struct {
	ID            int64     `json:"id" db:"id"`
	ClaimID       int64     `json:"claim_id" db:"claim_id"`
	FileName      string    `json:"file_name" db:"file_name"`
	FilePath      string    `json:"file_path" db:"file_path"`
	FileSize      int64     `json:"file_size" db:"file_size"`
	FileType      string    `json:"file_type" db:"file_type"`
	ThumbnailPath string    `json:"thumbnail_path,omitempty" db:"thumbnail_path"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// ClaimMaterialInput 提交时的媒体输入
type ClaimMaterialInput struct {
	FileName      string `json:"file_name" binding:"required"`
	FilePath      string `json:"file_path" binding:"required"`
	FileSize      int64  `json:"file_size"`
	FileType      string `json:"file_type" binding:"required"`
	ThumbnailPath string `json:"thumbnail_path"`
}
```

同时修改已有的 `ClaimSubmit` struct，添加 Materials 字段：

找到：
```go
type ClaimSubmit struct {
	Content string `json:"content" binding:"required"`
}
```

改为：
```go
type ClaimSubmit struct {
	Content   string               `json:"content" binding:"required"`
	Materials []ClaimMaterialInput `json:"materials"`
}
```

- [ ] **Step 3: 编译验证**

```bash
cd /Users/ke/code/miao-repo/miao && go build ./...
```

期望：无编译错误

- [ ] **Step 4: 提交**

```bash
cd /Users/ke/code/miao-repo/miao
git add internal/database/migration.go internal/model/claim.go
git commit -m "feat: 新增 claim_materials 表和模型（migration v9）"
```

---

## Task 2: 实现 ClaimMaterial repository 方法

**Files:**
- Modify: `miao/internal/repository/creator.go`

- [ ] **Step 1: 在 creator.go 末尾添加 CreateClaimMaterial 方法**

在 `miao/internal/repository/creator.go` 末尾追加：

```go
// CreateClaimMaterial 保存认领媒体文件记录
func (r *CreatorRepository) CreateClaimMaterial(material *model.ClaimMaterial) error {
	query := `
		INSERT INTO claim_materials (claim_id, file_name, file_path, file_size, file_type, thumbnail_path, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	result, err := r.db.Exec(query,
		material.ClaimID,
		material.FileName,
		material.FilePath,
		material.FileSize,
		material.FileType,
		material.ThumbnailPath,
		now,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	material.ID = id
	material.CreatedAt = now
	return nil
}

// GetClaimMaterials 获取某认领的所有媒体文件
func (r *CreatorRepository) GetClaimMaterials(claimID int64) ([]*model.ClaimMaterial, error) {
	query := `
		SELECT id, claim_id, file_name, file_path, file_size, file_type, thumbnail_path, created_at
		FROM claim_materials
		WHERE claim_id = ?
		ORDER BY id ASC
	`
	rows, err := r.db.Query(query, claimID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var materials []*model.ClaimMaterial
	for rows.Next() {
		m := &model.ClaimMaterial{}
		if err := rows.Scan(
			&m.ID, &m.ClaimID, &m.FileName, &m.FilePath,
			&m.FileSize, &m.FileType, &m.ThumbnailPath, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		materials = append(materials, m)
	}
	return materials, rows.Err()
}
```

注意：`creator.go` 顶部已经 import 了 `time`，无需重复添加。

- [ ] **Step 2: 编译验证**

```bash
cd /Users/ke/code/miao-repo/miao && go build ./...
```

期望：无编译错误

- [ ] **Step 3: 提交**

```bash
cd /Users/ke/code/miao-repo/miao
git add internal/repository/creator.go
git commit -m "feat: 新增 ClaimMaterial repository 方法"
```

---

## Task 3: 更新 SubmitClaim handler 支持媒体文件

**Files:**
- Modify: `miao/internal/handler/creator.go`

当前 `SubmitClaim` 函数在 `creator.go` 约第 249 行。它在调用 `creatorRepo.SubmitClaim(claimID, now, req.Content)` 后返回成功。

- [ ] **Step 1: 在 SubmitClaim 成功提交后批量保存媒体文件**

找到 SubmitClaim 函数中调用 `creatorRepo.SubmitClaim` 之后、发送通知之前的位置。当前代码结构约为：

```go
err = creatorRepo.SubmitClaim(claimID, now, req.Content)
if err != nil {
    c.JSON(http.StatusInternalServerError, Response{...})
    return
}

// Get task info and send notification
```

在 `err = creatorRepo.SubmitClaim(...)` 成功后，插入媒体保存逻辑：

```go
err = creatorRepo.SubmitClaim(claimID, now, req.Content)
if err != nil {
    c.JSON(http.StatusInternalServerError, Response{
        Code:    50002,
        Message: "提交失败",
    })
    return
}

// 保存媒体文件
for _, mat := range req.Materials {
    material := &model.ClaimMaterial{
        ClaimID:       claimID,
        FileName:      mat.FileName,
        FilePath:      mat.FilePath,
        FileSize:      mat.FileSize,
        FileType:      mat.FileType,
        ThumbnailPath: mat.ThumbnailPath,
    }
    if err := creatorRepo.CreateClaimMaterial(material); err != nil {
        // 媒体保存失败不影响提交本身，记录日志即可
        log.Printf("Failed to save claim material for claim %d: %v", claimID, err)
    }
}
```

- [ ] **Step 2: 确认 log 包已 import**

查看 `creator.go` 顶部的 import 列表。如果没有 `"log"`，添加它。

- [ ] **Step 3: 编译验证**

```bash
cd /Users/ke/code/miao-repo/miao && go build ./...
```

期望：无编译错误

- [ ] **Step 4: 提交**

```bash
cd /Users/ke/code/miao-repo/miao
git add internal/handler/creator.go
git commit -m "feat: SubmitClaim 支持媒体文件列表"
```

---

## Task 4: 更新 /works 接口返回 materials 字段

**Files:**
- Modify: `miao/internal/handler/work.go`

当前 `work.go` 的 `ListApprovedWorks` 和 `GetWork` 没有返回媒体文件。

- [ ] **Step 1: 更新 work.go 的 init/依赖，使用 CreatorRepository**

`work.go` 当前通过 `GetDB()` 动态获取 db。在每个 handler 里已经创建了 `creatorRepo`，只需追加 `GetClaimMaterials` 调用。

在 `ListApprovedWorks` 中，找到 works append 的地方：

```go
works = append(works, gin.H{
    "id":             claim.ID,
    ...
    "review_at":      claim.ReviewAt,
})
```

改为：

```go
materials, _ := creatorRepo.GetClaimMaterials(claim.ID)
if materials == nil {
    materials = []*model.ClaimMaterial{}
}
works = append(works, gin.H{
    "id":             claim.ID,
    "task_id":        claim.TaskID,
    "task_title":     taskTitle,
    "task_category":  taskCategory,
    "creator_id":     claim.CreatorID,
    "creator_name":   creatorName,
    "creator_avatar": creatorAvatar,
    "content":        claim.Content,
    "reward":         claim.CreatorReward,
    "submit_at":      claim.SubmitAt,
    "review_at":      claim.ReviewAt,
    "materials":      materials,
})
```

- [ ] **Step 2: 更新 GetWork 同样返回 materials**

在 `GetWork` 函数，找到最终 `c.JSON` 调用，在 Data 中添加 materials：

```go
materials, _ := creatorRepo.GetClaimMaterials(claim.ID)
if materials == nil {
    materials = []*model.ClaimMaterial{}
}

c.JSON(http.StatusOK, Response{
    Code:    0,
    Message: "success",
    Data: gin.H{
        "id":             claim.ID,
        "task_id":        claim.TaskID,
        "task_title":     taskTitle,
        "task_category":  taskCategory,
        "creator_id":     claim.CreatorID,
        "creator_name":   creatorName,
        "creator_avatar": creatorAvatar,
        "content":        claim.Content,
        "reward":         claim.CreatorReward,
        "submit_at":      claim.SubmitAt,
        "review_at":      claim.ReviewAt,
        "materials":      materials,
    },
})
```

- [ ] **Step 3: 添加 model import（如缺失）**

`work.go` 顶部 import 中确保有 `"github.com/tans/miao/internal/model"`。

- [ ] **Step 4: 编译验证**

```bash
cd /Users/ke/code/miao-repo/miao && go build ./...
```

期望：无编译错误

- [ ] **Step 5: 提交**

```bash
cd /Users/ke/code/miao-repo/miao
git add internal/handler/work.go
git commit -m "feat: /works 接口返回 claim_materials 媒体文件列表"
```

---

## Task 5: 清理废弃表——从 migration.go 移除 submissions/messages

**Files:**
- Modify: `miao/internal/database/migration.go`

目标：
1. 从 `schemaSQL`（v1 初始 schema）中删除 `messages`、`submissions`、`submission_materials` 表定义及相关索引
2. 将 migration v7（`submissions_table`）替换为空操作（已有数据库不能回退，但新建数据库不再创建这些表）

- [ ] **Step 1: 清理 schemaSQL 中的废弃表**

在 `migration.go` 的 `schemaSQL` 常量中，删除以下内容（约第 271-328 行）：

删除整个 messages 表定义：
```sql
CREATE TABLE IF NOT EXISTS messages (
    ...
);
```

删除整个 submissions 表定义：
```sql
CREATE TABLE IF NOT EXISTS submissions (
    ...
);
```

删除整个 submission_materials 表定义：
```sql
CREATE TABLE IF NOT EXISTS submission_materials (
    ...
);
```

删除对应的索引：
```sql
CREATE INDEX IF NOT EXISTS idx_submissions_task_id ON submissions(task_id);
CREATE INDEX IF NOT EXISTS idx_submissions_creator_id ON submissions(creator_id);
CREATE INDEX IF NOT EXISTS idx_submissions_status ON submissions(status);
CREATE INDEX IF NOT EXISTS idx_messages_user_id ON messages(user_id);
CREATE INDEX IF NOT EXISTS idx_messages_is_read ON messages(is_read);
```

- [ ] **Step 2: 将 migration v7 改为 DROP 旧表**

找到 migration Version: 7，将其 SQL 改为：

```go
{
    Version: 7,
    Name:    "drop_submissions_messages",
    SQL: `
DROP TABLE IF EXISTS submission_materials;
DROP TABLE IF EXISTS submissions;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS accounts;
`,
},
```

这样已有数据库执行 v7 时会清理掉这些废弃表。

- [ ] **Step 3: 编译验证**

```bash
cd /Users/ke/code/miao-repo/miao && go build ./...
```

期望：无编译错误

- [ ] **Step 4: 提交**

```bash
cd /Users/ke/code/miao-repo/miao
git add internal/database/migration.go
git commit -m "refactor: migration v7 改为删除废弃表（submissions/messages/accounts）"
```

---

## Task 6: 清理孤立 Go 代码（account model/repo）

**Files:**
- Delete: `miao/internal/model/account.go`
- Modify: `miao/internal/repository/account.go`
- Modify: `miao/internal/handler/account.go`（如引用了 Account model）

- [ ] **Step 1: 检查 Account model 的所有引用**

```bash
cd /Users/ke/code/miao-repo/miao
grep -r "model\.Account\b" --include="*.go" .
grep -r "model\.RechargeRequest\b" --include="*.go" .
grep -r "model\.PrepayRequest\b" --include="*.go" .
```

记录所有引用文件。

- [ ] **Step 2: 将 RechargeRequest/PrepayRequest 迁移**

如果 `model.RechargeRequest` 只在 `handler/account.go` 中使用，将这两个 struct 直接移入 `handler/account.go` 作为本地类型（或保留在 model 中但从 account.go 移到 user.go）。

如果 grep 结果显示 RechargeRequest 只在 account.go handler 里用，在 handler/account.go 文件顶部定义：

```go
type rechargeRequest struct {
    Amount float64 `json:"amount" binding:"required,gt=0"`
}
```

将所有 `model.RechargeRequest` 替换为 `rechargeRequest`（小写，包内可见即可）。

- [ ] **Step 3: 删除 model/account.go**

```bash
rm /Users/ke/code/miao-repo/miao/internal/model/account.go
```

- [ ] **Step 4: 清理 repository/account.go 中的废弃方法**

删除以下方法（它们查询 accounts 表，该表已不存在）：
- `CreateAccount`
- `GetAccountByUserID`
- `GetAccountByID`
- `UpdateAccount`

只保留：
- `CreateTransaction`
- `ListTransactions`
- `ListTransactionsByUserID`

同时删除文件顶部对 `model.Account` 的引用（import 中如果只剩 model.Transaction 则保留 model import）。

- [ ] **Step 5: 编译验证**

```bash
cd /Users/ke/code/miao-repo/miao && go build ./...
```

期望：无编译错误。如有编译错误，根据错误信息修复剩余引用。

- [ ] **Step 6: 提交**

```bash
cd /Users/ke/code/miao-repo/miao
git add -A
git commit -m "refactor: 删除 Account model 和废弃 repository 方法"
```

---

## Task 7: 更新小程序 api.js

**Files:**
- Modify: `miao-mini/utils/api.js`

- [ ] **Step 1: 更新 submitClaim 支持媒体列表**

找到当前的 `submitClaim` 方法：

```js
submitClaim(claimId, data) {
    return this.request('PUT', `/creator/claim/${claimId}/submit`, data);
},
```

无需修改函数签名，只需确保调用方传入正确的 data 结构。在 api.js 旁边加注释说明新的 data 格式：

```js
// data: { content: string, materials: [{file_name, file_path, file_size, file_type, thumbnail_path}] }
submitClaim(claimId, data) {
    return this.request('PUT', `/creator/claim/${claimId}/submit`, data);
},
```

- [ ] **Step 2: 确认 getWorks 方法存在**

检查 api.js 是否已有 `getWorks` 方法。如果存在：无需修改。

如果不存在，在 `getBalance()` 附近添加：

```js
getWorks(params = {}) {
    const q = [];
    if (params.page) q.push(`page=${params.page}`);
    if (params.limit) q.push(`limit=${params.limit}`);
    const qs = q.length ? '?' + q.join('&') : '';
    return this.request('GET', '/works' + qs, null, true);
},

getWork(id) {
    return this.request('GET', `/works/${id}`, null, true);
},
```

- [ ] **Step 3: 提交**

```bash
cd /Users/ke/code/miao-repo/miao-mini
git add utils/api.js
git commit -m "feat: api.js 添加 getWorks/getWork，更新 submitClaim 注释"
```

---

## 验证清单

- [ ] `go build ./...` 在 miao 目录通过
- [ ] migration v9 存在（claim_materials 表）
- [ ] migration v7 会 DROP submissions/messages/accounts 表
- [ ] schemaSQL 中不包含 submissions/messages 表定义
- [ ] `model/account.go` 已删除
- [ ] `repository/account.go` 只含 transaction 相关方法
- [ ] `GET /api/v1/works` 返回含 `materials` 数组的作品列表
- [ ] `PUT /creator/claim/:id/submit` 接受 `materials` 字段
- [ ] 小程序 `api.js` 有 `getWorks()` 和 `getWork()` 方法
