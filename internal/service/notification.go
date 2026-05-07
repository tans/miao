package service

import (
	"fmt"
	"github.com/tans/miao/internal/database"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

var generatedSubmissionNamePattern = regexp.MustCompile(`^(tmp|wxfile|mmexport|capture|image|video|upload)[-_]?[0-9a-z_-]{8,}$`)

type NotificationService struct {
	notificationRepo *repository.NotificationRepository
}

func NewNotificationService(db database.DB) *NotificationService {
	return &NotificationService{
		notificationRepo: repository.NewNotificationRepository(db),
	}
}

func normalizeSubmissionName(raw, fallback string) string {
	name := strings.TrimSpace(raw)
	for _, prefix := range []string{"视频稿件：", "视频稿件:", "稿件：", "稿件:"} {
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimSpace(strings.TrimPrefix(name, prefix))
			break
		}
	}
	if name == "" {
		name = strings.TrimSpace(fallback)
	}
	if ext := filepath.Ext(name); ext != "" {
		name = strings.TrimSpace(strings.TrimSuffix(name, ext))
	}
	if looksLikeGeneratedSubmissionName(name) {
		name = strings.TrimSpace(fallback)
	}
	return name
}

func looksLikeGeneratedSubmissionName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return false
	}
	return generatedSubmissionNamePattern.MatchString(normalized)
}

func normalizeReviewComment(comment string) string {
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return ""
	}

	for _, prefix := range []string{"举报：", "举报:", "原因：", "原因:"} {
		if strings.HasPrefix(comment, prefix) {
			remainder := strings.TrimSpace(strings.TrimPrefix(comment, prefix))
			if remainder == "" {
				return ""
			}
			return remainder
		}
	}

	return comment
}

// notify is a convenience helper
func (s *NotificationService) notify(userID int64, notifType model.NotificationType, title, content string, relatedID *int64) error {
	if s.notificationRepo == nil {
		return fmt.Errorf("notification repository not initialized")
	}
	var rid *uint
	if relatedID != nil {
		value := uint(*relatedID)
		rid = &value
	}
	return s.notificationRepo.CreateNotification(&model.Notification{
		UserID:    uint(userID),
		Type:      notifType,
		Title:     title,
		Content:   content,
		RelatedID: rid,
	})
}

// NotifyTaskCreated 通知商家任务已发布
func (s *NotificationService) NotifyTaskCreated(userID int64, taskID int64, taskTitle string) error {
	title := "任务已发布"
	content := fmt.Sprintf("任务《%s》已创建并上架，可开始征稿", taskTitle)
	return s.notify(userID, model.NotificationTypeTaskCreated, title, content, &taskID)
}

// NotifyTaskReviewed 通知任务审核结果
func (s *NotificationService) NotifyTaskReviewed(userID int64, taskID int64, taskTitle string, approved bool, reason string) error {
	var title, content string
	notifType := model.NotificationTypeTaskReviewRejected
	if approved {
		title = "任务审核通过"
		content = fmt.Sprintf("您的任务《%s》已通过审核，现已上架", taskTitle)
		notifType = model.NotificationTypeTaskReviewPassed
	} else {
		title = "任务审核未通过"
		content = fmt.Sprintf("您的任务《%s》未通过审核", taskTitle)
		if reason != "" {
			content += fmt.Sprintf("，原因：%s", reason)
		}
	}
	return s.notify(userID, notifType, title, content, &taskID)
}

// NotifyTaskClaimed 通知任务被认领
func (s *NotificationService) NotifyTaskClaimed(userID int64, taskID int64, taskTitle string, creatorName string) error {
	title := "收到创作者的报名"
	content := fmt.Sprintf("【%s】报名了您发布的《%s》任务。", creatorName, taskTitle)
	return s.notify(userID, model.NotificationTypeTaskClaimed, title, content, &taskID)
}

// NotifyClaimCreated 通知创作者认领成功
func (s *NotificationService) NotifyClaimCreated(userID int64, taskID int64, taskTitle string) error {
	title := "报名成功"
	content := fmt.Sprintf("你已报名《%s》任务，24小时内完成投稿。", taskTitle)
	return s.notify(userID, model.NotificationTypeClaimCreated, title, content, &taskID)
}

// NotifySubmissionSubmitted 通知投稿已提交（claim submit）
func (s *NotificationService) NotifySubmissionSubmitted(userID int64, taskID int64, taskTitle string) error {
	title := "收到新的稿件"
	content := fmt.Sprintf("你创建的任务《%s》收到了一条新的稿件，请及时去审核。", taskTitle)
	return s.notify(userID, model.NotificationTypeSubmissionReceived, title, content, &taskID)
}

// NotifySubmissionConfirmed 通知创作者投稿已提交
func (s *NotificationService) NotifySubmissionConfirmed(userID int64, taskID int64, taskTitle string) error {
	title := "作品提交成功"
	content := fmt.Sprintf("你成功上传《%s》的稿件，商家会在48小时内审核，奖励将在审核后发放。", taskTitle)
	return s.notify(userID, model.NotificationTypeSubmissionSubmitted, title, content, &taskID)
}

// NotifyReviewResult 通知验收结果
func (s *NotificationService) NotifyReviewResult(userID int64, taskID int64, taskTitle string, workTitle string, approved bool, comment string) error {
	var title, content string
	notifType := model.NotificationTypeReviewRejected
	_ = workTitle
	submissionLabel := "你的投稿"
	if strings.TrimSpace(taskTitle) != "" {
		submissionLabel = fmt.Sprintf("你提交到《%s》任务的投稿", strings.TrimSpace(taskTitle))
	}
	comment = normalizeReviewComment(comment)
	if approved {
		title = "作品审核通过"
		content = fmt.Sprintf("%s已通过审核，任务奖金已发放至你的钱包，请查收。", submissionLabel)
		notifType = model.NotificationTypeReviewPassed
	} else {
		title = "作品审核未通过"
		content = fmt.Sprintf("%s未通过审核", submissionLabel)
		if comment != "" {
			content += fmt.Sprintf("，原因：%s", comment)
		}
		content += "。"
	}
	return s.notify(userID, notifType, title, content, &taskID)
}

// NotifyTaskCancelled 通知任务取消
func (s *NotificationService) NotifyTaskCancelled(userID int64, taskID int64, taskTitle string) error {
	title := "任务已取消"
	content := fmt.Sprintf("任务《%s》已被商家取消，相关保证金会按规则退回", taskTitle)
	return s.notify(userID, model.NotificationTypeTaskCancelled, title, content, &taskID)
}

// NotifyAppealCreated 通知申诉提交成功
func (s *NotificationService) NotifyAppealCreated(userID int64, appealID int64) error {
	title := "申诉已提交"
	content := "你的申诉已进入处理队列，处理结果会通过消息中心通知"
	return s.notify(userID, model.NotificationTypeAppealCreated, title, content, &appealID)
}

// NotifyAppealHandled 通知申诉处理结果
func (s *NotificationService) NotifyAppealHandled(userID int64, appealID int64, result string) error {
	title := "申诉已处理"
	content := fmt.Sprintf("您的申诉已处理，处理结果：%s", result)
	return s.notify(userID, model.NotificationTypeAppealHandled, title, content, &appealID)
}

// NotifySystem 发送系统通知
func (s *NotificationService) NotifySystem(userID int64, title, content string) error {
	return s.notify(userID, model.NotificationTypeSystem, title, content, nil)
}

// CreateNotification creates a new notification
func (s *NotificationService) CreateNotification(userID uint, notifType model.NotificationType, title, content string) error {
	if s.notificationRepo == nil {
		return fmt.Errorf("notification repository not initialized")
	}
	return s.notificationRepo.CreateNotification(&model.Notification{
		UserID:  userID,
		Type:    notifType,
		Title:   title,
		Content: content,
	})
}

// CreateNotificationWithRelatedID creates a notification with a related ID
func (s *NotificationService) CreateNotificationWithRelatedID(userID uint, notifType model.NotificationType, title, content string, relatedID *uint) error {
	if s.notificationRepo == nil {
		return fmt.Errorf("notification repository not initialized")
	}
	return s.notificationRepo.CreateNotification(&model.Notification{
		UserID:    userID,
		Type:      notifType,
		Title:     title,
		Content:   content,
		RelatedID: relatedID,
	})
}

// GetNotifications retrieves notifications for a user
func (s *NotificationService) GetNotifications(userID uint, page, limit int) ([]model.Notification, int64, error) {
	if s.notificationRepo == nil {
		return nil, 0, fmt.Errorf("notification repository not initialized")
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	notifications, total, err := s.notificationRepo.GetNotifications(userID, "", nil, page, limit)
	if err != nil {
		return nil, 0, err
	}

	var result []model.Notification
	for _, n := range notifications {
		result = append(result, *n)
	}
	return result, total, nil
}

// GetNotificationsByType retrieves notifications by type
func (s *NotificationService) GetNotificationsByType(userID uint, notifType string, page, limit int) ([]model.Notification, int64, error) {
	if s.notificationRepo == nil {
		return nil, 0, fmt.Errorf("notification repository not initialized")
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	notifications, total, err := s.notificationRepo.GetNotifications(userID, notifType, nil, page, limit)
	if err != nil {
		return nil, 0, err
	}

	var result []model.Notification
	for _, n := range notifications {
		result = append(result, *n)
	}
	return result, total, nil
}

// MarkAsRead marks a notification as read
func (s *NotificationService) MarkAsRead(id, userID uint) error {
	if s.notificationRepo == nil {
		return fmt.Errorf("notification repository not initialized")
	}
	return s.notificationRepo.MarkAsRead(id, userID)
}

// GetUnreadCount returns the count of unread notifications
func (s *NotificationService) GetUnreadCount(userID uint) (int64, error) {
	if s.notificationRepo == nil {
		return 0, fmt.Errorf("notification repository not initialized")
	}
	return s.notificationRepo.GetUnreadCount(userID)
}

// MarkAllAsRead marks all notifications as read
func (s *NotificationService) MarkAllAsRead(userID uint) error {
	if s.notificationRepo == nil {
		return fmt.Errorf("notification repository not initialized")
	}
	return s.notificationRepo.MarkAllAsRead(userID)
}

// ListNotifications retrieves notifications with optional filters
func (s *NotificationService) ListNotifications(userID uint, notifType string, isRead *bool, page, limit int) ([]model.Notification, int64, error) {
	if s.notificationRepo == nil {
		return nil, 0, fmt.Errorf("notification repository not initialized")
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	notifications, total, err := s.notificationRepo.GetNotifications(userID, notifType, isRead, page, limit)
	if err != nil {
		return nil, 0, err
	}

	result := make([]model.Notification, 0, len(notifications))
	for _, n := range notifications {
		result = append(result, *n)
	}
	return result, total, nil
}

// NotifyTaskStatusChanged 通知任务状态变更
func (s *NotificationService) NotifyTaskStatusChanged(userID uint, taskID uint, taskTitle string, status string) error {
	title := "任务状态变更"
	content := taskTitle + " 状态已更新为: " + status
	return s.CreateNotificationWithRelatedID(userID, model.NotificationTypeTaskStatus, title, content, &taskID)
}

// NotifyNewSubmission 通知新提交（claim）
func (s *NotificationService) NotifyNewSubmission(userID uint, taskID uint, taskTitle string, creatorName string) error {
	title := "收到新的稿件"
	content := "你创建的任务《" + taskTitle + "》收到了一条新的稿件，请及时去审核。"
	return s.CreateNotificationWithRelatedID(userID, model.NotificationTypeSubmissionReceived, title, content, &taskID)
}

// NotifyClaimApproved 通知认领通过
func (s *NotificationService) NotifyClaimApproved(userID uint, claimID uint, taskTitle string) error {
	title := "报名成功"
	content := "你已报名《" + taskTitle + "》任务，24小时内完成投稿。"
	return s.CreateNotificationWithRelatedID(userID, model.NotificationTypeClaimCreated, title, content, &claimID)
}

// NotifyIncomeReceived 通知收益到账
func (s *NotificationService) NotifyIncomeReceived(userID uint, amount float64, taskTitle string) error {
	title := "收益到账"
	content := fmt.Sprintf("您完成任务《%s》获得收益 ¥%.2f", taskTitle, amount)
	return s.CreateNotification(userID, model.NotificationTypeReviewPassed, title, content)
}
