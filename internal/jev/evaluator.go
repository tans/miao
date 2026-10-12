package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// UpstreamCommit fixes the choice transport used by json-render's evaluator.
const UpstreamCommit = "fc2a696a50a30cb30c878ab1eb65e102487eea0f"

const (
	ProviderTypesafe         = "typesafe"
	ProviderOpenAICompatible = "openai-compatible"
)

// OfficialEndpoint is the first-party Typesafe API served at api.typesafe.ai.
const OfficialEndpoint = "https://api.typesafe.ai/v1/systemone"
const OfficialDefaultModel = "jev-latest"
const DefaultModel = OfficialDefaultModel

// EndpointFor is kept as a compatibility helper for the admin API.
func EndpointFor(provider string) string {
	if provider == ProviderOpenAICompatible {
		return ""
	}
	return OfficialEndpoint
}

type Question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

type Answer struct {
	Choice     string
	Confidence float64
	HasConf    bool
}

type Evaluation struct {
	Answers      map[string]Answer
	InputTokens  int
	OutputTokens int
	InputKnown   bool
	OutputKnown  bool
	Status       int
}

type Evaluator struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string
	URL      string
	Client   *http.Client
	Timeout  time.Duration
}

// Evaluate is stateless. The caller supplies its persisted observation anew
// after each tool result; no model-owned continuation transcript is required.
func (e Evaluator) Evaluate(ctx context.Context, state map[string]any, questions map[string]Question) (Evaluation, error) {
	result := Evaluation{}
	provider := e.Provider
	if provider == "" {
		provider = ProviderTypesafe
	}
	if provider != ProviderTypesafe && provider != ProviderOpenAICompatible {
		return result, errors.New("unsupported Jev provider")
	}
	if strings.TrimSpace(e.APIKey) == "" || strings.TrimSpace(e.Model) == "" {
		return result, errors.New("Jev requires an API key and model")
	}
	if len(questions) == 0 {
		return result, errors.New("Jev requires at least one choice question")
	}
	for name, question := range questions {
		if name == "" || question.Type != "choice" || len(question.Criteria) == 0 {
			return result, errors.New("Jev question must offer nonempty choice criteria")
		}
	}
	timeout := e.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	if timeout < time.Millisecond || timeout > time.Duration(math.MaxInt32)*time.Millisecond {
		return result, errors.New("Jev timeout must be between 1 and 2147483647 milliseconds")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if state == nil {
		state = map[string]any{}
	}
	body := map[string]any{"state": state, "questions": questions}
	body["model"] = e.Model
	var payload []byte
	var err error
	url := e.URL
	if provider == ProviderOpenAICompatible {
		if strings.TrimSpace(e.BaseURL) == "" {
			return result, errors.New("OpenAI-compatible Jev requires a base URL")
		}
		prompt, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return result, marshalErr
		}
		payload, err = json.Marshal(map[string]any{
			"model":       e.Model,
			"stream":      false,
			"temperature": 0,
			"messages": []map[string]string{
				{"role": "system", "content": "You are a bounded decision engine. Return JSON only, with no markdown. The answer must be an object with an answers field. For every question, return {type:\"choice\", choice:<one key from that question's criteria>, confidence:<number from 0 to 1>}. Never invent question names or choices."},
				{"role": "user", "content": string(prompt)},
			},
		})
		url = strings.TrimRight(e.BaseURL, "/") + "/chat/completions"
	} else {
		payload, err = json.Marshal(body)
	}
	if err != nil {
		return result, err
	}
	if url == "" {
		url = EndpointFor(provider)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", "Bearer "+e.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client := e.Client
	if client == nil {
		client = &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	result.Status = resp.StatusCode
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("Jev evaluator returned HTTP %d", resp.StatusCode)
	}
	const maxResponseBytes = 8 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return result, err
	}
	if len(data) > maxResponseBytes {
		return result, errors.New("Jev evaluator response exceeds size limit")
	}
	var wire struct {
		Answers map[string]struct {
			Type       string          `json:"type"`
			Choice     json.RawMessage `json:"choice"`
			Confidence json.RawMessage `json:"confidence"`
		} `json:"answers"`
		ProviderMetadata json.RawMessage `json:"providerMetadata"`
		Usage            json.RawMessage `json:"usage"`
		Choices          []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(data, &wire) != nil {
		return result, errors.New("Jev evaluator response is invalid")
	}
	if provider == ProviderOpenAICompatible {
		if len(wire.Choices) == 0 || strings.TrimSpace(wire.Choices[0].Message.Content) == "" {
			return result, errors.New("Jev evaluator response is invalid")
		}
		content := strings.TrimSpace(wire.Choices[0].Message.Content)
		if strings.HasPrefix(content, "```") {
			content = strings.TrimPrefix(content, "```")
			content = strings.TrimPrefix(content, "json")
			content = strings.TrimSpace(strings.TrimSuffix(content, "```"))
		}
		var compatible struct {
			Answers map[string]struct {
				Type       string          `json:"type"`
				Choice     json.RawMessage `json:"choice"`
				Confidence json.RawMessage `json:"confidence"`
			} `json:"answers"`
		}
		if json.Unmarshal([]byte(content), &compatible) != nil {
			return result, errors.New("Jev evaluator response is invalid")
		}
		if compatible.Answers == nil {
			var direct map[string]json.RawMessage
			if json.Unmarshal([]byte(content), &direct) != nil || len(direct) == 0 {
				return result, errors.New("Jev evaluator response is invalid")
			}
			compatible.Answers = make(map[string]struct {
				Type       string          `json:"type"`
				Choice     json.RawMessage `json:"choice"`
				Confidence json.RawMessage `json:"confidence"`
			}, len(direct))
			for name, choice := range direct {
				var value string
				if json.Unmarshal(choice, &value) != nil {
					return result, errors.New("Jev evaluator response is invalid")
				}
				compatible.Answers[name] = struct {
					Type       string          `json:"type"`
					Choice     json.RawMessage `json:"choice"`
					Confidence json.RawMessage `json:"confidence"`
				}{Type: "choice", Choice: choice}
			}
		}
		wire.Answers, wire.Usage = compatible.Answers, wire.Usage
	}
	if wire.Answers == nil {
		return result, errors.New("Jev evaluator response is invalid")
	}
	choices := make(map[string]string, len(wire.Answers))
	for name, answer := range wire.Answers {
		var choice string
		if answer.Type != "choice" || len(answer.Choice) == 0 || string(answer.Choice) == "null" || json.Unmarshal(answer.Choice, &choice) != nil {
			return result, errors.New("Jev evaluator answer is invalid")
		}
		choices[name] = choice
	}
	confidence := map[string]float64{}
	// The official API attaches confidence to every choice answer.
	for name, answer := range wire.Answers {
		if len(answer.Confidence) == 0 || string(answer.Confidence) == "null" {
			continue
		}
		var value float64
		if json.Unmarshal(answer.Confidence, &value) != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			return result, errors.New("Jev evaluator confidence is invalid")
		}
		confidence[name] = value
	}
	if len(wire.Usage) > 0 {
		var usage map[string]json.RawMessage
		if json.Unmarshal(wire.Usage, &usage) != nil || usage == nil {
			return result, errors.New("Jev evaluator usage is invalid")
		}
		inputName, outputName := "input_tokens", "output_tokens"
		if raw, exists := usage[inputName]; exists {
			if string(raw) == "null" || json.Unmarshal(raw, &result.InputTokens) != nil || result.InputTokens < 0 {
				return result, errors.New("Jev evaluator input token count is invalid")
			}
			result.InputKnown = true
		}
		if raw, exists := usage[outputName]; exists {
			if string(raw) == "null" || json.Unmarshal(raw, &result.OutputTokens) != nil || result.OutputTokens < 0 {
				return result, errors.New("Jev evaluator output token count is invalid")
			}
			result.OutputKnown = true
		}
	}
	result.Answers = make(map[string]Answer, len(questions))
	for name, question := range questions {
		choice, exists := choices[name]
		if !exists {
			return result, fmt.Errorf("Jev evaluator omitted choice %q", name)
		}
		if _, allowed := question.Criteria[choice]; !allowed {
			return result, fmt.Errorf("Jev evaluator returned an unavailable choice for %q", name)
		}
		value, hasConfidence := confidence[name]
		result.Answers[name] = Answer{Choice: choice, Confidence: value, HasConf: hasConfidence}
	}
	return result, nil
}
