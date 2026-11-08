package controller

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

const (
	defaultVisionMaxImages = 3
	// 本地调试转 base64 时限制原图大小，避免请求体过大被多模态接口拒绝。
	maxVisionImageBytes = 5 * 1024 * 1024
)

// visionAnswerEnabled 控制图片问诊是否让最终回答模型直接看图。
// 默认开启；设为 false 时完全回到原来的纯文本链路，便于线上快速回滚。
func visionAnswerEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("VISION_ANSWER_ENABLED"))
	if raw == "" {
		return true
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return true
	}
	return enabled
}

// visionMaxImages 限制单次传给模型的图片数，防止客户端塞入过多图片拖慢推理。
func visionMaxImages() int {
	raw := strings.TrimSpace(os.Getenv("VISION_MAX_IMAGES"))
	if raw == "" {
		return defaultVisionMaxImages
	}
	count, err := strconv.Atoi(raw)
	if err != nil || count < 1 {
		return defaultVisionMaxImages
	}
	if count > 5 {
		return 5
	}
	return count
}

// visionImageURLTTL 是 OSS 签名 URL 的有效期（秒）。
// 只用于模型拉图这一瞬间，默认 10 分钟，足够覆盖排队和重试。
func visionImageURLTTL() int64 {
	raw := strings.TrimSpace(os.Getenv("VISION_IMAGE_URL_TTL"))
	if raw == "" {
		return 600
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seconds < 60 {
		return 600
	}
	if seconds > 3600 {
		return 3600
	}
	return seconds
}

// parseImageURLList 兼容前端现在的两种格式：
//   - JSON 数组字符串：["https://a/1.jpg","https://a/2.jpg"]
//   - 单个 URL：https://a/1.jpg
//
// 只接受 http/https，并且最多返回 visionMaxImages 个去重后的地址。
func parseImageURLList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var candidates []string
	if strings.HasPrefix(raw, "[") {
		if err := json.Unmarshal([]byte(raw), &candidates); err != nil {
			return nil
		}
	} else {
		candidates = []string{raw}
	}

	limit := visionMaxImages()
	seen := make(map[string]bool, len(candidates))
	result := make([]string, 0, limit)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Host == "" {
			continue
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			continue
		}
		// 不允许带账号密码的 URL，避免把凭据转发给第三方模型。
		if parsed.User != nil {
			continue
		}
		seen[candidate] = true
		result = append(result, candidate)
		if len(result) >= limit {
			break
		}
	}
	return result
}

// visionImageSource 控制图片来源：
//   - auto（默认）：本机 ./uploads 有文件时优先 base64，否则用 URL；
//   - base64：优先读本地文件，读不到再退回 URL；
//   - url：始终把 URL 交给模型，由模型自己下载。
func visionImageSource() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("VISION_IMAGE_SOURCE"))) {
	case "url":
		return "url"
	case "base64":
		return "base64"
	default:
		return "auto"
	}
}

// prepareVisionImageURLs 把前端传来的原始 URL 转换成模型真正能访问的地址：
//   - local 模式：本机 ./uploads 有文件时优先转 base64，模型不必访问公网；
//   - 阿里云 OSS：私有 bucket 生成短时效签名 URL；
//   - 公网域名：原样返回，但必须在白名单内，避免被恶意客户端利用做 SSRF。
func prepareVisionImageURLs(raw string) []string {
	candidates := parseImageURLList(raw)
	if len(candidates) == 0 {
		return nil
	}

	source := visionImageSource()
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		// 默认优先 base64：医疗图片不暴露公网路径，也不依赖模型能访问服务器 IP。
		if source != "url" {
			if dataURI := localUploadDataURI(candidate); dataURI != "" {
				result = append(result, dataURI)
				continue
			}
		}
		if resolved := resolveVisionImageURL(candidate); resolved != "" {
			result = append(result, resolved)
		}
	}
	return result
}

// resolveVisionImageURL 校验域名并处理 OSS 签名。
// 返回空字符串表示这张图不能安全地交给模型，调用方会跳过它。
func resolveVisionImageURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if isPrivateHost(host) {
		// 私网地址模型无法访问；本地文件已经在 localUploadDataURI 里处理。
		return ""
	}
	if !isAllowedVisionImageHost(host) {
		log.Printf("图片问答：忽略不在白名单内的图片域名 %s", host)
		return ""
	}
	return signOSSURLIfNeeded(raw)
}

// isAllowedVisionImageHost 只允许本项目的 OSS 域名、PUBLIC_BASE_URL 域名，
// 以及运维通过 VISION_IMAGE_HOSTS 显式追加的域名。
func isAllowedVisionImageHost(host string) bool {
	if host == "" {
		return false
	}

	if endpoint, bucketName := ossEndpointAndBucket(); endpoint != "" && bucketName != "" {
		if strings.EqualFold(host, bucketName+"."+endpoint) {
			return true
		}
	}

	if baseURL := strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")); baseURL != "" {
		if parsed, err := url.Parse(baseURL); err == nil && strings.EqualFold(host, parsed.Hostname()) {
			return true
		}
	}

	// 支持精确域名和 *.example.com 通配，便于以后接 CDN/自定义域名。
	for _, item := range strings.Split(os.Getenv("VISION_IMAGE_HOSTS"), ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if strings.HasPrefix(item, "*.") {
			if strings.HasSuffix(host, strings.TrimPrefix(item, "*")) {
				return true
			}
			continue
		}
		if host == item {
			return true
		}
	}
	return false
}

// ossEndpointAndBucket 统一读取 OSS 配置。
// OSS_ENDPOINT 允许写成带 scheme 的形式，SDK 创建客户端时需要纯域名。
func ossEndpointAndBucket() (string, string) {
	endpoint := strings.TrimSpace(os.Getenv("OSS_ENDPOINT"))
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimRight(endpoint, "/")
	bucketName := strings.TrimSpace(os.Getenv("OSS_BUCKET_NAME"))
	if endpoint == "" || bucketName == "" {
		return "", ""
	}
	return endpoint, bucketName
}

// signOSSURLIfNeeded 对阿里云 OSS 地址生成短时效签名 URL。
// 公有读 bucket 用原 URL 也能访问，但私有 bucket 必须签名；
// 这里统一签名，兼容两种 bucket 策略。
func signOSSURLIfNeeded(raw string) string {
	endpoint, bucketName := ossEndpointAndBucket()
	if endpoint == "" || bucketName == "" {
		return raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if !strings.EqualFold(parsed.Hostname(), bucketName+"."+endpoint) {
		// 不是本项目的 OSS 域名，交由白名单后的原 URL 处理。
		return raw
	}

	objectKey := strings.TrimPrefix(parsed.Path, "/")
	if objectKey == "" {
		return ""
	}

	accessKeyID := strings.TrimSpace(os.Getenv("OSS_ACCESS_KEY_ID"))
	accessKeySecret := strings.TrimSpace(os.Getenv("OSS_ACCESS_KEY_SECRET"))
	if accessKeyID == "" || accessKeySecret == "" {
		// 没有密钥时保留原 URL：公有读 bucket 仍可用；
		// 私有 bucket 会在模型侧失败，再由调用方回退纯文本链路。
		return raw
	}

	client, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		log.Printf("图片问答：创建 OSS 客户端失败，使用原 URL: %v", err)
		return raw
	}
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		log.Printf("图片问答：获取 OSS bucket 失败，使用原 URL: %v", err)
		return raw
	}
	signedURL, err := bucket.SignURL(objectKey, oss.HTTPGet, visionImageURLTTL())
	if err != nil {
		log.Printf("图片问答：生成 OSS 签名 URL 失败，使用原 URL: %v", err)
		return raw
	}
	return signedURL
}

// localUploadDataURI 把本机 ./uploads 下的图片转成 data URI。
// 只要 URL 指向本机/私网，或指向 PUBLIC_BASE_URL 配置的站点根地址，就允许读本地文件。
func localUploadDataURI(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ""
	}
	if !isLocalUploadHost(strings.ToLower(parsed.Hostname())) {
		return ""
	}
	// 兼容 PUBLIC_BASE_URL 带路径前缀的部署，例如 https://example.com/app/uploads/...
	uploadsIndex := strings.Index(parsed.Path, "/uploads/")
	if uploadsIndex < 0 {
		return ""
	}

	cleanPath := filepath.Clean(parsed.Path[uploadsIndex+len("/uploads/"):])
	if cleanPath == "." || strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return ""
	}

	data, err := os.ReadFile(filepath.Join("uploads", cleanPath))
	if err != nil || len(data) == 0 || len(data) > maxVisionImageBytes {
		return ""
	}
	mimeType := http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "image/") {
		return ""
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// isPrivateHost 判断模型无法直接访问的本地/私网地址。
func isPrivateHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

// isLocalUploadHost 判断 URL 是否对应本机 ./uploads：
// 本地/私网地址，或 PUBLIC_BASE_URL 配置的公网站点根地址。
// 例如 PUBLIC_BASE_URL=http://43.142.39.16 时，
// http://43.142.39.16/uploads/... 也会优先读本地文件转 base64。
func isLocalUploadHost(host string) bool {
	if isPrivateHost(host) {
		return true
	}
	baseURL := strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL"))
	if baseURL == "" {
		return false
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(host, parsed.Hostname())
}

// buildVisionConversation 生成图片问诊的用户消息。
// 与原来的 fullConversation 相比，这里不再强制“从照片看，可能是X”开头，
// 而是要求模型先描述图片、再判断；分类器只作为弱提示。
func buildVisionConversation(input AskData, candidateText, structureText, memoryBlock string, imageCount int) string {
	prediction := strings.TrimSpace(input.Prediction)
	if prediction == "" {
		prediction = "无"
	}
	resultSearch := strings.TrimSpace(input.ResultSearch)
	if resultSearch == "" {
		resultSearch = "无"
	}
	descSearch := strings.TrimSpace(input.DescSearch)
	if descSearch == "" {
		descSearch = "无"
	}
	weight := strings.TrimSpace(input.Weight)
	if weight == "" {
		weight = "无"
	}

	return fmt.Sprintf(
		"【本次图片】已附上用户上传的 %d 张照片，请直接观察图片，不要说“我看不到图片”。\n"+
			"【用户问题】%s\n"+
			"【视觉预分析（仅供参考，可能不完整）】%s\n"+
			"【分类器倾向（弱提示，可能出错，不能当作结论）】%s\n"+
			"【分类器候选（按可能性从高到低）】%s\n"+
			"【知识库参考（仅在符合图片时使用，不要生搬硬套）】\n"+
			"预测结果检索：%s\n"+
			"用户描述检索：%s\n"+
			"检索权重：%s\n"+
			"%s"+
			"【历史回答说明】上面的历史对话和记忆仅用于理解上下文，之前的回答可能有误；本次以图片为准，不要复述旧结论。\n"+
			"【回答要求】\n"+
			"1. 先用 2~3 句话描述图片中可见的皮损：部位、数量/分布、边界、颜色、表面（隆起/鳞屑/水疱/结痂）。\n"+
			"2. 再给出倾向性判断，用“可能是”“更像是”；如果还有其他可能，补一句“也可能是”。\n"+
			"3. 分类器只是弱提示；如果图片与分类器冲突，以图片为准，并说明“分类器倾向 X，但视觉上更像 Y”。\n"+
			"4. 如果图片信息不足，明确说“仅凭这张照片难以确定”，列出 1~2 个需要排除的情况和就医建议。\n"+
			"5. 不要提及置信度、概率、百分比；不要描述图片里没有的东西。\n"+
			"6. 急症信号优先；普通回答结尾写“以上仅供参考，不能替代面诊，具体请遵医嘱。”\n",
		imageCount,
		input.Conversation,
		structureText,
		prediction,
		candidateText,
		resultSearch,
		descSearch,
		weight,
		memoryBlock,
	)
}
