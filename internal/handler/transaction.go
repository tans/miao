package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tans/miao/internal/middleware"
)

// GetTransactions 获取交易记录（支持分页）
// GET /api/v1/transactions?page=1&limit=20
func GetTransactions(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{
			Code:    40101,
			Message: "未登录",
			Data:    nil,
		})
		return
	}

	// 解析分页参数
	page := parseInt(c.DefaultQuery("page", "1"), 1)
	limit := parseInt(c.DefaultQuery("limit", "20"), 20)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// 查询交易记录
	accountRepo := GetAccountRepo()
	transactions, total, err := accountRepo.ListTransactionsByUserID(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    50001,
			Message: "获取交易记录失败",
			Data:    nil,
		})
		return
	}

	var formattedTransactions []gin.H
	for _, tx := range transactions {
		formattedTransactions = append(formattedTransactions, formatTransaction(tx))
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
			"data":  formattedTransactions,
		},
	})
}
