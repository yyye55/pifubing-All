package controller

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newAskDataContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	return c, recorder
}

func TestReadAskDataPostJSON(t *testing.T) {
	body := `{"conversation":"这个什么情况","prediction":"痤疮"}`
	c, recorder := newAskDataContext(http.MethodPost, "/predict/text", body)

	got, ok := readAskData(c)
	if !ok {
		t.Fatalf("POST JSON 应读取成功，状态码 %d", recorder.Code)
	}
	if string(got) != body {
		t.Fatalf("请求体不匹配: got=%s want=%s", got, body)
	}
}

func TestReadAskDataGetBase64(t *testing.T) {
	body := `{"conversation":"这个什么情况"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	c, _ := newAskDataContext(http.MethodGet, "/predict/text?data="+encoded, "")

	got, ok := readAskData(c)
	if !ok {
		t.Fatal("GET base64 应读取成功")
	}
	if string(got) != body {
		t.Fatalf("解码结果不匹配: got=%s want=%s", got, body)
	}
}

func TestReadAskDataRejectsMissingData(t *testing.T) {
	cases := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "GET 无参数", method: http.MethodGet, target: "/predict/text", body: ""},
		{name: "POST 空请求体", method: http.MethodPost, target: "/predict/text", body: "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := newAskDataContext(tc.method, tc.target, tc.body)
			if _, ok := readAskData(c); ok {
				t.Fatal("缺少数据时应返回失败")
			}
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("期望 400，实际 %d", recorder.Code)
			}
		})
	}
}
