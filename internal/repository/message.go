package repository

import (
	"database/sql"
	"time"

	"github.com/tans/miao/internal/model"
)

type MessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// CreateMessage 创建消息
func (r *MessageRepository) CreateMessage(msg *model.MessageCreate) error {
	query := `
		INSERT INTO messages (user_id, type, title, content, related_id, is_read, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?)
	`
	_, err := r.db.Exec(query, msg.UserID, msg.Type, msg.Title, msg.Content, msg.RelatedID, time.Now())
	return err
}

// GetMessages 获取消息列表（分页）
func (r *MessageRepository) GetMessages(userID int64, page, limit int) ([]*model.Message, int, error) {
	offset := (page - 1) * limit

	// 获取总数
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE user_id = ?`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 获取消息列表
	query := `
		SELECT id, user_id, type, title, content, related_id, is_read, created_at
		FROM messages
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var messages []*model.Message
	for rows.Next() {
		msg := &model.Message{}
		err := rows.Scan(&msg.ID, &msg.UserID, &msg.Type, &msg.Title, &msg.Content, &msg.RelatedID, &msg.IsRead, &msg.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		messages = append(messages, msg)
	}

	return messages, total, nil
}

// GetUnreadCount 获取未读消息数
func (r *MessageRepository) GetUnreadCount(userID int64) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE user_id = ? AND is_read = 0`, userID).Scan(&count)
	return count, err
}

// MarkAsRead 标记消息已读
func (r *MessageRepository) MarkAsRead(messageID, userID int64) error {
	query := `UPDATE messages SET is_read = 1 WHERE id = ? AND user_id = ?`
	_, err := r.db.Exec(query, messageID, userID)
	return err
}

// MarkAllAsRead 标记全部已读
func (r *MessageRepository) MarkAllAsRead(userID int64) error {
	query := `UPDATE messages SET is_read = 1 WHERE user_id = ? AND is_read = 0`
	_, err := r.db.Exec(query, userID)
	return err
}

// DeleteMessage 删除消息
func (r *MessageRepository) DeleteMessage(messageID, userID int64) error {
	query := `DELETE FROM messages WHERE id = ? AND user_id = ?`
	_, err := r.db.Exec(query, messageID, userID)
	return err
}
