package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// visionSystemPromptSuffix 是图片问诊的补充规则。
// 它追加在 skinDoctorSystemPrompt 后面，而不是替换原提示词：
// 安全边界、拒绝话术、免责声明继续复用同一份，避免文字/图片两条链路口径不一致。
const visionSystemPromptSuffix = `
【图片问诊补充规则】本次用户已上传图片，你可以直接看到图片内容，不要说“我看不到图片”。
1. 先描述图片中可见的皮损（部位、数量/分布、边界、颜色、表面），再给倾向性判断；这条优先于上面的“先给判断”。
2. 分类器结论只是弱提示，可能出错；图片与分类器冲突时，以图片为准，并说明“分类器倾向 X，但视觉上更像 Y”。
3. 不要提及置信度、概率、百分比；不要描述图片里不存在的东西。
4. 图片信息不足时，明确说“仅凭这张照片难以确定”，再列出 1~2 个需要排除的情况和就医建议。
`

// visionContentPart 对应 OpenAI 兼容多模态消息中的一个内容块。
// image_url 块和 text 块可以混排；本项目把图片放在前面，让模型先看图再看问题。
type visionContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *visionImageURL `json:"image_url,omitempty"`
}

type visionImageURL struct {
	URL string `json:"url"`
}

// visionMessage 的 Content 既可以是字符串（system），也可以是内容块数组（user）。
type visionMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type visionCompletionRequest struct {
	Model       string          `json:"model"`
	Messages    []visionMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
}

// firstNonEmpty 返回第一个非空字符串，用于按优先级回退配置。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// CallVisionModelStream 把图片 URL 和文字问题一起发给多模态模型，并流式转发回答。
// 返回的完整文本由调用方写入数据库；失败时调用方可以回退到原纯文本链路。
func CallVisionModelStream(imageURLs []string, fullConversation string, emit TextStreamEmitter) (string, error) {
	if len(imageURLs) == 0 {
		return "", fmt.Errorf("没有可用的图片地址")
	}

	// 优先用独立的视觉 key；没有时兼容现有部署：
	// 有的环境只配置了 LLM_API_KEY，有的图片链路只配置了 DeepSeek_R1_STUDIO_API_KEY。
	apiKey := firstNonEmpty(
		os.Getenv("VISION_API_KEY"),
		os.Getenv("LLM_API_KEY"),
		os.Getenv("DeepSeek_R1_STUDIO_API_KEY"),
	)
	if apiKey == "" {
		return "", fmt.Errorf("未配置 VISION_API_KEY 或 LLM_API_KEY")
	}

	baseURL := strings.TrimRight(firstNonEmpty(
		os.Getenv("VISION_BASE_URL"),
		os.Getenv("LLM_BASE_URL"),
		"https://open.bigmodel.cn/api/paas/v4",
	), "/")
	model := getEnvOrDefault("VISION_ANSWER_MODEL", "glm-4v-plus")

	// 图片块放在文字块前面。部分多模态模型对“先图后文”的指令跟随更稳定。
	parts := make([]visionContentPart, 0, len(imageURLs)+1)
	for _, imageURL := range imageURLs {
		parts = append(parts, visionContentPart{
			Type:     "image_url",
			ImageURL: &visionImageURL{URL: imageURL},
		})
	}
	parts = append(parts, visionContentPart{Type: "text", Text: fullConversation})

	requestBody := visionCompletionRequest{
		Model: model,
		Messages: []visionMessage{
			{Role: "system", Content: skinDoctorSystemPrompt + visionSystemPromptSuffix},
			{Role: "user", Content: parts},
		},
		Stream:      true,
		Temperature: getEnvFloat("LLM_TEMPERATURE", 0.3),
		TopP:        getEnvFloat("LLM_TOP_P", 0.9),
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("构造视觉请求失败: %v", err)
	}

	req, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建视觉请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// 图片下载 + 多图推理比纯文本慢，超时比普通文字接口放宽。
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用视觉模型失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 不把上游响应体原样返回：错误信息里可能带签名图片 URL，
		// 而签名 URL 等同于临时访问凭证，不能写进前端或日志。
		return "", fmt.Errorf("视觉模型返回状态码 %d", resp.StatusCode)
	}

	var fullResponseText string
	scanner := bufio.NewScanner(resp.Body)
	// 单条 SSE 可能包含较长的 Markdown，放宽默认 64KB 限制。
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var chunk CompletionResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// 心跳、注释等非 JSON 行直接跳过，不中断整条回答。
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		content := chunk.Choices[0].Delta.Content
		reasoningContent := chunk.Choices[0].Delta.ReasoningContent
		if reasoningContent != "" && emit != nil {
			emit("thoughts", gin.H{"thoughts": reasoningContent})
		}
		if content != "" {
			fullResponseText += content
			if emit != nil {
				emit("message", gin.H{"content": content})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		// 已经收到的部分正文一并返回，调用方据此判断是否还能回退。
		return fullResponseText, fmt.Errorf("读取视觉流式响应失败: %v", err)
	}
	if strings.TrimSpace(fullResponseText) == "" {
		return "", fmt.Errorf("视觉模型未返回内容")
	}
	return fullResponseText, nil
}
