package utils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// getEnvOrDefault 读取环境变量，为空时返回默认值。
func getEnvOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// SaveConversationToRedis 存储对话内容到 Redis
func SaveConversationToRedis(ctx context.Context, rdb *redis.Client, conversationKey, question, answer string) error {
	// 去除 JSON 格式标记
	cleanedText := strings.Replace(answer, "\n\n```json\n", "", -1)
	cleanedText = strings.Replace(cleanedText, "\n```\n", "", -1)

	// 组合对话内容
	currentMessage := fmt.Sprintf("Q: %s\nA: %s", question, cleanedText)

	// 将对话内容推入 Redis 列表
	err := rdb.RPush(ctx, conversationKey, currentMessage).Err()
	if err != nil {
		return fmt.Errorf("Failed to save conversation: %w", err)
	}

	// 限制 Redis 列表最大长度，例如保持最近的 10 条对话
	err = rdb.LTrim(ctx, conversationKey, -2, -1).Err()
	if err != nil {
		return fmt.Errorf("Failed to trim conversation history: %w", err)
	}

	return nil
}

// CompletionRequest 结构体
type CompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// Message 结构体
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionResponse struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`           // 回答内容
			ReasoningContent string `json:"reasoning_content"` // 深度思考内容
		} `json:"delta"`
	} `json:"choices"`
}

func CallDeepSeekR1AIStudioAPIStream(fullConversation string, c *gin.Context) (string, error) {
	apiKey := os.Getenv("DeepSeek_R1_STUDIO_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("AI_STUDIO_API_KEY environment variable not set")
	}

	url := "https://open.bigmodel.cn/api/paas/v4/chat/completions"

	requestBody := CompletionRequest{
		Model: "glm-4-flash",
		Messages: []Message{
			{Role: "system", Content: "你现在是一个专业的皮肤科医生，请结合一下内容生成专业的治疗方案"},
			{Role: "user", Content: fullConversation},
		},
		Stream: true, // 开启流式响应
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("Error marshalling request body: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("Error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Error sending request: %v", err)
	}
	defer resp.Body.Close()

	// 检查 API 状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned non-200 status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// 逐行读取流式响应并推送到前端
	var fullResponseText string // 用于存储完整的 AI 回复
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			if data == "[DONE]" {
				break
			}

			// 解析 JSON 数据
			var chunk CompletionResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return "", fmt.Errorf("Error decoding JSON: %v", err)
			}

			// 获取 AI 回复的内容和深度思考
			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				reasoningContent := chunk.Choices[0].Delta.ReasoningContent

				// 推送深度思考内容
				if reasoningContent != "" {
					c.SSEvent("thoughts", reasoningContent)
				}

				// 推送回答内容
				if content != "" {
					c.SSEvent("message", content)
					fullResponseText += content
				}

				c.Writer.Flush()
			}
		}
	}

	// 检查流式读取是否有错误
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("Error reading stream: %v", err)
	}

	return fullResponseText, nil
}

// textStreamPayload 对应 Python 流式接口输出的 JSON data。
type textStreamPayload struct {
	Content     string   `json:"content"`
	Thoughts    string   `json:"thoughts"`
	Suggestions []string `json:"suggestions"`
	Error       string   `json:"error"`
}

// CallTextStream 调用 Python 的 /process_text_stream，并把每个 chunk 转发给浏览器。
// 返回值是完整回答，供调用方在流结束后写入数据库和 Redis。
func CallTextStream(fullConversation string, c *gin.Context) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("conversation", fullConversation); err != nil {
		return "", fmt.Errorf("写入对话内容失败: %v", err)
	}
	if err := writer.WriteField("predicted_class", ""); err != nil {
		return "", fmt.Errorf("写入预测类别失败: %v", err)
	}
	if err := writer.WriteField("confidence", "0"); err != nil {
		return "", fmt.Errorf("写入置信度失败: %v", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭 multipart writer 失败: %v", err)
	}

	// 优先读独立配置；否则把现有普通文字接口地址的 /process_text 换成 /process_text_stream，
	// 这样本地 .env 只配置 AI_TEXT_API_URL 也能直接走流式。
	textAPIURL := getEnvOrDefault("AI_TEXT_API_URL", "http://8.137.81.23:8001/process_text")
	defaultStreamURL := strings.TrimSuffix(textAPIURL, "/process_text") + "/process_text_stream"
	apiURL := getEnvOrDefault("AI_TEXT_STREAM_API_URL", defaultStreamURL)

	// 绑定浏览器请求的上下文：用户关闭页面或中止请求时，同步取消上游 Python 流。
	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", apiURL, &body)
	if err != nil {
		return "", fmt.Errorf("创建流式请求失败: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "text/event-stream")

	// 流式回答可能持续较久，超时时间比普通请求放宽。
	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接文字流式服务失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("文字流式服务返回错误: %v, 响应内容: %s", resp.Status, string(responseBody))
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

		var payload textStreamPayload
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			// 忽略心跳、注释等非 JSON 行，避免整条流中断
			continue
		}

		if payload.Error != "" {
			return fullResponseText, fmt.Errorf("%s", payload.Error)
		}

		if payload.Thoughts != "" {
			c.SSEvent("thoughts", gin.H{"thoughts": payload.Thoughts})
			c.Writer.Flush()
		}

		if len(payload.Suggestions) > 0 {
			c.SSEvent("suggestions", gin.H{"suggestions": payload.Suggestions})
			c.Writer.Flush()
		}

		if payload.Content != "" {
			fullResponseText += payload.Content
			// 用 JSON 包一层，避免回答里的换行破坏 SSE 的 data 边界
			c.SSEvent("message", gin.H{"content": payload.Content})
			c.Writer.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		return fullResponseText, fmt.Errorf("读取文字流式响应失败: %v", err)
	}

	return fullResponseText, nil
}

type followupPayload struct {
	Suggestions []string `json:"suggestions"`
}

// GenerateFollowups 让 Python 根据本轮问答生成最多3个追问。
// 图片问答的正文由 Go 直接流式返回，因此需要单独调用这个接口补追问。
func GenerateFollowups(ctx context.Context, question, answer string) []string {
	if strings.TrimSpace(question) == "" || strings.TrimSpace(answer) == "" {
		return nil
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("question", question); err != nil {
		return nil
	}
	if err := writer.WriteField("answer", answer); err != nil {
		return nil
	}
	if err := writer.Close(); err != nil {
		return nil
	}

	// 和流式接口一样，默认从 AI_TEXT_API_URL 推导出追问接口地址。
	textAPIURL := getEnvOrDefault("AI_TEXT_API_URL", "http://8.137.81.23:8001/process_text")
	defaultFollowupURL := strings.TrimSuffix(textAPIURL, "/process_text") + "/generate_followups"
	apiURL := getEnvOrDefault("AI_FOLLOWUP_API_URL", defaultFollowupURL)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, &body)
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var payload followupPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}

	suggestions := make([]string, 0, 3)
	for _, item := range payload.Suggestions {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		suggestions = append(suggestions, item)
		if len(suggestions) == 3 {
			break
		}
	}
	return suggestions
}

// HotQuestion 是首页空状态快捷提问需要的短标题和完整问题。
type HotQuestion struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

type hotQuestionsPayload struct {
	Questions []HotQuestion `json:"questions"`
}

// 首页热门问题采用“问题池 + 随机抽取”：
//   - 每小时让大模型生成一池问题（hotQuestionsPoolSize 个）；
//   - 每次刷新从池中随机抽 count 个，保证相邻两次展示的问题不同；
//   - 池子有效时不再调用大模型，刷新几乎没有额外延迟。
var (
	hotQuestionsMu       sync.Mutex
	hotQuestionsCache    []HotQuestion
	hotQuestionsCachedAt time.Time
	hotQuestionsFailedAt time.Time
	// 上一次返回给前端的问题文本，用来避免连续两次抽到完全相同的一组
	hotQuestionsLastPick []string
)

const (
	// 问题池的刷新周期：每小时重新生成一批热门问题。
	hotQuestionsCacheTTL = 1 * time.Hour
	// 池子要明显大于前端展示数量（4 个），随机抽取时才有足够变化。
	hotQuestionsPoolSize = 8
	// AI 服务故障时短暂退避，避免每个用户打开首页都重新等一次超时。
	hotQuestionsFailureTTL = 30 * time.Second
)

// pickHotQuestions 从问题池中随机抽取 count 个，
// 并尽量避免与上一次返回的问题集合完全相同。
func pickHotQuestions(pool []HotQuestion, count int) []HotQuestion {
	if len(pool) == 0 {
		return nil
	}
	if count <= 0 {
		count = 4
	}
	if count >= len(pool) {
		// 池子不够抽时全部返回；打乱顺序让展示顺序也有变化。
		picked := append([]HotQuestion(nil), pool...)
		rand.Shuffle(len(picked), func(i, j int) {
			picked[i], picked[j] = picked[j], picked[i]
		})
		return picked
	}

	hotQuestionsMu.Lock()
	defer hotQuestionsMu.Unlock()

	var picked []HotQuestion
	// 随机抽到与上次相同组合的概率很低，循环几次兜底。
	for attempt := 0; attempt < 5; attempt++ {
		order := rand.Perm(len(pool))
		picked = make([]HotQuestion, 0, count)
		for _, idx := range order[:count] {
			picked = append(picked, pool[idx])
		}
		if !sameHotQuestionTexts(picked, hotQuestionsLastPick) {
			break
		}
	}

	hotQuestionsLastPick = make([]string, len(picked))
	for i, item := range picked {
		hotQuestionsLastPick[i] = item.Text
	}
	return picked
}

// sameHotQuestionTexts 判断两组问题是否为同一集合（与顺序无关）。
func sameHotQuestionTexts(questions []HotQuestion, last []string) bool {
	if len(questions) != len(last) {
		return false
	}
	counts := make(map[string]int, len(last))
	for _, text := range last {
		counts[text]++
	}
	for _, item := range questions {
		if counts[item.Text] == 0 {
			return false
		}
		counts[item.Text]--
	}
	return true
}

// GenerateHotQuestions 让 Python 大模型生成首页的热门皮肤科问题池，
// 再从中随机抽取本次要展示的问题。
// 任何异常都返回 nil，前端会继续使用内置的兜底问题，不影响正常问诊。
func GenerateHotQuestions(ctx context.Context, count int) []HotQuestion {
	if count <= 0 {
		count = 4
	}

	// 问题池有效时直接随机抽取，不再调用大模型。
	hotQuestionsMu.Lock()
	if len(hotQuestionsCache) > 0 && time.Since(hotQuestionsCachedAt) < hotQuestionsCacheTTL {
		pool := append([]HotQuestion(nil), hotQuestionsCache...)
		hotQuestionsMu.Unlock()
		return pickHotQuestions(pool, count)
	}
	// 仍在失败退避期内，直接让前端使用兜底问题。
	if !hotQuestionsFailedAt.IsZero() && time.Since(hotQuestionsFailedAt) < hotQuestionsFailureTTL {
		hotQuestionsMu.Unlock()
		return nil
	}
	hotQuestionsMu.Unlock()

	// fail 记录失败时间并返回 nil；后续请求在退避期内不再等待 AI 超时。
	fail := func() []HotQuestion {
		hotQuestionsMu.Lock()
		hotQuestionsFailedAt = time.Now()
		hotQuestionsMu.Unlock()
		return nil
	}

	// 和追问接口一样，默认从 AI_TEXT_API_URL 推导出热门问题接口地址。
	textAPIURL := getEnvOrDefault("AI_TEXT_API_URL", "http://8.137.81.23:8001/process_text")
	defaultHotQuestionsURL := strings.TrimSuffix(textAPIURL, "/process_text") + "/generate_hot_questions"
	apiURL := getEnvOrDefault("AI_HOT_QUESTIONS_API_URL", defaultHotQuestionsURL)
	apiURL = fmt.Sprintf("%s?count=%d", apiURL, hotQuestionsPoolSize)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fail()
	}

	// 一次要生成一整池问题，模型耗时比单个回答更长，超时给足。
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fail()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fail()
	}

	var payload hotQuestionsPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fail()
	}

	questions := make([]HotQuestion, 0, hotQuestionsPoolSize)
	for _, item := range payload.Questions {
		label := strings.TrimSpace(item.Label)
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		if label == "" {
			// 前端按钮需要短标题；模型漏给时用问题开头兜底。
			runes := []rune(text)
			if len(runes) > 8 {
				runes = runes[:8]
			}
			label = string(runes)
		}
		questions = append(questions, HotQuestion{Label: label, Text: text})
		if len(questions) == hotQuestionsPoolSize {
			break
		}
	}

	if len(questions) == 0 {
		return fail()
	}

	hotQuestionsMu.Lock()
	hotQuestionsCache = append([]HotQuestion(nil), questions...)
	hotQuestionsCachedAt = time.Now()
	hotQuestionsFailedAt = time.Time{} // 成功后清空失败记录
	hotQuestionsLastPick = nil         // 新池子重新开始记录
	hotQuestionsMu.Unlock()

	return pickHotQuestions(questions, count)
}

func CallDeepSeekV3AIStudioAPIStream(fullConversation string, c *gin.Context) (string, error) {
	apiKey := os.Getenv("DeepSeek_V3_STUDIO_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("AI_STUDIO_API_KEY environment variable not set")
	}

	url := "https://api.deepseek.com/v1/chat/completions"

	requestBody := CompletionRequest{
		Model: "deepseek-chat",
		Messages: []Message{
			{Role: "system", Content: "你现在是一个专业的皮肤科医生，请结合一下内容生成专业的治疗方案"},
			{Role: "user", Content: fullConversation},
		},
		Stream: true, // 开启流式响应
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("Error marshalling request body: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("Error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Error sending request: %v", err)
	}
	defer resp.Body.Close()

	// 检查 API 状态码
	if resp.StatusCode != http.StatusOK {
		fmt.Println(resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned non-200 status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// 逐行读取流式响应并推送到前端
	var fullResponseText string // 用于存储完整的 AI 回复
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			if data == "[DONE]" {
				break
			}

			// 解析 JSON 数据
			var chunk CompletionResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return "", fmt.Errorf("Error decoding JSON: %v", err)
			}

			// 获取 AI 回复的内容和深度思考
			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				// 推送回答内容
				if content != "" {
					c.SSEvent("message", content)
					fullResponseText += content
				}

				c.Writer.Flush()
			}
		}
	}

	// 检查流式读取是否有错误
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("Error reading stream: %v", err)
	}

	return fullResponseText, nil
}
