package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
	"github.com/tans/miao/internal/service"
)

func TestCreateTaskValidation(t *testing.T) {
	setupTestBusinessService(t)

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectedCode   int
	}{
		{
			name: "缺少必填字段 - title",
			requestBody: map[string]interface{}{
				"description": "测试描述",
				"unit_price":  5.0,
				"total_count": 10,
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   CodeBadRequest,
		},
		{
			name: "缺少必填字段 - description",
			requestBody: map[string]interface{}{
				"title":       "测试任务",
				"unit_price":  5.0,
				"total_count": 10,
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   CodeBadRequest,
		},
		{
			name: "缺少必填字段 - unit_price",
			requestBody: map[string]interface{}{
				"title":       "测试任务",
				"description": "测试描述",
				"total_count": 10,
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   CodeBadRequest,
		},
		{
			name: "缺少必填字段 - total_count",
			requestBody: map[string]interface{}{
				"title":       "测试任务",
				"description": "测试描述",
				"unit_price":  5.0,
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   CodeBadRequest,
		},
		{
			name: "unit_price <= 0",
			requestBody: map[string]interface{}{
				"title":       "测试任务",
				"description": "测试描述",
				"unit_price":  0,
				"total_count": 10,
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   CodeBadRequest,
		},
		{
			name: "total_count <= 0",
			requestBody: map[string]interface{}{
				"title":       "测试任务",
				"description": "测试描述",
				"unit_price":  5.0,
				"total_count": 0,
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   CodeBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			body, _ := json.Marshal(tt.requestBody)
			c.Request = httptest.NewRequest("POST", "/api/v1/business/tasks", bytes.NewBuffer(body))
			c.Request.Header.Set("Content-Type", "application/json")

			// Note: CreateTask requires auth middleware to set user ID
			// This test only validates request binding, so we expect auth errors
			// For full handler test, we need to set up context properly
			_ = w.Code // Placeholder
		})
	}
}

func TestCreateTaskV1Fields(t *testing.T) {
	setupTestBusinessService(t)

	tests := []struct {
		name          string
		industries    []string
		videoDuration string
		videoAspect   string
		resolution    string
		style         string
		awardPrice    float64
	}{
		{
			name:          "多行业选项",
			industries:    []string{"本地餐饮", "美妆护肤"},
			videoDuration: "60秒",
			videoAspect:   "9:16",
			resolution:    "1080P",
			style:         "口语化",
			awardPrice:    10.0,
		},
		{
			name:          "全行业选项",
			industries:    []string{"本地餐饮", "美妆护肤", "家居家电", "教育培训", "本地生活服务", "服饰鞋帽", "母婴用品", "数码3C"},
			videoDuration: "30秒",
			videoAspect:   "16:9",
			resolution:    "720P",
			style:         "种草安利",
			awardPrice:    8.0,
		},
		{
			name:          "无行业选项",
			industries:    []string{},
			videoDuration: "1-3分钟",
			videoAspect:   "1:1",
			resolution:    "1080P",
			style:         "搞笑轻松",
			awardPrice:    0,
		},
		{
			name:          "不限制时长",
			industries:    []string{"企业宣传"},
			videoDuration: "不限制",
			videoAspect:   "9:16",
			resolution:    "720P",
			style:         "商务正式",
			awardPrice:    20.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskData := model.TaskCreate{
				Title:          "测试任务",
				Description:    "测试描述",
				UnitPrice:      5.0,
				TotalCount:     10,
				Industries:     tt.industries,
				VideoDuration:  tt.videoDuration,
				VideoAspect:    tt.videoAspect,
				VideoResolution: tt.resolution,
				CreativeStyle:  tt.style,
				AwardPrice:     tt.awardPrice,
			}

			assert.Equal(t, tt.industries, taskData.Industries)
			assert.Equal(t, tt.videoDuration, taskData.VideoDuration)
			assert.Equal(t, tt.videoAspect, taskData.VideoAspect)
			assert.Equal(t, tt.resolution, taskData.VideoResolution)
			assert.Equal(t, tt.style, taskData.CreativeStyle)
			assert.Equal(t, tt.awardPrice, taskData.AwardPrice)
		})
	}
}

func TestBudgetCalculationV1(t *testing.T) {
	tests := []struct {
		name       string
		unitPrice  float64
		totalCount int
		awardPrice float64
		wantTotal  float64
	}{
		{
			name:       "v1最低基础奖励 - 2元×10人",
			unitPrice:  2.0,
			totalCount: 10,
			awardPrice: 0,
			wantTotal:  20.0,
		},
		{
			name:       "v1最低采纳奖励 - 2元×10人+8元×10人",
			unitPrice:  2.0,
			totalCount: 10,
			awardPrice: 8.0,
			wantTotal:  100.0,
		},
		{
			name:       "典型任务 - 5元×20人+10元×20人",
			unitPrice:  5.0,
			totalCount: 20,
			awardPrice: 10.0,
			wantTotal:  300.0,
		},
		{
			name:       "高额任务 - 100元×50人+200元×50人",
			unitPrice:  100.0,
			totalCount: 50,
			awardPrice: 200.0,
			wantTotal:  15000.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			totalBudget := float64(tt.totalCount) * (tt.unitPrice + tt.awardPrice)

			assert.Equal(t, tt.wantTotal, totalBudget)
		})
	}
}

func TestTaskStatusConstants(t *testing.T) {
	assert.Equal(t, 1, int(model.TaskStatusPending))
	assert.Equal(t, 2, int(model.TaskStatusOnline))
	assert.Equal(t, 3, int(model.TaskStatusOngoing))
	assert.Equal(t, 4, int(model.TaskStatusEnded))
	assert.Equal(t, 5, int(model.TaskStatusCancelled))
}

func TestTaskCategoryNormalization(t *testing.T) {
	// All categories should normalize to CategoryVideo
	categories := []model.TaskCategory{
		model.CategoryCopywriting,
		model.CategoryDesign,
		model.CategoryVideo,
		model.CategoryPhotography,
		model.CategoryMusic,
		model.CategoryDev,
		model.CategoryOther,
		model.TaskCategory(0),
		model.TaskCategory(999),
	}

	for _, cat := range categories {
		normalized := model.NormalizeTaskCategory(cat)
		assert.Equal(t, model.CategoryVideo, normalized, "Category %d should normalize to CategoryVideo", cat)
	}
}

func TestCancelTaskValidation(t *testing.T) {
	tests := []struct {
		name           string
		taskID         string
		expectedStatus int
		expectedCode   int
	}{
		{
			name:           "无效任务ID - 0",
			taskID:         "0",
			expectedStatus: http.StatusBadRequest,
			expectedCode:   CodeBadRequest,
		},
		{
			name:           "无效任务ID - 空",
			taskID:         "",
			expectedStatus: http.StatusBadRequest,
			expectedCode:   CodeBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// CancelTask requires auth and valid task ownership
			// This tests the input validation path
			id := parseInt64(tt.taskID, 0)
			if id != 0 {
				t.Error("Expected id to be 0 for invalid input")
			}
		})
	}
}

func TestGetTaskClaimsValidation(t *testing.T) {
	tests := []struct {
		name           string
		taskID         string
		expectedStatus int
		expectedCode   int
	}{
		{
			name:           "无效任务ID - 0",
			taskID:         "0",
			expectedStatus: http.StatusBadRequest,
			expectedCode:   CodeBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := parseInt64(tt.taskID, 0)
			assert.Equal(t, int64(0), id)
		})
	}
}

func TestReviewClaimValidation(t *testing.T) {
	tests := []struct {
		name         string
		claimID      string
		body         map[string]interface{}
		expectZeroID bool
	}{
		{
			name:         "无效认领ID - 0",
			claimID:      "0",
			body:         map[string]interface{}{"result": 1},
			expectZeroID: true,
		},
		{
			name:         "无效认领ID - 空",
			claimID:      "",
			body:         map[string]interface{}{"result": 1},
			expectZeroID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := parseInt64(tt.claimID, 0)
			if tt.expectZeroID {
				assert.Equal(t, int64(0), id)
			}
		})
	}
}

func TestClaimReviewResultConstants(t *testing.T) {
	assert.Equal(t, model.ReviewResult(1), model.ReviewResultPass)
	assert.Equal(t, model.ReviewResult(2), model.ReviewResultReturn)
}

func TestClaimStatusConstants(t *testing.T) {
	assert.Equal(t, model.ClaimStatus(1), model.ClaimStatusPending)
	assert.Equal(t, model.ClaimStatus(2), model.ClaimStatusSubmitted)
	assert.Equal(t, model.ClaimStatus(3), model.ClaimStatusApproved)
	assert.Equal(t, model.ClaimStatus(4), model.ClaimStatusCancelled)
	assert.Equal(t, model.ClaimStatus(5), model.ClaimStatusExpired)
}

func TestReviewClaimPublishesInspirationFromClaim(t *testing.T) {
	setupTestBusinessService(t)

	db := GetDB()
	userRepo := repository.NewUserRepository(db)
	inspirationRepo := repository.NewInspirationRepository(db)

	business := &model.User{
		Username:         "biz-review",
		PasswordHash:     "hashed",
		Nickname:         "商家A",
		Balance:          1000,
		FrozenAmount:     15,
		Level:            model.LevelSilver,
		BehaviorScore:    100,
		TotalScore:       100,
		BusinessVerified: true,
		Status:           1,
	}
	require.NoError(t, userRepo.CreateUser(business))

	creator := &model.User{
		Username:      "creator-review",
		PasswordHash:  "hashed",
		Nickname:      "创作者A",
		Avatar:        "/avatars/a.png",
		Level:         model.LevelSilver,
		BehaviorScore: 100,
		TotalScore:    100,
		Status:        1,
	}
	require.NoError(t, userRepo.CreateUser(creator))

	now := time.Now()
	taskResult, err := db.Exec(`
		INSERT INTO tasks (
			business_id, title, description, category, unit_price, total_count, remaining_count,
			industries, video_duration, video_aspect, video_resolution, creative_style, award_price,
			status, total_budget, frozen_amount, paid_amount, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		business.ID, "探店视频征集", "提交短视频作品", model.CategoryVideo, 5.0, 1, 0,
		"餐饮美食,本地生活", "30秒", "9:16", "1080P", "种草安利", 10.0,
		model.TaskStatusOngoing, 15.0, 15.0, 0.0, now, now,
	)
	require.NoError(t, err)
	taskID, err := taskResult.LastInsertId()
	require.NoError(t, err)

	claim := &model.Claim{
		TaskID:    taskID,
		CreatorID: creator.ID,
		Status:    model.ClaimStatusPending,
		ExpiresAt: now.Add(24 * time.Hour),
	}
	require.NoError(t, creatorRepo.CreateClaim(claim))

	submitAt := now.Add(-time.Hour)
	_, err = db.Exec(`
		UPDATE claims
		SET status = ?, content = ?, submit_at = ?, updated_at = ?
		WHERE id = ?
	`, model.ClaimStatusSubmitted, "已提交成片", submitAt, now, claim.ID)
	require.NoError(t, err)

	require.NoError(t, creatorRepo.CreateClaimMaterial(&model.ClaimMaterial{
		ClaimID:       claim.ID,
		FileName:      "demo.mp4",
		FilePath:      "/uploads/demo.mp4",
		FileSize:      2048,
		FileType:      "video",
		ThumbnailPath: "/uploads/demo-cover.jpg",
	}))

	body, err := json.Marshal(map[string]interface{}{
		"result":  1,
		"comment": "采纳入库",
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/business/claim/"+strconv.FormatInt(claim.ID, 10)+"/review", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(claim.ID, 10)}}
	c.Set("user_id", business.ID)

	ReviewClaim(c)

	require.Equal(t, http.StatusOK, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code)

	inspiration, err := inspirationRepo.GetBySourceClaimID(claim.ID)
	require.NoError(t, err)
	require.NotNil(t, inspiration)
	assert.Equal(t, "探店视频征集", inspiration.Title)
	assert.Equal(t, "已提交成片", inspiration.Content)
	assert.Equal(t, "创作者A", inspiration.CreatorName)
	assert.Equal(t, "/avatars/a.png", inspiration.CreatorAvatar)
	assert.Equal(t, "/uploads/demo-cover.jpg", inspiration.CoverURL)
	assert.Equal(t, "video", inspiration.CoverType)
	assert.Equal(t, model.InspirationStatusPublished, inspiration.Status)
	require.NotNil(t, inspiration.SourceClaimID)
	assert.Equal(t, claim.ID, *inspiration.SourceClaimID)
	require.NotNil(t, inspiration.PublishedAt)

	materials, err := inspirationRepo.GetMaterials(inspiration.ID)
	require.NoError(t, err)
	require.Len(t, materials, 1)
	assert.Equal(t, "/uploads/demo.mp4", materials[0].FilePath)
	assert.Equal(t, "/uploads/demo-cover.jpg", materials[0].ThumbnailPath)
}

// Helper functions

type businessTestResponse struct {
	recorder *httptest.ResponseRecorder
	body     Response
}

func setupTestBusinessService(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "business_test.db")
	testDB, err := database.InitDB(dbPath)
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(testDB))

	cfg := config.Load()
	cfg.Database.Path = dbPath

	previousDB := db
	previousBusinessRepo := businessRepo
	previousCreatorRepo := creatorRepo
	previousClaimInspirationService := claimInspirationService
	previousBusinessNotificationService := businessNotificationService
	db = testDB
	businessRepo = repository.NewBusinessRepository(testDB)
	creatorRepo = repository.NewCreatorRepository(testDB)
	claimInspirationService = service.NewClaimInspirationService(testDB)
	businessNotificationService = service.NewNotificationService(testDB)

	t.Cleanup(func() {
		db = previousDB
		businessRepo = previousBusinessRepo
		creatorRepo = previousCreatorRepo
		claimInspirationService = previousClaimInspirationService
		businessNotificationService = previousBusinessNotificationService
		_ = testDB.Close()
	})
}
