package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetBusinessStatsUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("GET", "/api/v1/business/stats", nil)

	GetBusinessStats(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 40101, response.Code)
}

func TestGetBusinessExpenseChartUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("GET", "/api/v1/business/chart/expense?period=7d", nil)

	GetBusinessExpenseChart(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 40101, response.Code)
}

func TestGetBusinessExpenseChartPeriods(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"7 day period", "period=7d"},
		{"30 day period", "period=30d"},
		{"default period", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			path := "/api/v1/business/chart/expense"
			if tt.query != "" {
				path += "?" + tt.query
			}
			c.Request = httptest.NewRequest("GET", path, nil)
			c.Set("user_id", int64(1))

			GetBusinessExpenseChart(c)

			// Should not return 401 (unauthorized will return that)
			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestGetCreatorStatsUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("GET", "/api/v1/creator/stats", nil)

	GetCreatorStats(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 40101, response.Code)
}

func TestGetCreatorIncomeChartUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("GET", "/api/v1/creator/chart/income?period=7d", nil)

	GetCreatorIncomeChart(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 40101, response.Code)
}

func TestGetCreatorIncomeChartPeriods(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"7 day period", "period=7d"},
		{"30 day period", "period=30d"},
		{"default period", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			path := "/api/v1/creator/chart/income"
			if tt.query != "" {
				path += "?" + tt.query
			}
			c.Request = httptest.NewRequest("GET", path, nil)
			c.Set("user_id", int64(1))

			GetCreatorIncomeChart(c)

			// Should not return 401
			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
		})
	}
}