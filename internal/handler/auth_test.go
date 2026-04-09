package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/repository"
	"github.com/tans/miao/internal/service"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRegister(t *testing.T) {
	setupTestAuthService(t)

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectedCode   int
	}{
		{
			name: "缺少必填字段",
			requestBody: map[string]interface{}{
				"username": "testuser",
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   CodeBadRequest,
		},
		{
			name: "用户名太短",
			requestBody: map[string]interface{}{
				"username": "ab",
				"password": "password123",
				"phone":    "13800138001",
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   CodeBadRequest,
		},
		{
			name: "密码太短",
			requestBody: map[string]interface{}{
				"username": "testuser456",
				"password": "12345",
				"phone":    "13800138002",
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
			c.Request = httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
			c.Request.Header.Set("Content-Type", "application/json")

			Register(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response Response
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, response.Code)
		})
	}
}

func TestLogin(t *testing.T) {
	setupTestAuthService(t)

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectedCode   int
	}{
		{
			name: "缺少必填字段",
			requestBody: map[string]interface{}{
				"username": "testuser",
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
			c.Request = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
			c.Request.Header.Set("Content-Type", "application/json")

			Login(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response Response
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, response.Code)
		})
	}
}

func TestRegisterReturnsAuthenticatedSession(t *testing.T) {
	setupTestAuthService(t)

	response := performAuthRequest(t, Register, "/api/v1/auth/register", map[string]interface{}{
		"username": "autologin_user",
		"password": "password123",
		"phone":    "13800138009",
	})

	assert.Equal(t, http.StatusOK, response.recorder.Code)
	assert.Equal(t, CodeSuccess, response.body.Code)

	data := response.dataMap(t)
	token, ok := data["token"].(string)
	require.True(t, ok, "register response should include token")
	assert.NotEmpty(t, token)

	userData, ok := data["user"].(map[string]interface{})
	require.True(t, ok, "register response should include user object")
	assert.Equal(t, "autologin_user", userData["username"])
}

func TestLoginDistinguishesUnknownUserAndWrongPassword(t *testing.T) {
	setupTestAuthService(t)

	_, err := authService.Register("known_user", "password123", "13800138010", false, "", "")
	require.NoError(t, err)

	t.Run("用户名不存在", func(t *testing.T) {
		response := performAuthRequest(t, Login, "/api/v1/auth/login", map[string]interface{}{
			"username": "missing_user",
			"password": "password123",
		})

		assert.Equal(t, http.StatusNotFound, response.recorder.Code)
		assert.Equal(t, CodeUserNotFound, response.body.Code)
		assert.Equal(t, "用户名不存在", response.body.Message)
	})

	t.Run("密码错误", func(t *testing.T) {
		response := performAuthRequest(t, Login, "/api/v1/auth/login", map[string]interface{}{
			"username": "known_user",
			"password": "wrong-password",
		})

		assert.Equal(t, http.StatusUnauthorized, response.recorder.Code)
		assert.Equal(t, CodeInvalidPassword, response.body.Code)
		assert.Equal(t, "密码错误", response.body.Message)
	})
}

func TestErrorResponse(t *testing.T) {
	tests := []struct {
		name            string
		code            int
		customMessage   []string
		expectedMessage string
	}{
		{
			name:            "标准错误消息",
			code:            CodeBadRequest,
			customMessage:   nil,
			expectedMessage: "请求参数错误",
		},
		{
			name:            "自定义错误消息",
			code:            CodeBadRequest,
			customMessage:   []string{"自定义错误"},
			expectedMessage: "自定义错误",
		},
		{
			name:            "未知错误码",
			code:            99999,
			customMessage:   nil,
			expectedMessage: "未知错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := ErrorResponse(tt.code, tt.customMessage...)
			assert.Equal(t, tt.code, resp.Code)
			assert.Equal(t, tt.expectedMessage, resp.Message)
			assert.Nil(t, resp.Data)
		})
	}
}

func TestSuccessResponse(t *testing.T) {
	data := map[string]string{"key": "value"}
	resp := SuccessResponse(data)

	assert.Equal(t, CodeSuccess, resp.Code)
	assert.Equal(t, "成功", resp.Message)
	assert.Equal(t, data, resp.Data)
}

type authTestResponse struct {
	recorder *httptest.ResponseRecorder
	body     Response
}

func (r authTestResponse) dataMap(t *testing.T) map[string]interface{} {
	t.Helper()

	data, ok := r.body.Data.(map[string]interface{})
	require.True(t, ok, "response data should be an object")
	return data
}

func setupTestAuthService(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "auth_test.db")
	db, err := database.InitDB(dbPath)
	require.NoError(t, err)

	schemaPath := filepath.Join("..", "..", "migrations", "schema.sql")
	schema, err := os.ReadFile(schemaPath)
	require.NoError(t, err)

	err = database.RunMigrations(db, string(schema))
	require.NoError(t, err)

	cfg := config.Load()
	cfg.Database.Path = dbPath

	previousAuthService := authService
	authService = service.NewAuthService(repository.NewUserRepository(db), cfg)

	t.Cleanup(func() {
		authService = previousAuthService
		_ = db.Close()
	})
}

func performAuthRequest(t *testing.T, handlerFunc gin.HandlerFunc, path string, requestBody map[string]interface{}) authTestResponse {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handlerFunc(c)

	var response Response
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	return authTestResponse{
		recorder: w,
		body:     response,
	}
}
