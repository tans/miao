package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
)

// AI Service for generating task descriptions

type AIWriteRequest struct {
	Title       string   `json:"title"`
	Industries  []string `json:"industries"`
	Styles      []string `json:"styles"`
	Description string   `json:"description"`
}

type AIWriteResponse struct {
	Description string   `json:"description"`
	Industries  []string `json:"industries,omitempty"`
	Styles      []string `json:"styles,omitempty"`
	Success     bool     `json:"success"`
	Error       string   `json:"error,omitempty"`
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

type aiTaskDraft struct {
	Industries  []string `json:"industries"`
	Styles      []string `json:"styles"`
	Description string   `json:"description"`
}

var aiServiceInstance *aiService

func GetAIService() *aiService {
	if aiServiceInstance != nil {
		return aiServiceInstance
	}

	cfg := config.Load()
	aiServiceInstance = &aiService{
		apiKey:   "",
		endpoint: "",
		model:    "",
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
			Error:   "AI服务未配置，请先在管理后台保存模型配置",
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

	descriptionStr := strings.TrimSpace(req.Description)
	if descriptionStr == "" {
		descriptionStr = "（未填写任务描述）"
	}

	allowedIndustries := "餐饮美食、酒店民宿、本地生活、房产家居、家居家电、服饰穿搭、美妆护肤、母婴亲子、数码科技、教育培训、汽车服务、医疗健康、金融理财、企业商务、电商零售、其他行业"
	allowedStyles := "口语化、高级感、接地气、幽默风趣、温馨治愈、时尚潮流"

	prompt := fmt.Sprintf("你是一个专业的视频任务脚本助手。请根据以下信息，生成适合招募创作者的任务内容，并顺带判断最合适的行业和风格。\n\n任务标题：%s\n当前已选行业：%s\n当前已选风格：%s\n当前任务描述：%s\n\n如果当前已选行业/风格为空，请从下面候选中自动选择最合适的；如果不为空，请沿用或微调。\n可选行业：%s\n可选风格：%s\n\n请只输出一个 JSON 对象，不要输出任何额外文字，格式如下：\n{\"industries\":[\"行业1\"],\"styles\":[\"风格1\"],\"description\":\"任务内容\"}\n\n输出要求：\n1. industries 和 styles 只放最合适的 1-3 项。\n2. description 要像任务卡片，3-5 段字段化输出，每段使用“字段名：正文”的格式。\n3. 字段名要短，正文要简洁、干净、可直接展示。\n4. 不要出现“标题”和“定位”字段。\n5. 不要编造明显不相关的信息。", req.Title, industryStr, styleStr, descriptionStr, allowedIndustries, allowedStyles)

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

	draft, err := s.extractTaskDraft(body)
	if err != nil {
		return &AIWriteResponse{
			Success: false,
			Error:   "解析AI响应失败",
		}, nil
	}

	if draft.Description == "" {
		return &AIWriteResponse{
			Success: false,
			Error:   "AI未返回有效内容",
		}, nil
	}

	return &AIWriteResponse{
		Description: cleanDescription(draft.Description),
		Industries:  normalizeStringList(draft.Industries),
		Styles:      normalizeStringList(draft.Styles),
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

	s.apiKey = strings.TrimSpace(apiKey)
	if endpoint = strings.TrimSpace(endpoint); endpoint != "" {
		s.endpoint = endpoint
	} else if s.endpoint == "" {
		s.endpoint = "https://api.openai.com/v1/responses"
	}
	if model = strings.TrimSpace(model); model != "" {
		s.model = model
	} else if s.model == "" {
		s.model = "gpt-4.1-mini"
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

func (s *aiService) extractTaskDraft(body []byte) (*aiTaskDraft, error) {
	var draft aiTaskDraft
	if err := json.Unmarshal(body, &draft); err == nil && draft.Description != "" {
		return &draft, nil
	}

	var responsesRes responsesAPIResponse
	if err := json.Unmarshal(body, &responsesRes); err == nil {
		if text := cleanDescription(responsesRes.OutputText); text != "" {
			return &aiTaskDraft{Description: text}, nil
		}
		for _, output := range responsesRes.Output {
			for _, content := range output.Content {
				if content.Type == "output_text" || content.Type == "text" {
					if text := cleanDescription(content.Text); text != "" {
						if parsed := parseTaskDraft(text); parsed != nil {
							return parsed, nil
						}
						return &aiTaskDraft{Description: text}, nil
					}
				}
			}
		}
	}

	var chatRes chatCompletionsResponse
	if err := json.Unmarshal(body, &chatRes); err != nil {
		return nil, err
	}
	if len(chatRes.Choices) == 0 {
		return nil, nil
	}
	text := cleanDescription(chatRes.Choices[0].Message.Content)
	if parsed := parseTaskDraft(text); parsed != nil {
		return parsed, nil
	}
	return &aiTaskDraft{Description: text}, nil
}

func parseTaskDraft(text string) *aiTaskDraft {
	var draft aiTaskDraft
	if err := json.Unmarshal([]byte(text), &draft); err == nil && draft.Description != "" {
		return &draft
	}
	return nil
}

func normalizeStringList(items []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
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
