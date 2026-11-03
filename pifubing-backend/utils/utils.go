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
	"net/http"
	"os"
	"strings"
)

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
