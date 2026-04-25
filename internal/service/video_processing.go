package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tans/miao/internal/database"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

type VideoProcessingService struct {
	cfg             *config.Config
	httpClient      *http.Client
	jobRepo         *repository.VideoProcessingRepository
	creatorRepo     *repository.CreatorRepository
	taskRepo        *repository.TaskRepository
	userRepo        *repository.UserRepository
	inspirationSync *ClaimInspirationService
}

func NewVideoProcessingService(db database.DB, cfg *config.Config) *VideoProcessingService {
	if cfg == nil {
		cfg = config.Load()
	}
	return &VideoProcessingService{
		cfg:             cfg,
		httpClient:      &http.Client{Timeout: 20 * time.Second},
		jobRepo:         repository.NewVideoProcessingRepository(db),
		creatorRepo:     repository.NewCreatorRepository(db),
		taskRepo:        repository.NewTaskRepository(db),
		userRepo:        repository.NewUserRepository(db),
		inspirationSync: NewClaimInspirationService(db),
	}
}

func (s *VideoProcessingService) Enabled() bool {
	return s != nil && s.cfg != nil && s.cfg.VideoProcessing.Enabled && strings.TrimSpace(s.cfg.VideoProcessing.ServiceURL) != ""
}

func (s *VideoProcessingService) QueueClaimVideo(claimID int64, material *model.ClaimMaterial) (*model.VideoProcessingJob, error) {
	if material == nil {
		return nil, fmt.Errorf("nil material")
	}
	if material.FileType != "video" {
		return nil, nil
	}

	sourceURL := strings.TrimSpace(material.SourceFilePath)
	if sourceURL == "" {
		sourceURL = strings.TrimSpace(material.FilePath)
	}
	if sourceURL == "" {
		return nil, fmt.Errorf("empty source url")
	}

	job := &model.VideoProcessingJob{
		JobID:             buildVideoJobID(claimID, material),
		MaterialID:        material.ID,
		BizType:           "claim_submission",
		BizID:             claimID,
		SourceURL:         sourceURL,
		Status:            model.VideoProcessStatusPending,
		WatermarkTemplate: s.cfg.VideoProcessing.WatermarkTemplate,
		TargetFormat:      s.cfg.VideoProcessing.TargetFormat,
		TargetResolution:  s.cfg.VideoProcessing.TargetResolution,
	}
	if err := s.jobRepo.Create(job); err != nil {
		return nil, err
	}

	if !s.Enabled() {
		err := fmt.Errorf("video processing service disabled")
		_ = s.jobRepo.UpdateDispatchStatus(job.JobID, model.VideoProcessStatusFailed, err.Error())
		_ = s.jobRepo.UpdateMaterialStatus(material.ID, model.VideoProcessStatusFailed, err.Error())
		return job, err
	}

	reqBody := model.VideoProcessingJobRequest{
		JobID:             job.JobID,
		SourceURL:         job.SourceURL,
		BizType:           job.BizType,
		BizID:             job.BizID,
		WatermarkTemplate: job.WatermarkTemplate,
		TargetFormat:      job.TargetFormat,
		TargetResolution:  job.TargetResolution,
		CallbackURL:       s.callbackURL(),
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return job, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, strings.TrimRight(s.cfg.VideoProcessing.ServiceURL, "/")+"/jobs", bytes.NewReader(body))
	if err != nil {
		return job, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if s.cfg.VideoProcessing.CallbackSecret != "" {
		httpReq.Header.Set("X-Miao-Signature", s.SignBody(body))
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		_ = s.jobRepo.UpdateDispatchStatus(job.JobID, model.VideoProcessStatusFailed, err.Error())
		_ = s.jobRepo.UpdateMaterialStatus(material.ID, model.VideoProcessStatusFailed, err.Error())
		return job, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("video processing create job failed: status %d", resp.StatusCode)
		_ = s.jobRepo.UpdateDispatchStatus(job.JobID, model.VideoProcessStatusFailed, err.Error())
		_ = s.jobRepo.UpdateMaterialStatus(material.ID, model.VideoProcessStatusFailed, err.Error())
		return job, err
	}

	_ = s.jobRepo.UpdateDispatchStatus(job.JobID, model.VideoProcessStatusProcessing, "")
	_ = s.jobRepo.UpdateMaterialStatus(material.ID, model.VideoProcessStatusProcessing, "")
	job.Status = model.VideoProcessStatusProcessing
	return job, nil
}

func (s *VideoProcessingService) HandleCallback(cb *model.VideoProcessingCallback) error {
	if cb == nil {
		return fmt.Errorf("nil callback")
	}
	job, err := s.jobRepo.ApplyCallback(cb.JobID, cb)
	if err != nil {
		return err
	}
	return s.syncClaimAssets(job.BizID)
}

func (s *VideoProcessingService) SignBody(body []byte) string {
	if s == nil || s.cfg == nil || s.cfg.VideoProcessing.CallbackSecret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.VideoProcessing.CallbackSecret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *VideoProcessingService) VerifySignature(body []byte, signature string) bool {
	if s == nil || s.cfg == nil || s.cfg.VideoProcessing.CallbackSecret == "" {
		return true
	}
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return false
	}
	expected := s.SignBody(body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (s *VideoProcessingService) callbackURL() string {
	base := strings.TrimRight(strings.TrimSpace(s.cfg.VideoProcessing.CallbackBaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(s.cfg.Static.Host), "/")
	}
	return base + "/internal/video-processing/callback"
}

func (s *VideoProcessingService) syncClaimAssets(claimID int64) error {
	claim, err := s.creatorRepo.GetClaimByID(claimID)
	if err != nil || claim == nil {
		return err
	}
	task, err := s.taskRepo.GetTaskByID(claim.TaskID)
	if err != nil || task == nil {
		return err
	}
	creator, err := s.userRepo.GetUserByID(claim.CreatorID)
	if err != nil || creator == nil {
		return err
	}
	materials, err := s.creatorRepo.GetClaimMaterials(claimID)
	if err != nil {
		return err
	}
	if claim.Status == model.ClaimStatusApproved {
		_, err = s.inspirationSync.PublishFromClaim(claim, task, creator, materials)
		return err
	}
	if claim.Status >= model.ClaimStatusSubmitted {
		_, err = s.inspirationSync.SaveDraftFromClaim(claim, task, creator, materials)
		return err
	}
	return nil
}

func buildVideoJobID(claimID int64, material *model.ClaimMaterial) string {
	if material != nil {
		sourceURL := strings.TrimSpace(material.SourceFilePath)
		if sourceURL == "" {
			sourceURL = strings.TrimSpace(material.FilePath)
		}
		if jobID := extractClaimSourceJobID(sourceURL, claimID); jobID != "" {
			return jobID
		}
		return fmt.Sprintf("claim-%d-material-%d-%d", claimID, material.ID, time.Now().UnixNano())
	}
	return fmt.Sprintf("claim-%d-%d", claimID, time.Now().UnixNano())
}

func extractClaimSourceJobID(sourceURL string, claimID int64) string {
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return ""
	}

	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return ""
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	claimSegment := strconv.FormatInt(claimID, 10)
	for i := 0; i+2 < len(segments); i++ {
		if segments[i] != "claim-source" || segments[i+1] != claimSegment {
			continue
		}
		base := path.Base(segments[i+2])
		jobID := strings.TrimSuffix(base, filepath.Ext(base))
		if jobID != "" {
			return jobID
		}
	}
	return ""
}
