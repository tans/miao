package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	ocrclient "github.com/alibabacloud-go/ocr-api-20210707/v3/client"
	teautil "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/repository"
)

var (
	ErrOCRNotConfigured    = errors.New("OCR服务未配置")
	ErrOCRDisabled         = errors.New("OCR服务已禁用")
	ErrOCRImageURLRequired = errors.New("营业执照图片地址不能为空")
	ErrOCRNoUsefulData     = errors.New("未识别到营业执照信息")
)

type MerchantAuthOCRService struct {
	cfg *config.Config
	db  database.DB
}

type MerchantAuthOCRResult struct {
	CompanyName string `json:"company_name"`
	CreditCode  string `json:"credit_code"`
	LegalPerson string `json:"legal_person"`
}

type merchantAuthOCRPayload struct {
	Data struct {
		CompanyName string `json:"companyName"`
		CreditCode  string `json:"creditCode"`
		LegalPerson string `json:"legalPerson"`
	} `json:"data"`
	CompanyName string `json:"companyName"`
	CreditCode  string `json:"creditCode"`
	LegalPerson string `json:"legalPerson"`
}

func NewMerchantAuthOCRService(cfg *config.Config, db database.DB) *MerchantAuthOCRService {
	if cfg == nil {
		cfg = config.Load()
	}
	return &MerchantAuthOCRService{cfg: cfg, db: db}
}

func (s *MerchantAuthOCRService) resolveOCRConfig() config.OCRConfig {
	resolved := config.OCRConfig{}
	if s != nil && s.cfg != nil {
		resolved = s.cfg.OCR
	}

	if s != nil && s.db != nil {
		if settings, err := repository.NewAdminRepository(s.db).GetAISettings(); err == nil && settings != nil {
			if v := strings.TrimSpace(settings.OCRAccessKeyID); v != "" {
				resolved.AccessKeyID = v
			}
			if v := strings.TrimSpace(settings.OCRAccessKeySecret); v != "" {
				resolved.AccessKeySecret = v
			}
			if v := strings.TrimSpace(settings.OCREndpoint); v != "" {
				resolved.Endpoint = v
			}
			if v := strings.TrimSpace(settings.OCRSecurityToken); v != "" {
				resolved.SecurityToken = v
			}
		}
	}

	if resolved.Endpoint == "" {
		resolved.Endpoint = "ocr-api.cn-hangzhou.aliyuncs.com"
	}
	return resolved
}

func parseMerchantAuthOCRResult(raw string) (*MerchantAuthOCRResult, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrOCRNoUsefulData
	}

	var payload merchantAuthOCRPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}

	result := &MerchantAuthOCRResult{
		CompanyName: strings.TrimSpace(firstNonEmpty(payload.Data.CompanyName, payload.CompanyName)),
		CreditCode:  strings.ToUpper(strings.TrimSpace(firstNonEmpty(payload.Data.CreditCode, payload.CreditCode))),
		LegalPerson: strings.TrimSpace(firstNonEmpty(payload.Data.LegalPerson, payload.LegalPerson)),
	}

	if result.CompanyName == "" && result.CreditCode == "" && result.LegalPerson == "" {
		return nil, ErrOCRNoUsefulData
	}

	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *MerchantAuthOCRService) RecognizeBusinessLicense(ctx context.Context, imageURL string) (*MerchantAuthOCRResult, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return nil, ErrOCRImageURLRequired
	}

	cfg := s.resolveOCRConfig()
	if !cfg.Enabled {
		return nil, ErrOCRDisabled
	}
	accessKeyID := strings.TrimSpace(cfg.AccessKeyID)
	accessKeySecret := strings.TrimSpace(cfg.AccessKeySecret)
	if accessKeyID == "" || accessKeySecret == "" {
		return nil, ErrOCRNotConfigured
	}

	openapiCfg := &openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
		Endpoint:        tea.String(cfg.Endpoint),
		Protocol:        tea.String("HTTPS"),
	}
	if token := strings.TrimSpace(cfg.SecurityToken); token != "" {
		openapiCfg.SecurityToken = tea.String(token)
	}

	client, err := ocrclient.NewClient(openapiCfg)
	if err != nil {
		return nil, fmt.Errorf("初始化阿里云 OCR 客户端失败: %w", err)
	}

	req := &ocrclient.RecognizeBusinessLicenseRequest{}
	req.SetUrl(imageURL)

	resp, err := client.RecognizeBusinessLicenseWithOptions(req, &teautil.RuntimeOptions{})
	if err != nil {
		return nil, fmt.Errorf("调用阿里云 OCR 失败: %w", err)
	}
	if resp == nil || resp.Body == nil {
		return nil, errors.New("OCR 响应为空")
	}
	if code := strings.TrimSpace(tea.StringValue(resp.Body.Code)); code != "" && code != "200" {
		msg := strings.TrimSpace(tea.StringValue(resp.Body.Message))
		if msg == "" {
			msg = "OCR 识别失败"
		}
		return nil, fmt.Errorf("%s: %s", msg, code)
	}
	if resp.Body.Data == nil {
		return nil, ErrOCRNoUsefulData
	}

	result, err := parseMerchantAuthOCRResult(tea.StringValue(resp.Body.Data))
	if err != nil {
		return nil, err
	}
	return result, nil
}
