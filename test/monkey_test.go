package test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tans/miao/internal/config"
)

const (
	BaseURL = "http://localhost:8888"
	APIURL  = BaseURL + "/api/v1"
)

// TestUser 测试用户结构
type TestUser struct {
	Username string
	Password string
	Phone    string
	Token    string
	UserID   int
}

// APIResponse 通用API响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func responseDataMap(t *testing.T, resp *APIResponse) map[string]interface{} {
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("响应 data 不是对象: %T", resp.Data)
	}
	return data
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	cfg := config.Load()
	dbPath := cfg.Database.Path
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Clean(filepath.Join("..", dbPath))
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}

	return db
}

func publishTaskForTest(t *testing.T, taskID int) {
	t.Helper()

	db := openTestDB(t)
	defer db.Close()

	now := time.Now()
	_, err := db.Exec(
		`UPDATE tasks SET status = ?, review_at = ?, publish_at = ?, updated_at = ? WHERE id = ?`,
		2,
		now,
		now,
		now,
		taskID,
	)
	if err != nil {
		t.Fatalf("设置任务上线失败: %v", err)
	}
}

func promoteCreatorForClaim(t *testing.T, userID int) {
	t.Helper()

	db := openTestDB(t)
	defer db.Close()

	_, err := db.Exec(
		`UPDATE users SET level = ?, updated_at = ? WHERE id = ?`,
		2,
		time.Now(),
		userID,
	)
	if err != nil {
		t.Fatalf("设置创作者等级失败: %v", err)
	}
}

// TestMonkeyFullFlow 完整业务流程测试
func TestMonkeyFullFlow(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	// 生成随机用户名
	timestamp := time.Now().Unix()
	creatorUser := &TestUser{
		Username: fmt.Sprintf("creator_%d", timestamp),
		Password: "test123456",
		Phone:    fmt.Sprintf("138%08d", rand.Intn(100000000)),
	}
	businessUser := &TestUser{
		Username: fmt.Sprintf("business_%d", timestamp),
		Password: "test123456",
		Phone:    fmt.Sprintf("139%08d", rand.Intn(100000000)),
	}

	t.Run("1. 注册创作者账号", func(t *testing.T) {
		testRegister(t, creatorUser)
	})

	t.Run("2. 注册商家账号", func(t *testing.T) {
		testRegister(t, businessUser)
	})

	t.Run("3. 创作者登录", func(t *testing.T) {
		testLogin(t, creatorUser)
	})

	t.Run("4. 商家登录", func(t *testing.T) {
		testLogin(t, businessUser)
	})

	t.Run("5. 商家充值", func(t *testing.T) {
		testRecharge(t, businessUser, 10000.0)
	})

	var taskID int
	t.Run("6. 商家发布任务", func(t *testing.T) {
		taskID = testCreateTask(t, businessUser)
	})

	t.Run("6.1 将任务标记为已上线", func(t *testing.T) {
		publishTaskForTest(t, taskID)
	})

	t.Run("6.2 提升创作者到白银等级", func(t *testing.T) {
		promoteCreatorForClaim(t, creatorUser.UserID)
	})

	var claimID int
	t.Run("7. 创作者浏览任务大厅", func(t *testing.T) {
		testBrowseTasks(t, creatorUser)
	})

	t.Run("8. 创作者认领任务", func(t *testing.T) {
		claimID = testClaimTask(t, creatorUser, taskID)
	})

	t.Run("9. 创作者查看我的认领", func(t *testing.T) {
		testViewMyClaims(t, creatorUser)
	})

	t.Run("10. 创作者提交作品", func(t *testing.T) {
		testSubmitWork(t, creatorUser, claimID)
	})

	t.Run("11. 商家查看投稿", func(t *testing.T) {
		testViewSubmissions(t, businessUser, taskID)
	})

	t.Run("12. 商家验收作品", func(t *testing.T) {
		testReviewSubmission(t, businessUser, claimID, true)
	})

	t.Run("13. 创作者查看钱包", func(t *testing.T) {
		testViewWallet(t, creatorUser)
	})

	t.Run("14. 创作者查看收益明细", func(t *testing.T) {
		testViewTransactions(t, creatorUser)
	})

	t.Run("15. 商家查看资金流水", func(t *testing.T) {
		testViewTransactions(t, businessUser)
	})

	t.Run("16. 创作者查看个人资料", func(t *testing.T) {
		testViewProfile(t, creatorUser)
	})

	t.Run("17. 商家查看工作台统计", func(t *testing.T) {
		testViewDashboard(t, businessUser)
	})

	t.Run("18. 创作者查看工作台统计", func(t *testing.T) {
		testViewDashboard(t, creatorUser)
	})

	t.Logf("✅ 完整业务流程测试通过！")
	t.Logf("创作者: %s (ID: %d)", creatorUser.Username, creatorUser.UserID)
	t.Logf("商家: %s (ID: %d)", businessUser.Username, businessUser.UserID)
	t.Logf("任务ID: %d, 认领ID: %d", taskID, claimID)
}

// testRegister 测试注册
func testRegister(t *testing.T, user *TestUser) {
	payload := map[string]string{
		"username": user.Username,
		"password": user.Password,
		"phone":    user.Phone,
	}

	resp := apiRequest(t, "POST", "/auth/register", payload, "")
	if resp.Code != 0 {
		t.Fatalf("注册失败: %s", resp.Message)
	}

	t.Logf("✅ 注册成功: %s", user.Username)
}

// testLogin 测试登录
func testLogin(t *testing.T, user *TestUser) {
	payload := map[string]string{
		"username": user.Username,
		"password": user.Password,
	}

	resp := apiRequest(t, "POST", "/auth/login", payload, "")
	if resp.Code != 0 {
		t.Fatalf("登录失败: %s", resp.Message)
	}

	// 提取token和user_id
	data := responseDataMap(t, resp)
	if userData, ok := data["user"].(map[string]interface{}); ok {
		if id, ok := userData["id"].(float64); ok {
			user.UserID = int(id)
		}
	}
	if token, ok := data["token"].(string); ok {
		user.Token = token
	}

	if user.Token == "" {
		t.Fatalf("登录成功但未获取到token")
	}

	t.Logf("✅ 登录成功: %s (Token: %s...)", user.Username, user.Token[:20])
}

// testRecharge 测试充值
func testRecharge(t *testing.T, user *TestUser, amount float64) {
	payload := map[string]interface{}{
		"amount": amount,
	}

	resp := apiRequest(t, "POST", "/business/recharge", payload, user.Token)
	if resp.Code != 0 {
		t.Fatalf("充值失败: %s", resp.Message)
	}

	t.Logf("✅ 充值成功: %.2f 元", amount)
}

// testCreateTask 测试发布任务
func testCreateTask(t *testing.T, user *TestUser) int {
	payload := map[string]interface{}{
		"title":       fmt.Sprintf("测试视频任务_%d", time.Now().Unix()),
		"description": "这是一个自动化测试视频任务，请按要求完成脚本、拍摄与剪辑交付。",
		"category":    3,
		"unit_price":  100.0,
		"total_count": 5,
		"deadline":    time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
	}

	resp := apiRequest(t, "POST", "/business/tasks", payload, user.Token)
	if resp.Code != 0 {
		t.Fatalf("发布任务失败: %s", resp.Message)
	}

	taskID := int(responseDataMap(t, resp)["task_id"].(float64))
	t.Logf("✅ 发布任务成功: ID=%d", taskID)
	return taskID
}

// testBrowseTasks 测试浏览任务大厅
func testBrowseTasks(t *testing.T, user *TestUser) {
	resp := apiRequest(t, "GET", "/creator/tasks", nil, user.Token)
	if resp.Code != 0 {
		t.Fatalf("浏览任务失败: %s", resp.Message)
	}

	tasks, ok := responseDataMap(t, resp)["data"].([]interface{})
	if !ok {
		t.Fatalf("任务列表格式错误")
	}

	t.Logf("✅ 浏览任务大厅成功: 共 %d 个任务", len(tasks))
}

// testClaimTask 测试认领任务
func testClaimTask(t *testing.T, user *TestUser, taskID int) int {
	payload := map[string]interface{}{
		"task_id": taskID,
	}

	resp := apiRequest(t, "POST", "/creator/claim", payload, user.Token)
	if resp.Code != 0 {
		t.Fatalf("认领任务失败: %s", resp.Message)
	}

	claimID := int(responseDataMap(t, resp)["claim_id"].(float64))
	t.Logf("✅ 认领任务成功: ClaimID=%d", claimID)
	return claimID
}

// testViewMyClaims 测试查看我的认领
func testViewMyClaims(t *testing.T, user *TestUser) {
	resp := apiRequest(t, "GET", "/creator/claims", nil, user.Token)
	if resp.Code != 0 {
		t.Fatalf("查看认领失败: %s", resp.Message)
	}

	claims, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("认领列表格式错误")
	}

	t.Logf("✅ 查看我的认领成功: 共 %d 个认领", len(claims))
}

// testSubmitWork 测试提交作品
func testSubmitWork(t *testing.T, user *TestUser, claimID int) {
	payload := map[string]interface{}{
		"content": "这是我的视频交付作品，已按要求完成脚本、拍摄与剪辑。",
	}

	resp := apiRequest(t, "PUT", fmt.Sprintf("/creator/claim/%d/submit", claimID), payload, user.Token)
	if resp.Code != 0 {
		t.Fatalf("提交作品失败: %s", resp.Message)
	}

	t.Logf("✅ 提交作品成功: ClaimID=%d", claimID)
}

// testViewSubmissions 测试查看投稿
func testViewSubmissions(t *testing.T, user *TestUser, taskID int) {
	resp := apiRequest(t, "GET", fmt.Sprintf("/business/tasks/%d/claims", taskID), nil, user.Token)
	if resp.Code != 0 {
		t.Fatalf("查看投稿失败: %s", resp.Message)
	}

	t.Logf("✅ 查看投稿成功: TaskID=%d", taskID)
}

// testReviewSubmission 测试验收作品
func testReviewSubmission(t *testing.T, user *TestUser, claimID int, approve bool) {
	payload := map[string]interface{}{
		"result":  map[bool]int{true: 1, false: 2}[approve],
		"comment": "作品质量不错，符合要求。",
	}

	resp := apiRequest(t, "PUT", fmt.Sprintf("/business/claim/%d/review", claimID), payload, user.Token)
	if resp.Code != 0 {
		t.Fatalf("验收作品失败: %s", resp.Message)
	}

	t.Logf("✅ 验收作品成功: ClaimID=%d, 结果=%v", claimID, approve)
}

// testViewWallet 测试查看钱包
func testViewWallet(t *testing.T, user *TestUser) {
	resp := apiRequest(t, "GET", "/creator/wallet", nil, user.Token)
	if resp.Code != 0 {
		t.Fatalf("查看钱包失败: %s", resp.Message)
	}

	balance := responseDataMap(t, resp)["balance"].(float64)
	t.Logf("✅ 查看钱包成功: 余额=%.2f", balance)
}

// testViewTransactions 测试查看交易记录
func testViewTransactions(t *testing.T, user *TestUser) {
	endpoint := "/business/transactions"

	resp := apiRequest(t, "GET", endpoint+"?limit=10", nil, user.Token)
	if resp.Code != 0 {
		t.Fatalf("查看交易记录失败: %s", resp.Message)
	}

	t.Logf("✅ 查看交易记录成功")
}

// testViewProfile 测试查看个人资料
func testViewProfile(t *testing.T, user *TestUser) {
	resp := apiRequest(t, "GET", "/user/profile", nil, user.Token)
	if resp.Code != 0 {
		t.Fatalf("查看个人资料失败: %s", resp.Message)
	}

	t.Logf("✅ 查看个人资料成功")
}

// testViewDashboard 测试查看工作台
func testViewDashboard(t *testing.T, user *TestUser) {
	endpoint := "/business/stats"

	resp := apiRequest(t, "GET", endpoint, nil, user.Token)
	if resp.Code != 0 {
		t.Fatalf("查看工作台失败: %s", resp.Message)
	}

	t.Logf("✅ 查看工作台成功")
}

// apiRequest 发送API请求
func apiRequest(t *testing.T, method, endpoint string, payload interface{}, token string) *APIResponse {
	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("JSON序列化失败: %v", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	url := APIURL + endpoint
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		t.Fatalf("解析响应失败: %v, 响应内容: %s", err, string(respBody))
	}

	return &apiResp
}
