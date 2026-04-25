package repository

import (
	"database/sql"
	"fmt"
	"github.com/tans/miao/internal/database"
	"strings"
	"time"

	"github.com/tans/miao/internal/model"
)

type PaymentRepository struct {
	db database.DB
}

func NewPaymentRepository(db database.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

// CreatePaymentOrder creates a new payment order
func (r *PaymentRepository) CreatePaymentOrder(order *model.PaymentOrder) error {
	query := `
		INSERT INTO payment_orders (user_id, order_no, amount, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	id, err := database.InsertReturningID(r.db, query,
		order.UserID,
		order.OrderNo,
		order.Amount,
		order.Status,
		now,
		now,
	)
	if err != nil {
		return err
	}
	order.ID = id
	order.CreatedAt = now
	order.UpdatedAt = now
	return nil
}

// GetPaymentOrderByOrderNo retrieves a payment order by order number
func (r *PaymentRepository) GetPaymentOrderByOrderNo(orderNo string) (*model.PaymentOrder, error) {
	query := `
		SELECT id, user_id, order_no, amount, status, pay_result, wechat_order_id, paid_at, created_at, updated_at
		FROM payment_orders
		WHERE order_no = ?
	`
	order := &model.PaymentOrder{}
	err := r.db.QueryRow(query, orderNo).Scan(
		&order.ID,
		&order.UserID,
		&order.OrderNo,
		&order.Amount,
		&order.Status,
		&order.PayResult,
		&order.WechatOrderID,
		&order.PaidAt,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return order, nil
}

// UpdatePaymentOrderPaid updates order to paid status
func (r *PaymentRepository) UpdatePaymentOrderPaid(orderNo string, wechatOrderID string) error {
	query := `
		UPDATE payment_orders
		SET status = ?, wechat_order_id = ?, paid_at = ?, updated_at = ?
		WHERE order_no = ? AND status = ?
	`
	now := time.Now()
	result, err := r.db.Exec(query,
		model.PaymentOrderStatusPaid,
		wechatOrderID,
		now,
		now,
		orderNo,
		model.PaymentOrderStatusPending,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("订单不存在或状态不对")
	}
	return nil
}

// UpdatePaymentOrderPaidTx updates order to paid status within a DB transaction.
// Returns true when status changed from pending to paid, false when no rows updated.
func (r *PaymentRepository) UpdatePaymentOrderPaidTx(tx database.Tx, orderNo string, wechatOrderID string) (bool, error) {
	query := `
		UPDATE payment_orders
		SET status = ?, wechat_order_id = ?, paid_at = ?, updated_at = ?
		WHERE order_no = ? AND status = ?
	`
	now := time.Now()
	result, err := tx.Exec(query,
		model.PaymentOrderStatusPaid,
		wechatOrderID,
		now,
		now,
		orderNo,
		model.PaymentOrderStatusPending,
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// GetPaymentOrderByOrderNoTx retrieves a payment order by order number within a transaction.
func (r *PaymentRepository) GetPaymentOrderByOrderNoTx(tx database.Tx, orderNo string) (*model.PaymentOrder, error) {
	query := `
		SELECT id, user_id, order_no, amount, status, pay_result, wechat_order_id, paid_at, created_at, updated_at
		FROM payment_orders
		WHERE order_no = ?
	`
	order := &model.PaymentOrder{}
	err := tx.QueryRow(query, orderNo).Scan(
		&order.ID,
		&order.UserID,
		&order.OrderNo,
		&order.Amount,
		&order.Status,
		&order.PayResult,
		&order.WechatOrderID,
		&order.PaidAt,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return order, nil
}

// ListPaymentOrdersByUserID retrieves payment orders for a user
func (r *PaymentRepository) ListPaymentOrdersByUserID(userID int64, query *model.PaymentOrderQuery) ([]*model.PaymentOrder, int, error) {
	conditions := []string{"user_id = ?"}
	args := []interface{}{userID}

	if query.Status > 0 {
		conditions = append(conditions, "status = ?")
		args = append(args, query.Status)
	}

	whereClause := strings.Join(conditions, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM payment_orders WHERE %s", whereClause)
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	selectQuery := fmt.Sprintf(`
		SELECT id, user_id, order_no, amount, status, pay_result, wechat_order_id, paid_at, created_at, updated_at
		FROM payment_orders
		WHERE %s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, query.PageSize, offset)
	rows, err := r.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []*model.PaymentOrder
	for rows.Next() {
		order := &model.PaymentOrder{}
		if err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.OrderNo,
			&order.Amount,
			&order.Status,
			&order.PayResult,
			&order.WechatOrderID,
			&order.PaidAt,
			&order.CreatedAt,
			&order.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		orders = append(orders, order)
	}
	return orders, total, nil
}
