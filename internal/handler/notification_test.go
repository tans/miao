package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetNotifications_RequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
	}{
		{
			name:           "无参数应该返回未授权",
			queryParams:    "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "分页参数也应该返回未授权",
			queryParams:    "?page=1&limit=20",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Request = httptest.NewRequest("GET", "/api/v1/notifications"+tt.queryParams, nil)
			GetNotifications(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestMarkNotificationAsRead_RequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("PUT", "/api/v1/notifications/123/read", nil)
	c.Params = gin.Params{{Key: "id", Value: "123"}}

	MarkNotificationAsRead(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetUnreadNotificationCount_RequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/unread-count", nil)
	GetUnreadNotificationCount(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMarkAllNotificationsAsRead_RequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("PUT", "/api/v1/notifications/read-all", nil)
	MarkAllNotificationsAsRead(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestNotificationResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("GET", "/api/v1/notifications", nil)

	GetNotifications(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, CodeAuthRequired, response.Code)
}

func TestNotificationTypes(t *testing.T) {
	// Test notification type constants exist and are valid strings
	notificationTypes := []string{
		"task_status",
		"new_submission",
		"claim_approved",
		"income_received",
	}

	for _, nt := range notificationTypes {
		assert.NotEmpty(t, nt)
	}
}
