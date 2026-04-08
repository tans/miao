package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRegister(t *testing.T) {
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
