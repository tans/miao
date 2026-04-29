package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
)

// AI Service for generating task descriptions

type AIWriteRequest struct {
	Title      string   `json:"title"`
	Industries []string `json:"industries"`
	Styles     []string `json:"styles"`
}

type AIWriteResponse struct {
	Description string `json:"description"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}

type aiService struct {
	apiKey   string
	endpoint string
	model    string
	client   *http.Client
	db       database.DB
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type responsesAPIResponse struct {
	OutputText string `json:"output_text"`
	Output     []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

var aiServiceInstance *aiService

func GetAIService() *aiService {
	if aiServiceInstance != nil {
		return aiServiceInstance
	}

	cfg := config.Load()
	aiServiceInstance = &aiService{
		apiKey:   os.Getenv("OPENAI_API_KEY"),
		endpoint: os.Getenv("OPENAI_API_ENDPOINT"),
		model:    os.Getenv("OPENAI_MODEL"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	if db, err := database.InitDB(cfg.Database); err == nil {
		aiServiceInstance.db = db
	}

	// Set defaults
	if aiServiceInstance.endpoint == "" {
		aiServiceInstance.endpoint = "https://api.openai.com/v1/responses"
	}
	if aiServiceInstance.model == "" {
		aiServiceInstance.model = "gpt-4.1-mini"
	}

	_ = cfg // avoid unused warning
	return aiServiceInstance
}

// GenerateTaskDescription generates a task description using AI
func (s *aiService) GenerateTaskDescription(req *AIWriteRequest) (*AIWriteResponse, error) {
	s.refreshConfig()
	if s.apiKey == "" {
		return &AIWriteResponse{
			Success: false,
			Error:   "AI服务未配置，请联系管理员设置OPENAI_API_KEY环境变量",
		}, nil
	}

	// Build prompt for task description generation
	industryStr := ""
	if len(req.Industries) > 0 {
		industryStr = joinWithAnd(req.Industries)
	}

	styleStr := ""
	if len(req.Styles) > 0 {
		styleStr = joinWithAnd(req.Styles)
	}

	prompt := fmt.Sprintf("你是一个专业的视频广告文案撰写专家。请根据以下信息，为商家生成一个详细、吸引人的任务描述（用于招募视频创作者）。\n\n任务标题：%s\n行业：%s\n风格：%s\n\n请生成一个10-100字的视频任务描述，要求：\n1. 语言简洁、口语化，适合视频创作者理解\n2. 突出任务要求和亮点\n3. 包含必要的产品/服务信息（假设合理）\n4. 鼓励创作者积极参与\n\n直接输出描述文字，不要加引号或前缀。", req.Title, industryStr, styleStr)

	apiReq := s.buildRequestPayload(prompt)

	jsonData, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", s.endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return &AIWriteResponse{
			Success: false,
			Error:   "AI服务请求失败: " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &AIWriteResponse{
			Success: false,
			Error:   "读取AI响应失败",
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return &AIWriteResponse{
			Success: false,
			Error:   fmt.Sprintf("AI服务返回错误: %d - %s", resp.StatusCode, string(body)),
		}, nil
	}

	description, err := s.extractDescription(body)
	if err != nil {
		return &AIWriteResponse{
			Success: false,
			Error:   "解析AI响应失败",
		}, nil
	}

	if description == "" {
		return &AIWriteResponse{
			Success: false,
			Error:   "AI未返回有效内容",
		}, nil
	}

	return &AIWriteResponse{
		Description: cleanDescription(description),
		Success:     true,
	}, nil
}

func (s *aiService) refreshConfig() {
	if s.db == nil {
		return
	}

	var apiKey, endpoint, model string
	err := s.db.QueryRow(`
		SELECT ai_api_key, ai_api_endpoint, ai_model
		FROM system_settings WHERE id = 1
	`).Scan(&apiKey, &endpoint, &model)
	if err != nil {
		return
	}

	if strings.TrimSpace(apiKey) != "" {
		s.apiKey = apiKey
	}
	if strings.TrimSpace(endpoint) != "" {
		s.endpoint = endpoint
	}
	if strings.TrimSpace(model) != "" {
		s.model = model
	}
}

func (s *aiService) buildRequestPayload(prompt string) map[string]interface{} {
	if strings.Contains(strings.ToLower(s.endpoint), "/responses") {
		return map[string]interface{}{
			"model":             s.model,
			"input":             prompt,
			"instructions":      "你是一个专业的视频广告文案撰写专家。",
			"temperature":       0.8,
			"max_output_tokens": 500,
		}
	}

	return map[string]interface{}{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个专业的视频广告文案撰写专家。"},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.8,
		"max_tokens":  500,
	}
}

func (s *aiService) extractDescription(body []byte) (string, error) {
	var responsesRes responsesAPIResponse
	if err := json.Unmarshal(body, &responsesRes); err == nil {
		if text := cleanDescription(responsesRes.OutputText); text != "" {
			return text, nil
		}
		for _, output := range responsesRes.Output {
			for _, content := range output.Content {
				if content.Type == "output_text" || content.Type == "text" {
					if text := cleanDescription(content.Text); text != "" {
						return text, nil
					}
				}
			}
		}
	}

	var chatRes chatCompletionsResponse
	if err := json.Unmarshal(body, &chatRes); err != nil {
		return "", err
	}
	if len(chatRes.Choices) == 0 {
		return "", nil
	}
	return chatRes.Choices[0].Message.Content, nil
}

func joinWithAnd(items []string) string {
	if len(items) == 0 {
		return ""
	}
	result := items[0]
	for i := 1; i < len(items); i++ {
		if i == len(items)-1 {
			result += "和" + items[i]
		} else {
			result += "、" + items[i]
		}
	}
	return result
}

func cleanDescription(desc string) string {
	// Remove leading/trailing whitespace and newlines
	desc = strings.TrimSpace(desc)

	// Remove quotes if present at start or end
	if len(desc) > 0 && (desc[0] == '"' || desc[0] == '\'') {
		desc = desc[1:]
	}
	if len(desc) > 0 {
		last := len(desc) - 1
		if desc[last] == '"' || desc[last] == '\'' {
			desc = desc[:last]
		}
	}

	return desc
}
