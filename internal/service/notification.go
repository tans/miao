package service

import (
	"fmt"
	"github.com/tans/miao/internal/database"

	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

type NotificationService struct {
	notificationRepo *repository.NotificationRepository
}

func NewNotificationService(db database.DB) *NotificationService {
	return &NotificationService{
		notificationRepo: repository.NewNotificationRepository(db),
	}
}

// notify is a convenience helper
func (s *NotificationService) notify(userID int64, notifType model.NotificationType, title, content string, relatedID *int64) error {
	if s.notificationRepo == nil {
		return fmt.Errorf("notification repository not initialized")
	}
	var rid uint
	if relatedID != nil {
		rid = uint(*relatedID)
	}
	return s.notificationRepo.CreateNotification(&model.Notification{
		UserID:    uint(userID),
		Type:      notifType,
		Title:     title,
		Content:   content,
		RelatedID: &rid,
	})
}

// NotifyTaskReviewed 通知任务审核结果
func (s *NotificationService) NotifyTaskReviewed(userID int64, taskID int64, taskTitle string, approved bool, reason string) error {
	var title, content string
	if approved {
		title = "任务审核通过"
		content = fmt.Sprintf("您的任务《%s》已通过审核，现已上架", taskTitle)
	} else {
		title = "任务审核未通过"
		content = fmt.Sprintf("您的任务《%s》未通过审核", taskTitle)
		if reason != "" {
			content += fmt.Sprintf("，原因：%s", reason)
		}
	}
	return s.notify(userID, model.NotificationTypeTaskStatus, title, content, &taskID)
}

// NotifyTaskClaimed 通知任务被认领
func (s *NotificationService) NotifyTaskClaimed(userID int64, taskID int64, taskTitle string, creatorName string) error {
	title := "任务被认领"
	content := fmt.Sprintf("创作者 %s 认领了您的任务《%s》", creatorName, taskTitle)
	return s.notify(userID, model.NotificationTypeClaimApproved, title, content, &taskID)
}

// NotifySubmissionSubmitted 通知投稿已提交（claim submit）
func (s *NotificationService) NotifySubmissionSubmitted(userID int64, taskID int64, taskTitle string, creatorName string) error {
	title := "收到新提交"
	content := fmt.Sprintf("创作者 %s 提交了任务《%s》的作品，请及时验收", creatorName, taskTitle)
	return s.notify(userID, model.NotificationTypeClaimApproved, title, content, &taskID)
}

// NotifyReviewResult 通知验收结果
func (s *NotificationService) NotifyReviewResult(userID int64, claimID int64, taskTitle string, approved bool, comment string) error {
	var title, content string
	if approved {
		title = "作品验收通过"
		content = fmt.Sprintf("您提交的任务《%s》作品已通过验收，奖励已发放", taskTitle)
	} else {
		title = "作品验收未通过"
		content = fmt.Sprintf("您提交的任务《%s》作品未通过验收", taskTitle)
		if comment != "" {
			content += fmt.Sprintf("，原因：%s", comment)
		}
	}
	return s.notify(userID, model.NotificationTypeIncomeReceived, title, content, &claimID)
}

// NotifyAppealHandled 通知申诉处理结果
func (s *NotificationService) NotifyAppealHandled(userID int64, appealID int64, result string) error {
	title := "申诉已处理"
	content := fmt.Sprintf("您的申诉已处理，处理结果：%s", result)
	return s.notify(userID, model.NotificationTypeTaskStatus, title, content, &appealID)
}

// NotifySystem 发送系统通知
func (s *NotificationService) NotifySystem(userID int64, title, content string) error {
	return s.notify(userID, model.NotificationTypeTaskStatus, title, content, nil)
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

// NotifyTaskStatusChanged 通知任务状态变更
func (s *NotificationService) NotifyTaskStatusChanged(userID uint, taskID uint, taskTitle string, status string) error {
	title := "任务状态变更"
	content := taskTitle + " 状态已更新为: " + status
	return s.CreateNotificationWithRelatedID(userID, model.NotificationTypeTaskStatus, title, content, &taskID)
}

// NotifyNewSubmission 通知新提交（claim）
func (s *NotificationService) NotifyNewSubmission(userID uint, taskID uint, taskTitle string, creatorName string) error {
	title := "新提交通知"
	content := "创作者 " + creatorName + " 提交了任务《" + taskTitle + "》的作品"
	return s.CreateNotificationWithRelatedID(userID, model.NotificationTypeClaimApproved, title, content, &taskID)
}

// NotifyClaimApproved 通知认领通过
func (s *NotificationService) NotifyClaimApproved(userID uint, claimID uint, taskTitle string) error {
	title := "认领通过"
	content := "您已成功认领任务《" + taskTitle + "》，请按时完成"
	return s.CreateNotificationWithRelatedID(userID, model.NotificationTypeClaimApproved, title, content, &claimID)
}

// NotifyIncomeReceived 通知收益到账
func (s *NotificationService) NotifyIncomeReceived(userID uint, amount float64, taskTitle string) error {
	title := "收益到账"
	content := fmt.Sprintf("您完成任务《%s》获得收益 ¥%.2f", taskTitle, amount)
	return s.CreateNotification(userID, model.NotificationTypeIncomeReceived, title, content)
}
