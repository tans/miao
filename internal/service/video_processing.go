package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/storage"
	"net/http"
	"net/url"
	"os"
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
	storageProvider storage.StorageProvider
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
		storageProvider: initVideoStorageProvider(cfg),
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
	sourceURL = s.readableSourceURL(sourceURL)

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

func (s *VideoProcessingService) readableSourceURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || s == nil || s.storageProvider == nil {
		return raw
	}
	if s.cfg != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Storage.Provider), "cos") {
		key := storage.ExtractObjectKey(raw, configuredStorageBucket(s.cfg))
		if storage.IsClaimAssetKey(key) {
			baseHost := strings.TrimSpace(s.cfg.Static.Host)
			if baseHost == "" {
				baseHost = strings.TrimRight(strings.TrimSpace(s.cfg.VideoProcessing.CallbackBaseURL), "/")
			}
			return storage.BuildProxyDownloadURL(baseHost, s.cfg.JWT.Secret, raw, 2*time.Hour)
		}
	}
	signedURL, err := storage.GetDownloadURL(context.Background(), s.storageProvider, configuredStorageBucket(s.cfg), raw, 2*time.Hour)
	if err != nil || signedURL == "" {
		return raw
	}
	return signedURL
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

func initVideoStorageProvider(cfg *config.Config) storage.StorageProvider {
	if cfg == nil {
		return nil
	}

	workDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	if workDir == "" || workDir == "." {
		workDir, _ = os.Getwd()
	}

	factory := storage.NewFactory(cfg.Static.Host, cfg.Static.CDN, workDir)
	var cfgType storage.StorageType
	switch strings.ToLower(strings.TrimSpace(cfg.Storage.Provider)) {
	case "rustfs":
		cfgType = storage.StorageTypeRustFS
	case "s3":
		cfgType = storage.StorageTypeS3
	case "oss":
		cfgType = storage.StorageTypeOSS
	case "cos":
		cfgType = storage.StorageTypeCOS
	default:
		cfgType = storage.StorageTypeLocal
	}

	provider, err := factory.NewProvider(storage.Config{
		Type:  cfgType,
		Local: storage.LocalConfig{},
		RustFS: storage.S3CompatibleConfig{
			Endpoint:          cfg.Storage.RustFS.Endpoint,
			Bucket:            cfg.Storage.RustFS.Bucket,
			AccessKey:         cfg.Storage.RustFS.AccessKey,
			SecretKey:         cfg.Storage.RustFS.SecretKey,
			Region:            cfg.Storage.RustFS.Region,
			UsePathStyle:      true,
			HostnameImmutable: false,
		},
		S3: storage.S3Config{
			Endpoint:        cfg.Storage.S3.Endpoint,
			Bucket:          cfg.Storage.S3.Bucket,
			Region:          cfg.Storage.S3.Region,
			AccessKeyID:     cfg.Storage.S3.AccessKeyID,
			SecretAccessKey: cfg.Storage.S3.SecretAccessKey,
		},
		OSS: storage.OSSConfig{
			Endpoint:    cfg.Storage.OSS.Endpoint,
			Bucket:      cfg.Storage.OSS.Bucket,
			AccessKeyID: cfg.Storage.OSS.AccessKey,
			SecretKey:   cfg.Storage.OSS.SecretKey,
			CDNHost:     cfg.Storage.OSS.CDNHost,
		},
		COS: storage.COSConfig{
			AppID:     cfg.Storage.COS.AppID,
			Bucket:    cfg.Storage.COS.Bucket,
			Region:    cfg.Storage.COS.Region,
			SecretKey: cfg.Storage.COS.SecretKey,
			SecretID:  cfg.Storage.COS.SecretID,
			CDNHost:   cfg.Storage.COS.CDNHost,
		},
	})
	if err != nil {
		return nil
	}
	return provider
}

func configuredStorageBucket(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Storage.Provider)) {
	case "cos":
		if cfg.Storage.COS.AppID != "" && !strings.Contains(cfg.Storage.COS.Bucket, "-") {
			return cfg.Storage.COS.Bucket + "-" + cfg.Storage.COS.AppID
		}
		return cfg.Storage.COS.Bucket
	case "s3":
		return cfg.Storage.S3.Bucket
	case "oss":
		return cfg.Storage.OSS.Bucket
	case "rustfs":
		return cfg.Storage.RustFS.Bucket
	default:
		return ""
	}
}
