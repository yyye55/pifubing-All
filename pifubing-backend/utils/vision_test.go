package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCallVisionModelStream(t *testing.T) {
	var requestBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("解析请求体失败: %v", err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("响应不支持 flush")
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"从照片看\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"，更像是湿疹\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	t.Setenv("VISION_BASE_URL", server.URL)
	t.Setenv("VISION_API_KEY", "test-key")
	t.Setenv("VISION_ANSWER_MODEL", "glm-4v-plus")

	var events []string
	text, err := CallVisionModelStream(
		[]string{"https://example.com/a.jpg"},
		"用户问题",
		func(event string, payload interface{}) bool {
			events = append(events, event)
			return true
		},
	)
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if text != "从照片看，更像是湿疹" {
		t.Fatalf("完整回答不匹配: %q", text)
	}
	if len(events) != 2 || events[0] != "message" || events[1] != "message" {
		t.Fatalf("流式事件不匹配: %v", events)
	}

	// 校验多模态消息结构：图片块在前，文字块在后。
	messages, ok := requestBody["messages"].([]interface{})
	if !ok || len(messages) != 2 {
		t.Fatalf("messages 结构不正确: %#v", requestBody["messages"])
	}
	userMessage, ok := messages[1].(map[string]interface{})
	if !ok {
		t.Fatalf("user message 结构不正确: %#v", messages[1])
	}
	content, ok := userMessage["content"].([]interface{})
	if !ok || len(content) != 2 {
		t.Fatalf("content 结构不正确: %#v", userMessage["content"])
	}
	first, _ := content[0].(map[string]interface{})
	second, _ := content[1].(map[string]interface{})
	if first["type"] != "image_url" || second["type"] != "text" {
		t.Fatalf("图片块必须排在文字块前面: %#v", content)
	}
	imageURL, _ := first["image_url"].(map[string]interface{})
	if imageURL["url"] != "https://example.com/a.jpg" {
		t.Fatalf("图片 URL 不正确: %#v", first["image_url"])
	}
	if requestBody["model"] != "glm-4v-plus" {
		t.Fatalf("模型名不正确: %v", requestBody["model"])
	}
}

func TestCallVisionModelStreamNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"message":"failed to download https://example.com/a.jpg?Signature=secret"}}`)
	}))
	defer server.Close()

	t.Setenv("VISION_BASE_URL", server.URL)
	t.Setenv("VISION_API_KEY", "test-key")

	_, err := CallVisionModelStream([]string{"https://example.com/a.jpg"}, "问题", nil)
	if err == nil {
		t.Fatal("非 200 应返回错误")
	}
	// 上游响应体可能带签名 URL，不能原样透出。
	if strings.Contains(err.Error(), "Signature") {
		t.Fatalf("错误信息泄露了签名 URL: %v", err)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("错误信息应包含状态码: %v", err)
	}
}
