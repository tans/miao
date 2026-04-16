package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetTransactionsUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("GET", "/api/v1/transactions?page=1&limit=20", nil)

	GetTransactions(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 40101, response.Code)
}

func TestGetTransactionsQueryParams(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		expectedPage   int
		expectedLimit  int
	}{
		{"default values", "", 1, 20},
		{"custom page", "page=3&limit=10", 3, 10},
		{"limit exceeds max", "page=1&limit=200", 1, 20},
		{"invalid page", "page=-1&limit=20", 1, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			path := "/api/v1/transactions"
			if tt.query != "" {
				path += "?" + tt.query
			}
			c.Request = httptest.NewRequest("GET", path, nil)
			c.Set("user_id", int64(1))

			GetTransactions(c)

			// Should not return 401 for authenticated user
			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
		})
	}
}