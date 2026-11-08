package controller

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseImageURLList(t *testing.T) {
	t.Setenv("VISION_MAX_IMAGES", "3")

	raw := `["https://a.com/1.jpg","ftp://a.com/2.jpg","https://a.com/1.jpg","https://a.com/3.jpg","https://a.com/4.jpg"]`
	got := parseImageURLList(raw)
	if len(got) != 3 {
		t.Fatalf("期望 3 个去重后的 http URL，实际 %d 个: %v", len(got), got)
	}
	want := []string{"https://a.com/1.jpg", "https://a.com/3.jpg", "https://a.com/4.jpg"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个 URL 不匹配: got=%s want=%s", i, got[i], want[i])
		}
	}

	if got := parseImageURLList("https://a.com/single.jpg"); len(got) != 1 || got[0] != "https://a.com/single.jpg" {
		t.Fatalf("单个 URL 解析失败: %v", got)
	}
	if got := parseImageURLList(""); got != nil {
		t.Fatalf("空字符串应返回 nil，实际 %v", got)
	}
	if got := parseImageURLList("[]"); len(got) != 0 {
		t.Fatalf("空数组应返回空，实际 %v", got)
	}
}

func TestLocalUploadDataURI(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "http://43.142.39.16")

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	if err := os.MkdirAll(filepath.Join(tempDir, "uploads"), 0o755); err != nil {
		t.Fatal(err)
	}
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2ZQAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "uploads", "test.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	got := localUploadDataURI("http://localhost:9094/uploads/test.png")
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("本地图片应转成 data URI，实际: %q", got)
	}

	// PUBLIC_BASE_URL 指向的公网 IP 也应优先读本地文件。
	got = localUploadDataURI("http://43.142.39.16/uploads/test.png")
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("PUBLIC_BASE_URL 图片应转成 data URI，实际: %q", got)
	}

	// ../ 不能逃出 uploads 目录。
	if got := localUploadDataURI("http://localhost:9094/uploads/../secret.png"); got != "" {
		t.Fatalf("路径穿越应被拒绝，实际: %q", got)
	}
	// 不属于 PUBLIC_BASE_URL 的公网域名不能读本地文件。
	if got := localUploadDataURI("https://example.com/uploads/test.png"); got != "" {
		t.Fatalf("其他公网域名不应触发本地读取，实际: %q", got)
	}
}

func TestPrepareVisionImageURLsPrefersBase64(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "http://43.142.39.16")

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	if err := os.MkdirAll(filepath.Join(tempDir, "uploads"), 0o755); err != nil {
		t.Fatal(err)
	}
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2ZQAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "uploads", "test.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	raw := "http://43.142.39.16/uploads/test.png"
	t.Setenv("VISION_IMAGE_SOURCE", "auto")
	got := prepareVisionImageURLs(raw)
	if len(got) != 1 || !strings.HasPrefix(got[0], "data:image/png;base64,") {
		t.Fatalf("auto 模式应优先返回 base64，实际: %v", got)
	}

	// 强制 URL 模式时保留原始 URL，交给模型自己下载。
	t.Setenv("VISION_IMAGE_SOURCE", "url")
	got = prepareVisionImageURLs(raw)
	if len(got) != 1 || got[0] != raw {
		t.Fatalf("url 模式应返回原始 URL，实际: %v", got)
	}
}

func TestSignOSSURLIfNeeded(t *testing.T) {
	t.Setenv("OSS_ENDPOINT", "oss-cn-chengdu.aliyuncs.com")
	t.Setenv("OSS_BUCKET_NAME", "demo-bucket")
	t.Setenv("OSS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("OSS_ACCESS_KEY_SECRET", "test-access-secret")
	t.Setenv("VISION_IMAGE_URL_TTL", "600")

	raw := "https://demo-bucket.oss-cn-chengdu.aliyuncs.com/2/image/a.jpg"
	got := signOSSURLIfNeeded(raw)
	if !strings.Contains(got, "OSSAccessKeyId=") || !strings.Contains(got, "Signature=") {
		t.Fatalf("OSS URL 未生成签名: %s", got)
	}
	if !strings.Contains(got, "Expires=") {
		t.Fatalf("OSS 签名缺少过期时间: %s", got)
	}

	// 非本项目的域名原样返回，便于 PUBLIC_BASE_URL / 自定义域名继续使用。
	other := "https://example.com/uploads/a.jpg"
	if got := signOSSURLIfNeeded(other); got != other {
		t.Fatalf("非 OSS 域名不应改写，实际: %s", got)
	}
}

func TestBuildVisionConversation(t *testing.T) {
	input := AskData{
		Conversation: "这个什么情况",
		Prediction:   "湿疹",
		ResultSearch: "湿疹概述",
		DescSearch:   "红斑瘙痒",
		Weight:       "0.6",
	}
	got := buildVisionConversation(input, "湿疹、接触性皮炎", "红斑、少量鳞屑", "", 2)
	for _, want := range []string{
		"已附上用户上传的 2 张照片",
		"先用 2~3 句话描述",
		"分类器只是弱提示",
		"以图片为准",
		"不要提及置信度",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("提示词缺少 %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "必须逐字以“从照片看，可能是”开头") {
		t.Fatalf("视觉提示词不应再强制复述分类器首句")
	}
}

func TestIsPrivateHost(t *testing.T) {
	cases := map[string]bool{
		"localhost":   true,
		"127.0.0.1":   true,
		"192.168.1.2": true,
		"10.0.0.1":    true,
		"172.16.0.1":  true,
		"example.com": false,
		"8.8.8.8":     false,
	}
	for host, want := range cases {
		if got := isPrivateHost(host); got != want {
			t.Fatalf("isPrivateHost(%q)=%v, want %v", host, got, want)
		}
	}
}
