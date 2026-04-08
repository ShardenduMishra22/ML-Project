package explain

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"landrisk/backend-go/internal/domain"
	"landrisk/backend-go/internal/httpclient"
)

const (
	defaultBaseURL = "https://openrouter.ai/api/v1"
	defaultModel   = "openai/gpt-oss-120b:free"
)

type Client struct {
	baseURL    string
	apiKey     string
	model      string
	timeout    time.Duration
	httpClient *http.Client
	logger     zerolog.Logger
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content any `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type llmCitizenSummary struct {
	Overview   string   `json:"overview"`
	KeyPoints  []string `json:"keyPoints"`
	NextSteps  []string `json:"nextSteps"`
	Disclaimer string   `json:"disclaimer"`
}

type llmInput struct {
	RiskClass             string   `json:"riskClass"`
	ConfidencePercent     int      `json:"confidencePercent"`
	ModelProbability      int      `json:"modelProbabilityPercent"`
	CrossSourceAgreement  bool     `json:"crossSourceAgreement"`
	WaterOverlapRatio     float64  `json:"waterOverlapRatio"`
	VegetationDensity     float64  `json:"vegetationDensity"`
	RuleOverrides         []string `json:"ruleOverrides"`
	ValidationNotes       []string `json:"validationNotes"`
	TechnicalExplanation  []string `json:"technicalExplanation"`
	NonTechnicalObjective string   `json:"nonTechnicalObjective"`
}

func NewClient(baseURL, apiKey, model string, timeout time.Duration, logger zerolog.Logger) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = defaultModel
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     strings.TrimSpace(apiKey),
		model:      model,
		timeout:    timeout,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger.With().Str("component", "citizen-summary-client").Logger(),
	}
}

func (c *Client) Summarize(ctx context.Context, report domain.Report) domain.CitizenSummary {
	fallback := buildFallbackSummary(report)
	if c.apiKey == "" {
		return fallback
	}

	payload, err := c.buildRequestPayload(report)
	if err != nil {
		c.logger.Warn().Err(err).Msg("failed to build openrouter request")
		return fallback
	}

	body, status, err := httpclient.DoWithRetry(ctx, c.httpClient, httpclient.RequestConfig{
		Method:      http.MethodPost,
		URL:         c.baseURL + "/chat/completions",
		Headers:     map[string]string{"Content-Type": "application/json", "Accept": "application/json", "Authorization": "Bearer " + c.apiKey},
		Body:        payload,
		RetryCount:  1,
		BaseBackoff: 300 * time.Millisecond,
		Timeout:     c.timeout,
	})
	if err != nil {
		c.logger.Warn().Err(err).Int("status", status).Msg("openrouter summary request failed")
		return fallback
	}

	candidate, err := parseChatSummary(body)
	if err != nil {
		c.logger.Warn().Err(err).Msg("openrouter summary parse failed")
		return fallback
	}

	return sanitizeSummary(candidate, fallback)
}

func (c *Client) buildRequestPayload(report domain.Report) ([]byte, error) {
	confidence := clampToUnit(report.Confidence)
	modelProbability := clampToUnit(report.MLPrediction.Probability)

	input := llmInput{
		RiskClass:             safeRiskClass(report.RiskClass),
		ConfidencePercent:     int(math.Round(confidence * 100)),
		ModelProbability:      int(math.Round(modelProbability * 100)),
		CrossSourceAgreement:  report.ValidationSummary.CrossSourceAgreement,
		WaterOverlapRatio:     report.SatelliteFeatures.WaterOverlapRatio,
		VegetationDensity:     report.SatelliteFeatures.VegetationDensity,
		RuleOverrides:         report.ValidationSummary.RuleOverrides,
		ValidationNotes:       report.ValidationSummary.Notes,
		TechnicalExplanation:  report.Explanation,
		NonTechnicalObjective: "Explain result in simple terms for non-technical land owners.",
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	req := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "You explain land-risk assessment outcomes to non-technical users. Return only JSON with keys overview, keyPoints, nextSteps, disclaimer. Keep language plain and short. Avoid legal guarantees.",
			},
			{
				Role:    "user",
				Content: "Generate a reliable plain-language summary from this assessment data: " + string(inputJSON),
			},
		},
		Temperature: 0.2,
	}

	return json.Marshal(req)
}

func parseChatSummary(body []byte) (llmCitizenSummary, error) {
	var response chatResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return llmCitizenSummary{}, err
	}
	if len(response.Choices) == 0 {
		return llmCitizenSummary{}, fmt.Errorf("no choices in openrouter response")
	}

	content := extractContent(response.Choices[0].Message.Content)
	if strings.TrimSpace(content) == "" {
		return llmCitizenSummary{}, fmt.Errorf("empty message content")
	}

	jsonChunk := extractJSONObject(content)
	if strings.TrimSpace(jsonChunk) == "" {
		return llmCitizenSummary{}, fmt.Errorf("no json object found in content")
	}

	var parsed llmCitizenSummary
	if err := json.Unmarshal([]byte(jsonChunk), &parsed); err != nil {
		return llmCitizenSummary{}, err
	}
	return parsed, nil
}

func extractContent(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, raw := range typed {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			text, ok := item["text"].(string)
			if ok && strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func extractJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return strings.TrimSpace(trimmed[start : end+1])
}

func sanitizeSummary(candidate llmCitizenSummary, fallback domain.CitizenSummary) domain.CitizenSummary {
	overview := cleanSentence(candidate.Overview)
	if overview == "" {
		overview = fallback.Overview
	}

	keyPoints := cleanList(candidate.KeyPoints, 4)
	if len(keyPoints) == 0 {
		keyPoints = fallback.KeyPoints
	}

	nextSteps := cleanList(candidate.NextSteps, 3)
	if len(nextSteps) == 0 {
		nextSteps = fallback.NextSteps
	}

	disclaimer := cleanSentence(candidate.Disclaimer)
	if disclaimer == "" {
		disclaimer = fallback.Disclaimer
	}

	return domain.CitizenSummary{
		Overview:   overview,
		KeyPoints:  keyPoints,
		NextSteps:  nextSteps,
		Disclaimer: disclaimer,
		Source:     "openrouter",
	}
}

func buildFallbackSummary(report domain.Report) domain.CitizenSummary {
	riskClass := strings.ToLower(safeRiskClass(report.RiskClass))
	confidence := clampToUnit(report.Confidence)
	if confidence == 0 {
		confidence = clampToUnit(report.MLPrediction.Probability)
	}
	confidencePct := int(math.Round(confidence * 100))
	modelProbabilityPct := int(math.Round(clampToUnit(report.MLPrediction.Probability) * 100))

	overview := fmt.Sprintf(
		"This land parcel is currently assessed as %s risk with about %d%% confidence after checking location, satellite signals, and validation rules.",
		riskClass,
		confidencePct,
	)

	keyPoints := []string{
		fmt.Sprintf("Model probability for this risk result is about %d%%.", modelProbabilityPct),
		satellitePoint(report.SatelliteFeatures.WaterOverlapRatio, report.SatelliteFeatures.VegetationDensity),
		agreementPoint(report.ValidationSummary.CrossSourceAgreement),
	}
	if len(report.ValidationSummary.RuleOverrides) > 0 {
		keyPoints = append(keyPoints, "A validation rule override influenced the final risk decision.")
	}
	keyPoints = cleanList(keyPoints, 4)

	nextSteps := []string{
		"Use this as an early screening result and verify with official land records.",
		nextStepByRisk(safeRiskClass(report.RiskClass)),
		"If ownership or boundary details are critical, request a local field verification.",
	}

	return domain.CitizenSummary{
		Overview:   overview,
		KeyPoints:  cleanList(keyPoints, 4),
		NextSteps:  cleanList(nextSteps, 3),
		Disclaimer: "This summary supports decision-making and is not a legal approval or rejection.",
		Source:     "fallback-rule-engine",
	}
}

func safeRiskClass(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "Unknown"
	}
	return trimmed
}

func nextStepByRisk(riskClass string) string {
	switch strings.ToLower(strings.TrimSpace(riskClass)) {
	case "high":
		return "Treat this case as priority for manual review before any final decision."
	case "medium":
		return "Proceed with caution and collect supporting documents before final action."
	default:
		return "Proceed with normal verification checks before final action."
	}
}

func agreementPoint(agreement bool) string {
	if agreement {
		return "Independent data checks are mostly aligned, which improves reliability."
	}
	return "Some data sources disagree, so this result should be reviewed carefully."
}

func satellitePoint(waterOverlap, vegetationDensity float64) string {
	if waterOverlap >= 0.4 {
		return "Satellite imagery shows notable water overlap near or within the parcel area."
	}
	if waterOverlap <= 0.1 {
		return "Satellite imagery shows limited water overlap for this parcel area."
	}
	if vegetationDensity >= 0.6 {
		return "Vegetation appears relatively dense, which may affect local land-use interpretation."
	}
	return "Satellite indicators are moderate and do not show an extreme land signal."
}

func clampToUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func cleanSentence(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
}

func cleanList(values []string, maxItems int) []string {
	if maxItems <= 0 {
		return []string{}
	}

	out := make([]string, 0, maxItems)
	seen := map[string]struct{}{}
	for _, raw := range values {
		item := cleanSentence(raw)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
		if len(out) == maxItems {
			break
		}
	}
	return out
}
