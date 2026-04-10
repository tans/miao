package repository

import (
	"database/sql"
	"time"

	"github.com/tans/miao/internal/model"
)

type NotificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// CreateNotification creates a new notification
func (r *NotificationRepository) CreateNotification(notif *model.Notification) error {
	query := `
		INSERT INTO notifications (user_id, type, title, content, related_id, is_read, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	result, err := r.db.Exec(query,
		notif.UserID,
		notif.Type,
		notif.Title,
		notif.Content,
		notif.RelatedID,
		false,
		now,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	notif.ID = uint(id)
	notif.CreatedAt = now
	notif.IsRead = false
	return nil
}

// GetNotifications retrieves notifications with pagination
func (r *NotificationRepository) GetNotifications(userID uint, notifType string, isRead *bool, page, limit int) ([]*model.Notification, int64, error) {
	offset := (page - 1) * limit

	// Build count query
	countQuery := `SELECT COUNT(*) FROM notifications WHERE user_id = ?`
	args := []interface{}{userID}

	if notifType != "" {
		countQuery += ` AND type = ?`
		args = append(args, notifType)
	}
	if isRead != nil {
		countQuery += ` AND is_read = ?`
		args = append(args, *isRead)
	}

	var total int64
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Build select query
	selectQuery := `
		SELECT id, user_id, type, title, content, related_id, is_read, created_at
		FROM notifications
		WHERE user_id = ?`
	selectArgs := []interface{}{userID}

	if notifType != "" {
		selectQuery += ` AND type = ?`
		selectArgs = append(selectArgs, notifType)
	}
	if isRead != nil {
		selectQuery += ` AND is_read = ?`
		selectArgs = append(selectArgs, *isRead)
	}
	selectQuery += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	selectArgs = append(selectArgs, limit, offset)

	rows, err := r.db.Query(selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var notifications []*model.Notification
	for rows.Next() {
		notif := &model.Notification{}
		var relatedID sql.NullInt64
		err := rows.Scan(
			&notif.ID,
			&notif.UserID,
			&notif.Type,
			&notif.Title,
			&notif.Content,
			&relatedID,
			&notif.IsRead,
			&notif.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		if relatedID.Valid {
		 rid := uint(relatedID.Int64)
			notif.RelatedID = &rid
		}
		notifications = append(notifications, notif)
	}

	return notifications, total, rows.Err()
}

// GetUnreadCount returns the count of unread notifications
func (r *NotificationRepository) GetUnreadCount(userID uint) (int64, error) {
	var count int64
	err := r.db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = false`, userID).Scan(&count)
	return count, err
}

// MarkAsRead marks a notification as read
func (r *NotificationRepository) MarkAsRead(id, userID uint) error {
	query := `UPDATE notifications SET is_read = true WHERE id = ? AND user_id = ?`
	_, err := r.db.Exec(query, id, userID)
	return err
}

// MarkAllAsRead marks all notifications as read for a user
func (r *NotificationRepository) MarkAllAsRead(userID uint) error {
	query := `UPDATE notifications SET is_read = true WHERE user_id = ? AND is_read = false`
	_, err := r.db.Exec(query, userID)
	return err
}

// DeleteNotification deletes a notification
func (r *NotificationRepository) DeleteNotification(id, userID uint) error {
	query := `DELETE FROM notifications WHERE id = ? AND user_id = ?`
	_, err := r.db.Exec(query, id, userID)
	return err
}

// GetNotificationByID retrieves a notification by ID
func (r *NotificationRepository) GetNotificationByID(id uint) (*model.Notification, error) {
	query := `
		SELECT id, user_id, type, title, content, related_id, is_read, created_at
		FROM notifications
		WHERE id = ?
	`
	notif := &model.Notification{}
	var relatedID sql.NullInt64
	err := r.db.QueryRow(query, id).Scan(
		&notif.ID,
		&notif.UserID,
		&notif.Type,
		&notif.Title,
		&notif.Content,
		&relatedID,
		&notif.IsRead,
		&notif.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if relatedID.Valid {
		 rid := uint(relatedID.Int64)
		notif.RelatedID = &rid
	}
	return notif, nil
}
