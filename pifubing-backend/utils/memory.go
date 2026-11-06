package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// 记忆接口只给 Go 后端内部调用，默认和文字问诊服务同机同端口。
// 部署时如果 Python 服务单独部署，可用 MEMORY_API_URL 覆盖。
func memoryAPIBaseURL() string {
	if value := strings.TrimSpace(os.Getenv("MEMORY_API_URL")); value != "" {
		return strings.TrimRight(value, "/")
	}
	textAPIURL := getEnvOrDefault("AI_TEXT_API_URL", "http://8.137.81.23:8001/process_text")
	baseURL := strings.TrimSuffix(textAPIURL, "/process_text")
	return strings.TrimRight(baseURL, "/") + "/memory"
}

// setMemoryAuth 给内部记忆请求带上可选令牌。
// Python 端配置了 MEMORY_API_TOKEN 时，没有这个头的请求会被拒绝。
func setMemoryAuth(req *http.Request) {
	if token := strings.TrimSpace(os.Getenv("MEMORY_API_TOKEN")); token != "" {
		req.Header.Set("X-Memory-Token", token)
	}
}

type memorySearchRequest struct {
	UserID int    `json:"user_id"`
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
}

type memorySearchResponse struct {
	Results []struct {
		Memory string  `json:"memory"`
		Score  float64 `json:"score"`
	} `json:"results"`
	Error string `json:"error"`
}

// SearchMemories 查询用户的长期记忆，返回纯文本事实列表。
// 任何失败都返回 nil：记忆只是增强项，不能影响正常问诊。
func SearchMemories(ctx context.Context, userID int, query string) []string {
	query = strings.TrimSpace(query)
	if userID <= 0 || query == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// 检索只需要一次 embedding 调用，正常 0.2~0.3 秒；3 秒足够，超时就放弃。
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	limit := 5
	if raw := strings.TrimSpace(os.Getenv("MEMORY_SEARCH_LIMIT")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 10 {
		limit = 10
	}

	payload, err := json.Marshal(memorySearchRequest{UserID: userID, Query: query, Limit: limit})
	if err != nil {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, memoryAPIBaseURL()+"/search", bytes.NewReader(payload))
	if err != nil {
		log.Printf("创建长期记忆检索请求失败: %v", err)
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	setMemoryAuth(req)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("长期记忆检索失败（用户 %d）: %v", userID, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		log.Printf("长期记忆检索返回异常（用户 %d）: %s %s", userID, resp.Status, strings.TrimSpace(string(body)))
		return nil
	}

	var result memorySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("解析长期记忆检索结果失败（用户 %d）: %v", userID, err)
		return nil
	}
	if result.Error != "" {
		log.Printf("长期记忆检索服务报错（用户 %d）: %s", userID, result.Error)
	}

	memories := make([]string, 0, len(result.Results))
	for _, item := range result.Results {
		text := strings.TrimSpace(item.Memory)
		if text != "" {
			memories = append(memories, text)
		}
	}
	return memories
}

type memoryAddRequest struct {
	UserID     int    `json:"user_id"`
	Question   string `json:"question"`
	ChatID     int    `json:"chat_id,omitempty"`
	Source     string `json:"source,omitempty"`
	Prediction string `json:"prediction,omitempty"`
}

// memoryAddSemaphore 限制同时进行的记忆抽取数量。
// mem0 的 add 需要调用一次大模型做事实抽取，通常 3~7 秒；
// 限制并发可以避免多个用户同时问诊时把大模型配额打满。
var memoryAddSemaphore = make(chan struct{}, 2)

// AddMemoryAsync 在回答完成后异步抽取用户事实，不阻塞本次问诊。
// 只传用户原话，不传 AI 回答，避免把模型诊断存成下一次问诊的“事实”。
func AddMemoryAsync(userID int, chatID int, question string, source string) {
	question = strings.TrimSpace(question)
	// 太短的问题（如“你好”“看看”）没有可抽取的事实，直接跳过。
	if userID <= 0 || utf8.RuneCountInString(question) < 2 {
		return
	}
	select {
	case memoryAddSemaphore <- struct{}{}:
	default:
		log.Printf("长期记忆写入队列已满，跳过本次记忆（用户 %d）", userID)
		return
	}

	go func() {
		defer func() { <-memoryAddSemaphore }()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		payload, err := json.Marshal(memoryAddRequest{
			UserID:   userID,
			Question: question,
			ChatID:   chatID,
			Source:   source,
		})
		if err != nil {
			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, memoryAPIBaseURL()+"/add", bytes.NewReader(payload))
		if err != nil {
			log.Printf("创建长期记忆写入请求失败: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		setMemoryAuth(req)

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("长期记忆写入失败（用户 %d）: %v", userID, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
			log.Printf("长期记忆写入返回异常（用户 %d）: %s %s", userID, resp.Status, strings.TrimSpace(string(body)))
			return
		}
		// 只读取并丢弃响应体，确保连接可以复用；不打印用户病史内容。
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))
	}()
}
