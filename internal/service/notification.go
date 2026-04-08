package service

import (
	"database/sql"
	"fmt"

	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

type NotificationService struct {
	messageRepo *repository.MessageRepository
}

func NewNotificationService(db *sql.DB) *NotificationService {
	return &NotificationService{
		messageRepo: repository.NewMessageRepository(db),
	}
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

	return s.messageRepo.CreateMessage(&model.MessageCreate{
		UserID:    userID,
		Type:      model.MessageTypeTaskReview,
		Title:     title,
		Content:   content,
		RelatedID: &taskID,
	})
}

// NotifyTaskClaimed 通知任务被认领
func (s *NotificationService) NotifyTaskClaimed(userID int64, taskID int64, taskTitle string, creatorName string) error {
	title := "任务被认领"
	content := fmt.Sprintf("创作者 %s 认领了您的任务《%s》", creatorName, taskTitle)

	return s.messageRepo.CreateMessage(&model.MessageCreate{
		UserID:    userID,
		Type:      model.MessageTypeClaimStatus,
		Title:     title,
		Content:   content,
		RelatedID: &taskID,
	})
}

// NotifySubmissionSubmitted 通知投稿已提交
func (s *NotificationService) NotifySubmissionSubmitted(userID int64, taskID int64, taskTitle string, creatorName string) error {
	title := "收到新投稿"
	content := fmt.Sprintf("创作者 %s 提交了任务《%s》的投稿，请及时验收", creatorName, taskTitle)

	return s.messageRepo.CreateMessage(&model.MessageCreate{
		UserID:    userID,
		Type:      model.MessageTypeClaimStatus,
		Title:     title,
		Content:   content,
		RelatedID: &taskID,
	})
}

// NotifyReviewResult 通知验收结果
func (s *NotificationService) NotifyReviewResult(userID int64, claimID int64, taskTitle string, approved bool, comment string) error {
	var title, content string
	if approved {
		title = "投稿验收通过"
		content = fmt.Sprintf("您提交的任务《%s》投稿已通过验收，奖励已发放", taskTitle)
	} else {
		title = "投稿验收未通过"
		content = fmt.Sprintf("您提交的任务《%s》投稿未通过验收", taskTitle)
		if comment != "" {
			content += fmt.Sprintf("，原因：%s", comment)
		}
	}

	return s.messageRepo.CreateMessage(&model.MessageCreate{
		UserID:    userID,
		Type:      model.MessageTypeReviewResult,
		Title:     title,
		Content:   content,
		RelatedID: &claimID,
	})
}

// NotifyAppealHandled 通知申诉处理结果
func (s *NotificationService) NotifyAppealHandled(userID int64, appealID int64, result string) error {
	title := "申诉已处理"
	content := fmt.Sprintf("您的申诉已处理，处理结果：%s", result)

	return s.messageRepo.CreateMessage(&model.MessageCreate{
		UserID:    userID,
		Type:      model.MessageTypeAppealHandled,
		Title:     title,
		Content:   content,
		RelatedID: &appealID,
	})
}

// NotifySystem 发送系统通知
func (s *NotificationService) NotifySystem(userID int64, title, content string) error {
	return s.messageRepo.CreateMessage(&model.MessageCreate{
		UserID:  userID,
		Type:    model.MessageTypeSystem,
		Title:   title,
		Content: content,
	})
}
