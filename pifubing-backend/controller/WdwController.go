package controller

import (
	"SkinDisease/dao"
	"SkinDisease/models"
	"SkinDisease/utils"
	"archive/zip"
	"encoding/base64"
	"sync"

	//"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/dysmsapi"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"io"
	"log"
	"math"
	"math/big"
	"mime"
	"mime/multipart"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// getEnvOrDefault 读取环境变量，为空时返回默认值。
// 用于把 AI 服务地址、端口等部署相关配置从代码里挪到环境变量。
func getEnvOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func GenerateVerificationCode(length int) (string, error) {
	var code string
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code += n.String()
	}
	return code, nil
}

type EmailRequest struct {
	To string `json:"to"`
}

func sendEmail(to string, body string) error {
	// 从环境变量中读取邮件配置信息
	from := os.Getenv("EMAIL_FROM")
	password := os.Getenv("EMAIL_PASSWORD")
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	subject := os.Getenv("EMAIL_SUBJECT")

	// 构造邮件内容：主题按 RFC 2047 编码为 UTF-8，正文声明 UTF-8 字符集，
	// 避免部分邮件客户端把中文标题和验证码显示成乱码。
	message := []byte("From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: 8bit\r\n" +
		"\r\n" + body + "\r\n")

	auth := smtp.PlainAuth("", from, password, smtpHost)

	//发送邮件
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, message)
	if err != nil {
		return err
	}
	return nil
}

func SendEmailHandler(c *gin.Context) {
	var req EmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    100,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 去掉邮箱两端的空格，避免生成 Redis key 时带上空白字符。
	req.To = strings.TrimSpace(req.To)
	if req.To == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": "邮箱不能为空",
		})
		return
	}

	code, err := GenerateVerificationCode(6)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 100,
			"err":  "生成验证码出错",
		})
		return
	}

	// 设置验证码的过期时间为当前时间的五分钟。
	expirationTime := time.Now().Add(5 * time.Minute)

	// 将验证码存入 Redis，并设置过期时间。
	if err := models.Rdb.SetEX(ctx, req.To, code, 5*time.Minute).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 100,
			"err":  "存储验证码失败",
		})
		return
	}

	// 本地开发模式：把验证码打印到后端控制台，方便没有邮件服务时联调。
	if strings.EqualFold(os.Getenv("USE_MEMORY_REDIS"), "true") {
		log.Printf("[开发模式] 邮箱 %s 的验证码：%s", req.To, code)
	}

	// SKIP_EMAIL_SEND=true 时不真正发邮件，只依赖上面的控制台验证码。
	// 默认 false，保持正常发送邮件的行为。
	skipEmail := strings.EqualFold(os.Getenv("SKIP_EMAIL_SEND"), "true")
	if !skipEmail {
		body := "您的注册验证码是：" + code + "\n\n验证码 5 分钟内有效，请尽快完成验证。\n如非本人操作，请忽略此邮件。"
		if err := sendEmail(req.To, body); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 100,
				"err":  "邮件发送失败：" + err.Error(),
			})
			return
		}
	}

	message := "邮件发送成功!"
	if skipEmail {
		message = "开发模式：验证码已生成，请查看后端控制台"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":           200,
		"message":        message,
		"expirationTime": expirationTime.Format(time.RFC3339),
	})
}

var ctx = context.Background()

func VerifyCode(to string, code string) (bool, error) {
	//从Redis获取存储的验证码
	storedCode, err := models.Rdb.Get(ctx, to).Result()
	if err == redis.Nil {
		return false, fmt.Errorf("验证码不存在或已经过期")
	} else if err != nil {
		return false, fmt.Errorf("验证码不正确")
	}

	//检查验证码是否正确
	if storedCode != code {
		return false, fmt.Errorf("验证码不正确")
	}

	//验证成功，删除验证码
	err = models.Rdb.Del(ctx, to).Err()
	if err != nil {
		return false, fmt.Errorf("删除验证码失败：%v", err)
	}

	return true, nil
}

type RegisterInput struct {
	Account      string `json:"account"`
	Password     string `json:"password"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Age          int    `json:"age"`
	Gender       string `json:"gender"`
	Phone_number string `json:"phone_number"`
	Code         string `json:"code"`
}

func Register(c *gin.Context) {
	var input RegisterInput

	// 绑定表单数据；失败必须 return，否则后面会继续执行并返回两段 JSON。
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": err.Error(),
		})
		return
	}

	// 去掉账号、邮箱等字段两端的空格，密码不做 trim（密码中的空格可能是有意的）。
	input.Account = strings.TrimSpace(input.Account)
	input.Email = strings.TrimSpace(input.Email)
	input.Name = strings.TrimSpace(input.Name)
	input.Code = strings.TrimSpace(input.Code)

	if input.Account == "" || input.Password == "" || input.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": "账号、密码和邮箱不能为空",
		})
		return
	}

	// 密码强度：至少 6 位
	if len(input.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": "密码长度至少为 6 位",
		})
		return
	}

	// 获取验证码
	code := input.Code
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": "验证码不为空",
		})
		return
	}

	// 先检查账号是否已存在，避免重复注册。
	var existing models.User
	existing.Account = input.Account
	if err := models.CheckAccount(dao.DB, &existing); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": "账号已存在",
		})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "查询账号失败",
		})
		return
	}

	// 验证验证码是否正确；验证成功后会删除 Redis 中的验证码。
	if _, err := VerifyCode(input.Email, code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": err.Error(),
		})
		return
	}

	// 密码加密；加密失败必须 return，不能继续用空密码注册。
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "密码加密失败",
		})
		return
	}

	user := models.User{
		Account:      input.Account,
		Password:     string(hashedPassword),
		Email:        input.Email,
		Name:         input.Name,
		Age:          input.Age,
		Gender:       input.Gender,
		Phone_number: input.Phone_number,
	}

	// 创建用户；数据库失败必须 return。
	if err := models.CreateUser(dao.DB, &user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法注册用户",
		})
		return
	}

	// 成功响应
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "用户注册成功"})
}

type LoginInput struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

// JwtKey 由 InitJWTKey 在启动时设置，不再硬编码固定密钥。
var JwtKey []byte

// InitJWTKey 在 main 加载完 .env 之后调用，用环境变量设置签名密钥。
// 不能放在 init() 里，因为 init() 执行时 .env 还没有被 godotenv 读取。
func InitJWTKey() {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret != "" {
		JwtKey = []byte(secret)
		return
	}

	// 未配置密钥时随机生成，避免使用代码里的固定弱密钥。
	randomKey := make([]byte, 32)
	if _, err := rand.Read(randomKey); err == nil {
		JwtKey = randomKey
		log.Println("警告: 未配置 JWT_SECRET，已生成临时密钥；重启后已有 token 会失效")
		return
	}
	log.Println("警告: 未配置 JWT_SECRET 且随机密钥生成失败，请尽快在 .env 中配置")
}

type Claims struct {
	Account string `json:"account"`
	ID      int    `json:"id"`
	jwt.StandardClaims
}

func Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":  100,
			"error": "当前用户没有注册",
		})
		return
	}

	//验证用户是否存在
	var user models.User
	user.Account = input.Account
	if err := models.CheckAccount(dao.DB, &user); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":  100,
			"error": "当前用户没有注册",
		})
		return
	}

	//验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":  100,
			"error": "账号或密码错误",
		})
		return
	}

	//生成jwt token
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Account: input.Account,
		ID:      user.ID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(JwtKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": "无法生成token"})
		return
	}
	// 前端登录后会把 user_id 存进 localStorage，后续查资料/聊天都用它，必须一起返回。
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登录成功",
		"token":   tokenString,
		"user_id": user.ID,
	})
}

type Forget_password_input struct {
	Account  string `json:"account"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Code     string `json:"code"`
}

func Forget_password(c *gin.Context) {
	var input Forget_password_input

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 100, "error": err.Error()})
		return
	}

	// 验证验证码是否正确
	_, err := VerifyCode(input.Email, input.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": err.Error()})
		return
	}

	// 密码强度：至少 6 位
	if len(input.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 100, "error": "密码长度至少为 6 位"})
		return
	}

	// 密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": err.Error()})
		return
	}

	if err := dao.DB.Model(&models.User{}).Where("account = ?", input.Account).Update("password", hashedPassword).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "修改成功"})
}

func Logout(c *gin.Context) {
	tokenString := c.GetHeader("Authorization")

	//移除‘Bearer'
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 100, "error": "token为空"})
		return
	}
	err := models.AddToBlacklist(tokenString)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": "无法将token拉入黑名单"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "登出成功, token失效"})
}

// 解析JWT并返回userID
func ParseToken(tokenString string) (int, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return JwtKey, nil
	})
	if err != nil {
		return 0, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID, ok := claims["id"].(float64)
		if !ok {
			return 0, errors.New("invalid token claims")
		}

		if err != nil {
			return 0, err
		}

		return (int(userID)), nil
	}

	return 0, errors.New("invalid token")
}

// ValidateToken 只负责解析 token，不直接写响应。
// 由调用方统一返回 401，避免 ValidateToken 和 handler 各写一次 JSON。
func ValidateToken(c *gin.Context) (int, error) {
	// 获取 Authorization 头部
	tokenString := c.GetHeader("Authorization")
	//去除“Bearer”
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	if tokenString == "" {
		return 0, fmt.Errorf("token为空")
	}

	userID, err := ParseToken(tokenString)
	if err != nil {
		return 0, fmt.Errorf("token无效或已过期")
	}

	return userID, nil
}

// UploadToOSS 上传文件并返回可公开访问的 URL。
// STORAGE_MODE=local 时改为保存到本机 ./uploads 目录，适合没有 OSS 的本地开发/演示环境。
func UploadToOSS(localFilePath string, ossPath string) (string, error) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("STORAGE_MODE")), "local") {
		return saveFileLocally(localFilePath, ossPath)
	}

	// 从环境变量读取配置
	OSSEndpoint := os.Getenv("OSS_ENDPOINT")
	OSSAccessKeyID := os.Getenv("OSS_ACCESS_KEY_ID")
	OSSAccessKeySecret := os.Getenv("OSS_ACCESS_KEY_SECRET")
	OSSBucketName := os.Getenv("OSS_BUCKET_NAME")

	// 创建OSS客户端
	client, err := oss.New(OSSEndpoint, OSSAccessKeyID, OSSAccessKeySecret)
	if err != nil {
		return "", fmt.Errorf("创建OSS客户端失败: %v", err)
	}

	// 获取存储桶
	bucket, err := client.Bucket(OSSBucketName)
	if err != nil {
		return "", fmt.Errorf("获取存储桶失败: %v", err)
	}

	// 上传文件到OSS，使用带有用户名和文件名的路径
	err = bucket.PutObjectFromFile(ossPath, localFilePath)
	if err != nil {
		return "", fmt.Errorf("文件上传失败: %v", err)
	}

	// 获取文件的URL
	url := fmt.Sprintf("https://%s.%s/%s", OSSBucketName, OSSEndpoint, ossPath)

	return url, nil
}

// saveFileLocally 把上传文件复制到 ./uploads/<ossPath>，并返回网关上的静态访问地址。
// ossPath 形如 "2/image/xxx.png"，其中可能带用户输入的文件名，必须清洗后再拼接。
func saveFileLocally(localFilePath string, ossPath string) (string, error) {
	// 去掉前导分隔符和 ../，避免路径穿越到 uploads 目录之外。
	cleanPath := strings.TrimLeft(filepath.ToSlash(ossPath), "/")
	cleanPath = filepath.Clean(cleanPath)
	if cleanPath == "." || strings.HasPrefix(cleanPath, "..") {
		return "", fmt.Errorf("非法的存储路径: %s", ossPath)
	}

	destPath := filepath.Join("uploads", cleanPath)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return "", fmt.Errorf("创建上传目录失败: %v", err)
	}

	src, err := os.Open(localFilePath)
	if err != nil {
		return "", fmt.Errorf("打开临时文件失败: %v", err)
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("创建目标文件失败: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("保存文件失败: %v", err)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "http://localhost:9094"
	}
	return fmt.Sprintf("%s/uploads/%s", baseURL, filepath.ToSlash(cleanPath)), nil
}

func CallLargeModelAPIForImageAndZip(filePath string, text string) (string, error) {
	// 创建一个新的 multipart/form-data 请求
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if filePath != "" {
		// 打开文件
		file, err := os.Open(filePath)
		if err != nil {
			return "", fmt.Errorf("无法打开文件: %v", err)
		}
		defer file.Close()

		// 检查文件类型
		buffer := make([]byte, 512)
		if _, err := file.Read(buffer); err != nil {
			return "", fmt.Errorf("读取文件头失败: %v", err)
		}
		fileType := http.DetectContentType(buffer)
		if !strings.HasPrefix(fileType, "image/") && !strings.HasPrefix(fileType, "application/zip") {
			return "", fmt.Errorf("文件类型不支持: %s", fileType)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return "", fmt.Errorf("重置文件指针失败: %v", err)
		}

		// 无论文件类型是图片还是压缩包，都使用 "image" 作为字段名
		fieldName := "image"

		// 写入文件部分
		part, err := writer.CreateFormFile(fieldName, filepath.Base(filePath))
		if err != nil {
			return "", fmt.Errorf("创建文件部分失败: %v", err)
		}
		if _, err := io.Copy(part, file); err != nil {
			return "", fmt.Errorf("写入文件数据失败: %v", err)
		}
	}

	// 写入文字字段
	if err := writer.WriteField("conversation", text); err != nil {
		return "", fmt.Errorf("写入文字字段失败: %v", err)
	}

	// 结束 multipart 数据写入
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭 multipart writer 失败: %v", err)
	}

	// 图片预测服务地址：优先读环境变量，未配置时保持原来的远程地址。
	apiURL := getEnvOrDefault("AI_IMAGE_API_URL", "http://8.137.81.23:8001/analyze")

	// 创建 HTTP POST 请求
	req, err := http.NewRequest("POST", apiURL, &body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 发送请求
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body) // 读取响应内容以便调试
		return "", fmt.Errorf("大模型接口返回错误: %v, 响应内容: %s", resp.Status, string(responseBody))
	}

	// 读取响应内容
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析并返回大模型的响应
	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return "", fmt.Errorf("解析响应数据失败: %v", err)
	}

	result, _ := json.MarshalIndent(response, "", "  ") // 格式化输出
	return string(result), nil
}

func CallLargeModelAPIForText(conversation ConversationData) (string, error) {
	// 创建一个新的 multipart/form-data 请求
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// 写入各个字段
	if err := writer.WriteField("conversation", conversation.Conversation); err != nil {
		return "", fmt.Errorf("写入对话上下文失败: %v", err)
	}
	if err := writer.WriteField("predicted_class", conversation.PredictedClass); err != nil {
		return "", fmt.Errorf("写入预测类别失败: %v", err)
	}
	if err := writer.WriteField("confidence", fmt.Sprintf("%f", conversation.Confidence)); err != nil {
		return "", fmt.Errorf("写入置信度失败: %v", err)
	}
	// 历史上下文单独提交，Python 只把它放进 system prompt，不用于知识库检索。
	if err := writer.WriteField("memory_context", conversation.MemoryContext); err != nil {
		return "", fmt.Errorf("写入历史上下文失败: %v", err)
	}

	// 结束 multipart 数据写入
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭 multipart writer 失败: %v", err)
	}

	// 文字诊断服务地址：优先读环境变量，未配置时保持原来的远程地址。
	apiURL := getEnvOrDefault("AI_TEXT_API_URL", "http://8.137.81.23:8001/process_text")

	// 创建 HTTP POST 请求
	req, err := http.NewRequest("POST", apiURL, &body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 发送请求
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("大模型接口返回错误: %v", resp.Status)
	}

	// 读取响应内容
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析并返回大模型的响应
	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return "", fmt.Errorf("解析响应数据失败: %v", err)
	}

	// 格式化输出
	result, _ := json.MarshalIndent(response, "", "  ")
	return string(result), nil
}

// chatExists 判断对话是否仍属于该用户。
// 回答生成可能持续几十秒，期间用户可能已经删除对话；
// 保存前再确认一次，避免把完整回答写进一个已删除的对话，留下孤儿记录。
func chatExists(chatID, userID int) bool {
	if chatID <= 0 {
		return false
	}
	var count int64
	if err := dao.DB.Model(&models.Chat{}).
		Where("id = ? AND user_id = ?", chatID, userID).
		Count(&count).Error; err != nil {
		log.Printf("检查对话是否存在失败: %v", err)
		return false
	}
	return count > 0
}

// 使用异步处理,减少前端响应时间
func saveImageMessageAsync(saveResponse *models.Response) {
	go func() {
		// 对话在回答生成期间被删除时不再保存，避免留下孤儿记录。
		if !chatExists(saveResponse.Chat_id, saveResponse.User_id) {
			log.Printf("对话 %d 不存在或已删除，跳过保存问诊记录", saveResponse.Chat_id)
			return
		}
		//异步保存数据到数据库
		if dbErr := models.SaveToDataBase(dao.DB, saveResponse); dbErr != nil {
			log.Printf("保存到数据库失败: %v", dbErr)
		} else {
			log.Println("保存成功！")
		}
	}()
}

type ConversationData struct {
	Conversation   string  `json:"conversation"`
	PredictedClass string  `json:"predicted_class"`
	Confidence     float64 `json:"confidence"`
	// MemoryContext 是同一对话最近问答 + 用户长期记忆，仅供大模型参考。
	// 它不参与 Python 的 FAISS 检索，避免历史内容把知识库检索带偏。
	MemoryContext string `json:"memory_context"`
}

// ModelResponse 结构体定义
type ModelResponse struct {
	GeneratedText  string  `json:"generated_text"` // 修改为 string 类型
	PredictedClass string  `json:"predicted_class"`
	Confidence     float64 `json:"confidence"`
	Status         string  `json:"status"`
}

//func PredictImage(c *gin.Context) {
//	userID, err := ValidateToken(c)
//	if err != nil {
//		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
//		return
//	}
//	var localFilePath string
//	var url string
//	var baseName string
//	file, err := c.FormFile("file")
//	if err == nil {
//		// 获取文件名（不带路径和扩展名）
//		fileName := file.Filename
//		// 去除文件扩展名
//		ext := filepath.Ext(fileName)
//		baseName = fileName[:len(fileName)-len(ext)]
//
//		// 保存上传的文件到本地
//		localFilePath = "./temp/" + fileName
//
//		// 保存上传的文件到本地
//		if err := c.SaveUploadedFile(file, localFilePath); err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("保存文件失败: %v", err)})
//			return
//		}
//
//		// 判断是否是一个压缩包
//		if ext == ".zip" {
//			// 解压文件
//			destPath := "./temp/unzipped"
//			err = unzipFile(localFilePath, destPath)
//			if err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{"code": 100,
//					"error": fmt.Sprintf("解压文件失败: %v", err),
//				})
//				return
//			}
//
//			ossPath := fmt.Sprintf("%s/zip/%s", strconv.Itoa(userID), fileName)
//
//			fileUrl, err := UploadToOSS(localFilePath, ossPath)
//			if err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{
//					"code":  100,
//					"error": fmt.Sprintf("文件上传到OSS失败: %v", err),
//				})
//				return
//			}
//			url = fileUrl
//		} else {
//			// 如果上传的文件是单个图片，直接上传到 OSS
//			// 上传图片到 OSS
//			ossPath := fmt.Sprintf("%s/image/%s", strconv.Itoa(userID), fileName)
//			fileUrl, err := UploadToOSS(localFilePath, ossPath)
//			if err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{
//					"code":  100,
//					"error": fmt.Sprintf("图片上传到OSS失败: %v", err),
//				})
//				return
//			}
//			fmt.Println(222)
//			url = fileUrl
//		}
//	} else {
//		// 如果未上传文件，记录空的文件路径和URL
//		localFilePath = ""
//		url = ""
//	}
//	fmt.Println(baseName)
//
//	// 调用大模型接口进行图片预测
//	modelResponse, modelErr := CallLargeModelAPIForImageAndZip(localFilePath)
//	if modelErr != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("调用图片大模型接口失败：%v", modelErr)})
//		return
//	}
//
//	// 解析图片预测结果
//	var response ModelResponse
//	err = json.Unmarshal([]byte(modelResponse), &response)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "解析图片预测响应失败"})
//		return
//	}
//
//	// 获取预测结果
//	predictedClass := response.PredictedClass
//	confidence := response.Confidence
//	confidence = confidence * 100
//	confidence = math.Round(confidence*100) / 100
//	confidenceString := fmt.Sprintf("%.2f%%", confidence)
//
//	// 返回图片预测结果
//	c.JSON(http.StatusOK, gin.H{
//		"code":                200,
//		"imagePredictedClass": predictedClass,
//		"imageConfidence":     confidenceString,
//		"imageUrl":            url,
//	})
//
//	//删除上传的文件
//	if localFilePath != "" {
//		err := os.Remove(localFilePath)
//		if err != nil {
//			fmt.Printf("删除上传文件失败: %v\n", err)
//		}
//	}
//}

type Data struct {
	Chat_id        int     `json:"chat_id"`
	User_id        int     `json:"user_id"`
	Conversation   string  `json:"conversation"`
	PredictedClass string  `json:"predicted_class"`
	Confidence     float64 `json:"confidence"`
	Url            string  `json:"url"`
}

func Create_image_ask_messages(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	var input Data
	input.User_id = userID

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": err.Error(),
		})
		return
	}

	// 查询用户信息
	var user models.Disease_information
	if err := dao.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": 100, "error": "用户信息未找到"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "查询用户信息失败"})
		}
		return
	}

	conversationKey := fmt.Sprintf("conversation:%d", userID)

	// 获取整个列表
	history, err := models.Rdb.LRange(ctx, conversationKey, -2, -1).Result()
	if err != nil {
		log.Println("Failed to retrieve conversation history:", err)
	}
	//fmt.Println(history)

	var historyText string
	if len(history) > 0 {
		historyText = strings.Join(history, "\n") + "\n"
	}
	fmt.Println(historyText)
	// 拼接 medical_history 和 allergies
	fullConversation := "Q: " + input.Conversation
	fullConversation += "\n我的患病史: " + user.Medical_history
	fullConversation += "\n过敏情况: " + user.Allergies

	// 构造对话结构体
	conversationData := ConversationData{
		Conversation:   fullConversation,
		PredictedClass: input.PredictedClass,
		Confidence:     input.Confidence,
	}

	// 调用大模型接口
	modelResponseForText, modelErr := CallLargeModelAPIForText(conversationData)
	if modelErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("调用文字描述大模型接口失败：%v", modelErr)})
		return
	}

	// 解析大模型返回的结果
	var responseForText ModelResponse
	err = json.Unmarshal([]byte(modelResponseForText), &responseForText)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "解析文字描述响应失败"})
		return
	}

	// 去除 JSON 格式标记
	cleanedText := strings.Replace(responseForText.GeneratedText, "\n\n```json\n", "", -1)
	cleanedText = strings.Replace(cleanedText, "\n```\n", "", -1)

	// 返回大模型的结果
	c.JSON(http.StatusOK, gin.H{
		"code":           200,
		"PredictedClass": responseForText.PredictedClass,
		"GeneratedText":  responseForText.GeneratedText,
		"Confidence":     fmt.Sprintf("%.2f", responseForText.Confidence),
	})

	// 保存到数据库
	saveResponse := models.Response{
		Chat_id:    input.Chat_id,
		User_id:    userID,
		Ask:        input.Conversation,
		Materials:  input.Url,
		Type:       responseForText.PredictedClass,
		Message:    responseForText.GeneratedText,
		Confidence: fmt.Sprintf("%.2f", responseForText.Confidence),
	}
	saveImageMessageAsync(&saveResponse)

	// 存储对话内容到 Redis
	//redisKey := fmt.Sprintf("conversation:%d", userID)
	//redisValue := fmt.Sprintf("%s|%s|%s", input.Conversation, input.PredictedClass, modelResponseForText)

	// 将本次问题和回答存储到 Redis
	currentMessage := fmt.Sprintf("Q: %s\nA: %s", input.Conversation, modelResponseForText)
	err = models.Rdb.RPush(ctx, conversationKey, currentMessage).Err() // 将对话内容推入到 Redis 列表中
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "Failed to save conversation"})
		return
	}
	// 可选：限制 Redis 列表的最大长度，例如保持最近的 10 条对话
	err = models.Rdb.LTrim(ctx, conversationKey, -2, -1).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "Failed to trim conversation history"})
	}
	return

}

type Check_image_ask_messages_input struct {
	User_id int `json:"user_id"`
	Chat_id int `json:"chat_id"`
	Page    int `json:"page"`
	Size    int `json:"size"`
}

func Check_image_ask_messages(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	//手动提取查询参数
	page, _ := strconv.Atoi(c.Query("page"))
	size, _ := strconv.Atoi(c.Query("size"))
	chat_id, _ := strconv.Atoi(c.Query("chat_id"))

	// 未传分页参数时给默认值，避免 total_page 出现除以 0 的异常结果。
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	input := Check_image_ask_messages_input{
		User_id: userID,
		Chat_id: chat_id,
		Page:    page,
		Size:    size,
	}
	// 调用 ImageAskMessages 函数查询聊天记录
	ImageAskMessages, total, err := models.CheckImageAskMessages(dao.DB, (*models.Check_image_ask_messages_input)(&input))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法查看个人聊天框历史记录: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "查看个人聊天框历史记录成功",
		"data":    ImageAskMessages, // 返回查询到的聊天记录
		"pagination": gin.H{
			"page":       input.Page,
			"page_size":  input.Size,
			"total":      total,                                                // 总记录数
			"total_page": int(math.Ceil(float64(total) / float64(input.Size))), // 总页数
		},
	})
}

type Chat_input struct {
	ID      int    `json:"id"`
	User_id int    `json:"user_id"`
	Name    string `json:"name"`
}

func Create_chat(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	var input Chat_input

	input.User_id = userID

	// 绑定表单数据
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": err.Error(),
		})
		return
	}

	if err := models.CreateChat(dao.DB, (*models.Chat)(&input)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法创建个人聊天框",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "创建个人聊天框成功",
		"chat_id": input.ID,
	})

}

type Check_Chat_input struct {
	User_id int `json:"user_id"`
	Page    int `json:"page"`
	Size    int `json:"size"`
}

func Check_chat(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	//手动提取查询参数
	page, _ := strconv.Atoi(c.Query("page"))
	size, _ := strconv.Atoi(c.Query("size"))
	userIDParam, _ := strconv.Atoi(c.Query("user_id"))

	input := Check_Chat_input{
		User_id: userIDParam,
		Page:    page,
		Size:    size,
	}

	// 检查分页参数是否合法
	if input.Page < 1 || input.Size < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": "分页参数无效",
		})
		return
	}

	input.User_id = userID
	//var response models.Chat
	// 调用 CheckChat 函数查询聊天记录
	chats, total, err := models.CheckChat(dao.DB, input.User_id, input.Page, input.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法查看个人聊天框: " + err.Error(),
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "查看个人聊天框成功",
		"data":    chats, // 返回查询到的聊天记录
		"pagination": gin.H{
			"page":       input.Page,
			"page_size":  input.Size,
			"total":      total,                                                // 总记录数
			"total_page": int(math.Ceil(float64(total) / float64(input.Size))), // 总页数
		},
	})
}

func Update_chat(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	var input Chat_input

	input.User_id = userID

	// 绑定表单数据
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": err.Error(),
		})
		return
	}

	if err := models.UpdateChat(dao.DB, (*models.Chat)(&input)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法修改个人聊天框名称",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "修改个人聊天框名称成功",
	})

}

type AutoTitleChatInput struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Auto_title_chat 在用户发出首条问题后，把默认标题“新对话”改成问题摘要。
// 只在数据库中的标题仍为默认值时更新，用户手动改过标题不会被覆盖。
func Auto_title_chat(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	var input AutoTitleChatInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 100, "error": err.Error()})
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	if input.ID <= 0 || input.Name == "" || len([]rune(input.Name)) > 30 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 100, "error": "标题参数无效"})
		return
	}

	updated, err := models.UpdateChatNameIfDefault(dao.DB, input.ID, userID, input.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "自动更新对话标题失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "自动标题处理完成",
		"updated": updated,
	})
}

type Delete_Chat_input struct {
	User_id int `json:"user_id"`
	Chat_id int `json:"chat_id"`
}

func Delete_chat(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	//手动提取查询参数
	chat_id, _ := strconv.Atoi(c.Query("chat_id"))
	if chat_id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": "chat_id 无效",
		})
		return
	}

	input := Delete_Chat_input{
		User_id: userID,
		Chat_id: chat_id,
	}

	// 绑定表单数据
	//if err := c.ShouldBindJSON(&input); err != nil {
	//	c.JSON(http.StatusBadRequest, gin.H{
	//		"code":  100,
	//		"error": err.Error(),
	//	})
	//	return
	//}

	deleted, err := models.DeleteChat(dao.DB, (*models.Delete_Chat_input)(&input))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法删除个人聊天框",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "删除个人聊天框成功",
		"deleted": deleted,
	})

}

// 清理当前用户名下没有任何消息的空对话。
func Delete_empty_chats(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	deletedIDs, err := models.DeleteEmptyChats(dao.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法清理空对话: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":        200,
		"message":     "清理空对话成功",
		"deleted":     len(deletedIDs),
		"deleted_ids": deletedIDs,
	})
}

// 发送短信验证码的函数。
// AccessKey 等敏感信息改为从环境变量读取，避免把密钥写死在代码里。
func sendSms(phoneNumber, code string) error {
	accessKeyID := os.Getenv("SMS_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("SMS_ACCESS_KEY_SECRET")
	signName := os.Getenv("SMS_SIGN_NAME")
	templateCode := os.Getenv("SMS_TEMPLATE_CODE")

	client, err := dysmsapi.NewClientWithAccessKey("cn-hangzhou", accessKeyID, accessKeySecret)
	if err != nil {
		return err
	}

	request := dysmsapi.CreateSendSmsRequest()
	request.Scheme = "https"
	request.PhoneNumbers = phoneNumber
	request.SignName = signName
	request.TemplateCode = templateCode
	request.TemplateParam = fmt.Sprintf("{\"code\":\"%s\"}", code)

	// 发送请求
	response, err := client.SendSms(request)
	if err != nil {
		return err
	}

	// 检查响应
	if response.Code != "OK" {
		return fmt.Errorf("failed to send SMS: %s", response.Message)
	}

	return nil
}

func SMS_verification(c *gin.Context) {
	var Number struct {
		PhoneNumber string `json:"phone_number"`
	}

	if err := c.ShouldBindJSON(&Number); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	code, err := GenerateVerificationCode(6)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 100,
			"err":  "生成验证码出错",
		})
		return
	}

	//将验证码存入redis,并设置过期时间
	err = models.Rdb.SetEX(ctx, Number.PhoneNumber, code, 5*time.Minute).Err()

	if err != nil {
		c.JSON(500, gin.H{
			"code": 100,
			"err":  "存储验证码失败",
		})
		return
	}

	// 调用发送短信函数
	err = sendSms(Number.PhoneNumber, code)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "手机短信验证码发送成功，请在5分钟内验证。"})
}

type Users_avatar struct {
	Avatar_url string `json:"avatar_url"`
}

func Updates_avatar(c *gin.Context) {
	var input Users_avatar
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	var localFilePath string
	var url string

	file, err := c.FormFile("avatar")
	// 获取文件名（不带路径和扩展名）
	fileName := file.Filename
	if err == nil {
		// 如果上传了文件，处理文件逻辑
		localFilePath = "./avatar_temp/" + file.Filename

		// 保存上传的文件到本地
		if err := c.SaveUploadedFile(file, localFilePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("保存文件失败: %v", err)})
			return
		}
		ossPath := fmt.Sprintf("%s/avatar/%s", strconv.Itoa(userID), fileName)
		//上传文件到OSS
		url, err = UploadToOSS(localFilePath, ossPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("文件上传到OSS失败: %v", err)})
			return
		}
	} else {
		// 如果未上传文件，记录空的文件路径和URL
		localFilePath = ""
		url = ""
	}

	input.Avatar_url = url

	if err := models.Updates_users_avatar(dao.DB, (*models.Users_avatar)(&input), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法更新头像框",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "修改头像框成功",
		"data":    url,
	})

}

type Users_all_information struct {
	Account         string `json:"account"`
	Email           string `json:"email"`
	Name            string `json:"name"`
	Age             int    `json:"age"`
	Gender          string `json:"gender"`
	Phone_number    string `json:"phone_number"`
	Medical_history string `json:"medical_history"`
	Allergies       string `json:"allergies"`
	Avatar_url      string `json:"avatar_url"`
}

func Check_users_information(c *gin.Context) {
	var input Users_all_information
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	if err := models.Check_users_information(dao.DB, (*models.Users_all_information)(&input), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法查询个人信息",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "查询个人信息成功",
		"data":    input,
	})

}

type Users_information struct {
	Account      string `json:"account"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Age          int    `json:"age"`
	Gender       string `json:"gender"`
	Phone_number string `json:"phone_number"`
}

func Updates_users_information(c *gin.Context) {
	var input Users_information
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	//绑定表单数据
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": err.Error(),
		})
		return
	}

	if err := models.Update_users_information(dao.DB, (*models.Users_information)(&input), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法更新个人信息",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "修改更新个人信息成功",
		"data":    input,
	})
}

type Disease_information struct {
	Medical_history string `json:"medical_history"`
	Allergies       string `json:"allergies"`
}

func Create_disease_information(c *gin.Context) {
	var input Disease_information
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	//绑定表单数据
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": err.Error(),
		})
		return
	}

	if err := models.Update_disease_information(dao.DB, (*models.Disease_information)(&input), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法创建个人信息病史，过敏情况",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "创建个人信息病史，过敏情况成功",
		"data":    input,
	})

}

type Users_history struct {
	User_id    int    `json:"user_id"`
	Type       string `json:"type"`
	Materials  string `json:"url"`
	Ask        string `json:"ask"`
	Message    string `json:"message"`
	Confidence string `json:"confidence"`
}

func Check_users_history(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	history, err := models.Check_users_history(dao.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法查询个人信息",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "查询个人信息成功",
		"data":    history,
	})
}

// healthArchiveHistoryLimit 限制参与生成档案的问诊条数（最多 12 条）。
// 12 条已经足够概括近期情况，也能避免提示词过长、模型耗时过久。
const healthArchiveHistoryLimit = 12

// truncateForArchive 按字符（而不是字节）截断文本，避免中文被切成乱码。
func truncateForArchive(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

// buildArchiveProfile 把用户填写的个人情况拼成一段文字，交给大模型整理。
func buildArchiveProfile(info Users_all_information) string {
	// 建档日期让档案带有明确的时间信息，便于用户后续对照复诊。
	parts := make([]string, 0, 6)
	parts = append(parts, "建档日期："+time.Now().Format("2006-01-02"))
	if name := strings.TrimSpace(info.Name); name != "" {
		parts = append(parts, "姓名："+truncateForArchive(name, 30))
	}
	if info.Age > 0 {
		parts = append(parts, fmt.Sprintf("年龄：%d 岁", info.Age))
	}
	if gender := strings.TrimSpace(info.Gender); gender != "" {
		parts = append(parts, "性别："+truncateForArchive(gender, 10))
	}
	if medicalHistory := strings.TrimSpace(info.Medical_history); medicalHistory != "" {
		parts = append(parts, "皮肤病/既往病史："+truncateForArchive(medicalHistory, 500))
	}
	if allergies := strings.TrimSpace(info.Allergies); allergies != "" {
		parts = append(parts, "过敏史："+truncateForArchive(allergies, 500))
	}
	return strings.Join(parts, "\n")
}

// GetHealthArchive 根据当前用户的个人皮肤病情况和最近问诊记录，
// 调用 Python 大模型生成一份 Markdown 健康档案。
func GetHealthArchive(c *gin.Context) {
	// 档案会随个人信息和问诊记录变化，禁止浏览器/代理缓存旧结果。
	c.Header("Cache-Control", "no-store")

	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	var info Users_all_information
	if err := models.Check_users_information(dao.DB, (*models.Users_all_information)(&info), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法查询个人信息",
		})
		return
	}

	history, err := models.Check_recent_users_history(dao.DB, userID, healthArchiveHistoryLimit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法查询历史问诊记录",
		})
		return
	}

	profile := buildArchiveProfile(info)
	historyParts := make([]string, 0, len(history))
	// 表里没有时间字段，按数据库返回顺序拼接；顺序不影响档案归纳结果。
	for _, item := range history {
		ask := strings.TrimSpace(item.Ask)
		message := strings.TrimSpace(item.Message)
		if ask == "" && message == "" {
			continue
		}
		label := strings.TrimSpace(item.Type)
		if label == "" {
			label = "皮肤问诊"
		}
		historyParts = append(historyParts, fmt.Sprintf(
			"【%s】\n用户描述：%s\nAI 建议：%s",
			label,
			truncateForArchive(ask, 350),
			truncateForArchive(message, 350),
		))
	}
	historyText := strings.Join(historyParts, "\n\n")
	// 明确告诉模型记录条数，便于在档案概览中说明数据来源。
	if len(historyParts) > 0 {
		historyText = fmt.Sprintf("共 %d 条问诊记录：\n\n%s", len(historyParts), historyText)
	}

	// 只有姓名、年龄等基础资料、但没有皮肤病情况和问诊记录时，
	// 生成的档案没有实际意义，直接返回空内容，不调用大模型。
	hasSkinInfo := strings.TrimSpace(info.Medical_history) != "" ||
		strings.TrimSpace(info.Allergies) != ""
	if !hasSkinInfo && historyText == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "暂无可生成健康档案的数据",
			"data": gin.H{
				"archive":      "",
				"record_count": 0,
			},
		})
		return
	}

	archive, err := utils.GenerateHealthArchive(c.Request.Context(), profile, historyText)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"code":  500,
			"error": "AI 健康档案生成失败，请稍后重试",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "生成健康档案成功",
		"data": gin.H{
			"archive":      archive,
			"record_count": len(historyParts),
		},
	})
}

//func Oss(c *gin.Context) {
//	//保存每张地图的网络地址
//	var urls []string
//
//	// 获取上传的压缩文件
//	file, err := c.FormFile("file")
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"code":  100,
//			"error": fmt.Sprintf("获取文件失败：%v", err),
//		})
//		return
//	}
//
//	// 获取文件名（不带路径和扩展名）
//	fileName := file.Filename
//	// 去除文件扩展名
//	ext := filepath.Ext(fileName)
//	baseName := fileName[:len(fileName)-len(ext)]
//
//	// 保存上传的文件到本地
//	localFilePath := "./temp/" + fileName
//	if err := c.SaveUploadedFile(file, localFilePath); err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"code":  100,
//			"error": fmt.Sprintf("保存文件失败：%v", err),
//		})
//		return
//	}
//
//	// 解压文件
//	destPath := "./temp/unzipped"
//	err = unzipFile(localFilePath, destPath)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"code":  100,
//			"error": fmt.Sprintf("解压文件失败: %v", err),
//		})
//		return
//	}
//	fmt.Println(destPath)
//	destPath = "./temp/unzipped/" + baseName
//	fmt.Println(destPath)
//	// 遍历解压后的文件
//	files, err := os.ReadDir(destPath)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"code":  100,
//			"error": fmt.Sprintf("读取解压文件失败: %v", err),
//		})
//		return
//	}
//	for _, file := range files {
//		if file.IsDir() {
//			continue // 忽略文件夹
//		}
//
//		localFilePath := filepath.Join(destPath, file.Name())
//		url, err := uploadToOSS(localFilePath)
//		if err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{
//				"code":  100,
//				"error": fmt.Sprintf("文件上传到OSS失败：%v", err),
//			})
//			return
//		}
//		urls = append(urls, url)
//	}
//
//	urlsFilePath := "./uploaded_urls.txt"
//	err = writeUrlsToFile(urls, urlsFilePath)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"code":  100,
//			"error": fmt.Sprintf("写入文本文件失败: %v", err),
//		})
//		return
//	}
//
//	textFileUrl, err := uploadToOSS(urlsFilePath)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"code":  100,
//			"error": fmt.Sprintf("文本文件上传到OSS失败: %v", err)})
//	}
//
//	// 返回文本文件的 URL
//	c.JSON(http.StatusOK, gin.H{
//		"code":    200,
//		"message": "文件上传成功，URLs 已保存到文本文件",
//		"data":    textFileUrl,
//	})
//
//	// 删除临时文件夹及其内容
//	err = os.RemoveAll("./temp")
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"code":  100,
//			"error": fmt.Sprintf("删除临时文件夹失败: %v", err),
//		})
//		return
//	}
//}

// 解压文件
func unzipFile(src, dest string) error {
	// 打开压缩文件
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("打开压缩文件失败: %v", err)
	}
	defer r.Close()

	// 创建目标文件夹
	err = os.MkdirAll(dest, os.ModePerm)
	if err != nil {
		return fmt.Errorf("创建目标文件夹失败: %v", err)
	}

	// 遍历并解压每个文件
	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			// 如果是文件夹，创建文件夹
			err := os.MkdirAll(fpath, f.Mode())
			if err != nil {
				return fmt.Errorf("创建文件夹失败: %v", err)
			}
			continue
		}

		// 如果是文件，解压文件
		err := func() error {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			// 创建目标文件
			outFile, err := os.Create(fpath)
			if err != nil {
				return err
			}
			defer outFile.Close()

			// 将内容复制到目标文件
			_, err = io.Copy(outFile, rc)
			return err
		}()
		if err != nil {
			return fmt.Errorf("解压文件失败: %v", err)
		}
	}

	return nil
}

func writeUrlsToFile(urls []string, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, url := range urls {
		_, err := file.WriteString(url + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

type Images_description_input struct {
	ID          int    `json:"id"`
	Image_Url   string `json:"image_url"`
	Description string `json:"description"`
}

// 删除 temp 目录下的所有文件
func deleteTempFiles() error {
	// 获取 temp 目录下的所有文件
	files, err := filepath.Glob("./temp/*")
	if err != nil {
		return err
	}

	// 删除所有文件
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			return fmt.Errorf("删除文件 %s 失败: %v", file, err)
		}
	}

	return nil
}

func CreateImagesDescription(c *gin.Context) {
	var input Images_description_input
	// 获取表单数据
	description := c.DefaultPostForm("description", "")
	if description == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  400,
			"error": "描述不能为空",
		})
		return
	}
	input.Description = description

	var localFilePath string

	file, err := c.FormFile("file")
	if err == nil {
		// 获取文件名（不带路径和扩展名）
		fileName := file.Filename
		localFilePath = "./temp/" + fileName
		// 保存上传的文件到本地
		if err := c.SaveUploadedFile(file, localFilePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("保存文件失败: %v", err)})
			return
		}

		// 上传图片到 OSS
		ossPath := fmt.Sprintf("images_description/%s", fileName)
		fileUrl, err := UploadToOSS(localFilePath, ossPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":  100,
				"error": fmt.Sprintf("图片上传到OSS失败: %v", err),
			})
			return
		}
		input.Image_Url = fileUrl
	}

	if err := models.CreateImagesDescription(dao.DB, (*models.Images_description)(&input)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "新增iamges_description失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "新增iamges_description成功",
		"data":    input,
	})

	// 删除 temp 目录下的所有文件
	if err := deleteTempFiles(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": fmt.Sprintf("删除临时文件失败: %v", err),
		})
		return
	}

}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type CompletionResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func CheckImagesDescription(c *gin.Context) {
	var imagesDescriptions []models.Images_description

	// 查询所有数据
	if err := models.CheckImagesDescription(dao.DB, &imagesDescriptions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "查询images_description失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "查询images_description成功",
		"data":    imagesDescriptions,
	})
}

// PredictionCandidate 是分类模型返回的单个候选病种。
// 置信度在 Python 侧是 0~1，Go 返回给前端前会换算成百分比。
type PredictionCandidate struct {
	Prediction string  `json:"prediction"`
	Confidence float64 `json:"confidence"`
}

// BaseResponse 结构体定义，严格按照图片中的字段
type BaseResponse struct {
	Prediction   string  `json:"prediction"`    // 对应图片中的 "prediction"
	Confidence   float64 `json:"confidence"`    // 对应图片中的 "confidence"
	ResultSearch string  `json:"result_search"` // 对应图片中的 "result_search"
	DescSearch   string  `json:"desc_search"`   // 对应图片中的 "desc_search"`
	Weight       string  `json:"weight"`        // 对应图片中的 "weight"`
	// 以下字段由 Python /analyze 新增；旧版服务不返回时分别是 nil / 空切片，
	// 按“没有质量警告、没有候选”兼容处理。
	Candidates     []PredictionCandidate `json:"candidates"`
	QualityOK      *bool                 `json:"quality_ok"`
	QualityReasons []string              `json:"quality_reasons"`
	// 视觉模型提取的形态学结构；失败时为空对象，前端和提示词按“无结构信息”处理。
	Structure        map[string]interface{} `json:"structure"`
	StructureSummary string                 `json:"structure_summary"`
}

//func Ceshi(c *gin.Context) {
//	userID, err := ValidateToken(c)
//	if err != nil {
//		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
//		return
//	}
//
//	//var input Data
//	//input.User_id = userID
//
//	var localFilePath string
//	var url string
//	var baseName string
//	file, err := c.FormFile("file")
//	conversation := c.PostForm("conversation")
//	chat_id := c.PostForm("chat_id")
//	// 将 chat_id 转换为 int 类型
//	chatID, err := strconv.Atoi(chat_id)
//	if err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{
//			"code":  400,
//			"error": "chat_id 必须是有效的整数",
//		})
//		return
//	}
//	fmt.Println(conversation)
//	fmt.Println(chat_id)
//	if err == nil {
//		// 获取文件名（不带路径和扩展名）
//		fileName := file.Filename
//		// 去除文件扩展名
//		ext := filepath.Ext(fileName)
//		baseName = fileName[:len(fileName)-len(ext)]
//
//		// 保存上传的文件到本地
//		localFilePath = "./temp/" + fileName
//
//		// 保存上传的文件到本地
//		if err := c.SaveUploadedFile(file, localFilePath); err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("保存文件失败: %v", err)})
//			return
//		}
//
//		// 判断是否是一个压缩包
//		if ext == ".zip" {
//			// 解压文件
//			destPath := "./temp/unzipped"
//			err = unzipFile(localFilePath, destPath)
//			if err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{"code": 100,
//					"error": fmt.Sprintf("解压文件失败: %v", err),
//				})
//				return
//			}
//
//			ossPath := fmt.Sprintf("%s/zip/%s", strconv.Itoa(userID), fileName)
//
//			fileUrl, err := UploadToOSS(localFilePath, ossPath)
//			if err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{
//					"code":  100,
//					"error": fmt.Sprintf("文件上传到OSS失败: %v", err),
//				})
//				return
//			}
//			url = fileUrl
//		} else {
//			// 如果上传的文件是单个图片，直接上传到 OSS
//			// 上传图片到 OSS
//			ossPath := fmt.Sprintf("%s/image/%s", strconv.Itoa(userID), fileName)
//			fileUrl, err := UploadToOSS(localFilePath, ossPath)
//			if err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{
//					"code":  100,
//					"error": fmt.Sprintf("图片上传到OSS失败: %v", err),
//				})
//				return
//			}
//			url = fileUrl
//		}
//	} else {
//		// 如果未上传文件，记录空的文件路径和URL
//		localFilePath = ""
//		url = ""
//	}
//	fmt.Println(baseName)
//	fmt.Println(url)
//
//	// 调用大模型接口进行图片预测，并传递 text
//	modelResponse, modelErr := CallLargeModelAPIForImageAndZip(localFilePath, conversation)
//	if modelErr != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("调用图片大模型接口失败：%v", modelErr)})
//		return
//	}
//
//	// 解析图片预测结果
//	var response BaseResponse
//	err = json.Unmarshal([]byte(modelResponse), &response)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "解析图片预测响应失败"})
//		return
//	}
//	confidence := response.Confidence
//	confidence = confidence * 100
//
//	fmt.Println(response)
//	//置信度小于80时直接退出，告诉前端置信度过低，弹出弹窗
//	if confidence < 80 {
//		c.JSON(http.StatusOK, gin.H{
//			"code":  201,
//			"error": "Confidence 小于 80，预测结果不可靠",
//		})
//		return
//	}
//
//	//fmt.Println(response)
//
//	confidence = math.Round(confidence*100) / 100
//	confidenceString := fmt.Sprintf("%.2f%%", confidence)
//	c.JSON(http.StatusOK, gin.H{
//		"code":            200,
//		"predicted_class": response.Prediction,
//		"confidence":      confidenceString,
//	})
//
//	// 等待前端处理完初步数据后，再进行 SSE 连接
//	time.Sleep(500 * time.Millisecond) // 让前端有时间接收 JSON 响应
//
//	var user models.Disease_information
//	if err := dao.DB.Where("id = ?", userID).First(&user).Error; err != nil {
//		if errors.Is(err, gorm.ErrRecordNotFound) {
//			c.JSON(http.StatusNotFound, gin.H{"code": 100, "error": "用户信息未找到"})
//		} else {
//			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "查询用户信息失败"})
//		}
//		return
//	}
//
//	conversationKey := fmt.Sprintf("conversation:%d", userID)
//
//	// 获取整个列表
//	history, err := models.Rdb.LRange(ctx, conversationKey, -2, -1).Result()
//	if err != nil {
//		log.Println("Failed to retrieve conversation history:", err)
//	}
//	//fmt.Println(history)
//
//	var historyText string
//	if len(history) > 0 {
//		historyText = strings.Join(history, "\n") + "\n"
//	}
//	fmt.Println(historyText)
//	//结合人工智能返回的内容去整合顺序
//	//用户的皮肤病图片进过模型分类之后，得到预测结果｛prediction｝和预测置信度｛confidence｝，
//	//并且经过知识库检索后得到了预测结果的检索内容｛result_search｝和用户描述的检索内容｛desc_search｝，
//	//以及结果和描述各自检索所占权重大小｛weight｝
//	// 整合完整对话
//	fullConversation := fmt.Sprintf(
//		"\n模型检索: 用户的皮肤病图片经过模型分类后，得到预测结果【%s】和预测置信度【%.2f%%】。\n"+
//			"通过知识库检索，获取到以下信息:\n"+
//			"🔹 预测结果检索内容: %s\n"+
//			"🔹 用户描述检索内容: %s\n"+
//			"🔹 结果与描述的检索权重占比: %s\n"+
//			"结合用户描述:\n%s",
//		response.Prediction,
//		confidence,
//		response.ResultSearch, // 直接使用字符串
//		response.DescSearch,   // 直接使用字符串
//		response.Weight,       // 直接使用字符串
//		conversation,          // 用户输入的对话
//	)
//
//	//fullConversation += "\n前两次用户询问的历史记录" + historyText
//
//	responseText, err := utils.CallAIStudioAPI(c, fullConversation)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
//		return
//	}
//
//	// deepseek回复后将结果异步保存到数据库
//	saveResponse := models.Response{
//		Chat_id:    chatID,
//		User_id:    userID,
//		Ask:        conversation,
//		Materials:  url,
//		Type:       response.Prediction,
//		Message:    responseText, //deepseek回复的内容
//		Confidence: fmt.Sprintf("%.2f", response.Confidence),
//	}
//	saveImageMessageAsync(&saveResponse)
//
//	err = utils.SaveConversationToRedis(ctx, models.Rdb, conversationKey, conversation, responseText)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
//		return
//	}
//
//	if localFilePath != "" {
//		err := os.Remove(localFilePath)
//		if err != nil {
//			fmt.Printf("删除上传文件失败: %v\n", err)
//		}
//	}
//
//	return
//}

//func Ceshi(c *gin.Context) {
//	userID, err := ValidateToken(c)
//	if err != nil {
//		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
//		return
//	}
//
//	//var input Data
//	//input.User_id = userID
//
//	var localFilePath string
//	var url string
//	var baseName string
//	file, err := c.FormFile("file")
//	conversation := c.PostForm("conversation")
//	chat_id := c.PostForm("chat_id")
//	// 将 chat_id 转换为 int 类型
//	chatID, err := strconv.Atoi(chat_id)
//	if err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{
//			"code":  400,
//			"error": "chat_id 必须是有效的整数",
//		})
//		return
//	}
//	fmt.Println(conversation)
//	fmt.Println(chat_id)
//	if err == nil {
//		// 获取文件名（不带路径和扩展名）
//		fileName := file.Filename
//		// 去除文件扩展名
//		ext := filepath.Ext(fileName)
//		baseName = fileName[:len(fileName)-len(ext)]
//
//		// 保存上传的文件到本地
//		localFilePath = "./temp/" + fileName
//
//		// 保存上传的文件到本地
//		if err := c.SaveUploadedFile(file, localFilePath); err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("保存文件失败: %v", err)})
//			return
//		}
//
//		// 判断是否是一个压缩包
//		if ext == ".zip" {
//			// 解压文件
//			destPath := "./temp/unzipped"
//			err = unzipFile(localFilePath, destPath)
//			if err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{"code": 100,
//					"error": fmt.Sprintf("解压文件失败: %v", err),
//				})
//				return
//			}
//
//			ossPath := fmt.Sprintf("%s/zip/%s", strconv.Itoa(userID), fileName)
//
//			fileUrl, err := UploadToOSS(localFilePath, ossPath)
//			if err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{
//					"code":  100,
//					"error": fmt.Sprintf("文件上传到OSS失败: %v", err),
//				})
//				return
//			}
//			url = fileUrl
//		} else {
//			// 如果上传的文件是单个图片，直接上传到 OSS
//			// 上传图片到 OSS
//			ossPath := fmt.Sprintf("%s/image/%s", strconv.Itoa(userID), fileName)
//			fileUrl, err := UploadToOSS(localFilePath, ossPath)
//			if err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{
//					"code":  100,
//					"error": fmt.Sprintf("图片上传到OSS失败: %v", err),
//				})
//				return
//			}
//			url = fileUrl
//		}
//	} else {
//		// 如果未上传文件，记录空的文件路径和URL
//		localFilePath = ""
//		url = ""
//	}
//	fmt.Println(baseName)
//	fmt.Println(url)
//
//	// 调用大模型接口进行图片预测，并传递 text
//	modelResponse, modelErr := CallLargeModelAPIForImageAndZip(localFilePath, conversation)
//	if modelErr != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("调用图片大模型接口失败：%v", modelErr)})
//		return
//	}
//
//	// 解析图片预测结果
//	var response BaseResponse
//	err = json.Unmarshal([]byte(modelResponse), &response)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "解析图片预测响应失败"})
//		return
//	}
//	confidence := response.Confidence
//	confidence = confidence * 100
//
//	fmt.Println(response)
//	//置信度小于80时直接退出，告诉前端置信度过低，弹出弹窗
//	if confidence < 80 {
//		c.JSON(http.StatusOK, gin.H{
//			"code":  201,
//			"error": "Confidence 小于 80，预测结果不可靠",
//		})
//		return
//	}
//
//	//fmt.Println(response)
//
//	confidence = math.Round(confidence*100) / 100
//	confidenceString := fmt.Sprintf("%.2f%%", confidence)
//	c.JSON(http.StatusOK, gin.H{
//		"code":            200,
//		"predicted_class": response.Prediction,
//		"confidence":      confidenceString,
//	})
//
//	// 等待前端处理完初步数据后，再进行 SSE 连接
//	time.Sleep(500 * time.Millisecond) // 让前端有时间接收 JSON 响应
//
//	var user models.Disease_information
//	if err := dao.DB.Where("id = ?", userID).First(&user).Error; err != nil {
//		if errors.Is(err, gorm.ErrRecordNotFound) {
//			c.JSON(http.StatusNotFound, gin.H{"code": 100, "error": "用户信息未找到"})
//		} else {
//			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "查询用户信息失败"})
//		}
//		return
//	}
//
//	conversationKey := fmt.Sprintf("conversation:%d", userID)
//
//	// 获取整个列表
//	history, err := models.Rdb.LRange(ctx, conversationKey, -2, -1).Result()
//	if err != nil {
//		log.Println("Failed to retrieve conversation history:", err)
//	}
//	//fmt.Println(history)
//
//	var historyText string
//	if len(history) > 0 {
//		historyText = strings.Join(history, "\n") + "\n"
//	}
//	fmt.Println(historyText)
//	//结合人工智能返回的内容去整合顺序
//	//用户的皮肤病图片进过模型分类之后，得到预测结果｛prediction｝和预测置信度｛confidence｝，
//	//并且经过知识库检索后得到了预测结果的检索内容｛result_search｝和用户描述的检索内容｛desc_search｝，
//	//以及结果和描述各自检索所占权重大小｛weight｝
//	// 整合完整对话
//	fullConversation := fmt.Sprintf(
//		"\n模型检索: 用户的皮肤病图片经过模型分类后，得到预测结果【%s】和预测置信度【%.2f%%】。\n"+
//			"通过知识库检索，获取到以下信息:\n"+
//			"🔹 预测结果检索内容: %s\n"+
//			"🔹 用户描述检索内容: %s\n"+
//			"🔹 结果与描述的检索权重占比: %s\n"+
//			"结合用户描述:\n%s",
//		response.Prediction,
//		confidence,
//		response.ResultSearch, // 直接使用字符串
//		response.DescSearch,   // 直接使用字符串
//		response.Weight,       // 直接使用字符串
//		conversation,          // 用户输入的对话
//	)
//
//	//fullConversation += "\n前两次用户询问的历史记录" + historyText
//
//	responseText, err := utils.CallAIStudioAPI(c, fullConversation)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
//		return
//	}
//
//	// deepseek回复后将结果异步保存到数据库
//	saveResponse := models.Response{
//		Chat_id:    chatID,
//		User_id:    userID,
//		Ask:        conversation,
//		Materials:  url,
//		Type:       response.Prediction,
//		Message:    responseText, //deepseek回复的内容
//		Confidence: fmt.Sprintf("%.2f", response.Confidence),
//	}
//	saveImageMessageAsync(&saveResponse)
//
//	err = utils.SaveConversationToRedis(ctx, models.Rdb, conversationKey, conversation, responseText)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
//		return
//	}
//
//	if localFilePath != "" {
//		err := os.Remove(localFilePath)
//		if err != nil {
//			fmt.Printf("删除上传文件失败: %v\n", err)
//		}
//	}
//
//	return
//}

//func UploadFile(c *gin.Context) {
//	// 验证用户身份
//	userID, err := ValidateToken(c)
//	if err != nil {
//		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
//		return
//	}
//
//	// 获取 chat_id，确保是有效整数
//	chatID, err := strconv.Atoi(c.PostForm("chat_id"))
//	if err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "chat_id 必须是有效整数"})
//		return
//	}
//
//	// 处理文件上传逻辑...
//	file, err := c.FormFile("file")
//	localFilePath := ""
//	url := ""
//
//	if file != nil {
//		fileName := file.Filename
//		localFilePath = "./temp/" + fileName
//		if err := c.SaveUploadedFile(file, localFilePath); err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": "保存文件失败"})
//			return
//		}
//
//		// 上传到 OSS
//		ossPath := fmt.Sprintf("%d/image/%s", userID, fileName)
//		url, err = UploadToOSS(localFilePath, ossPath)
//		if err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": "OSS 上传失败"})
//			return
//		}
//	}
//
//	// **异步** 处理 AI 预测，并推送消息
//	//go processAI(chatID, userID, localFilePath, url, c.PostForm("conversation"))
//
//	// 立即返回响应，不阻塞
//	c.JSON(http.StatusOK, gin.H{"code": 200, "chat_id": chatID, "message": "上传成功，等待AI分析"})
//}

// 维护全局的 SSE 客户端连接
var clients = make(map[int]chan string)
var mu sync.Mutex // 方式 1：声明全局互斥锁（推荐）

// SSE 连接处理
func SSEHandler(c *gin.Context) {
	// 获取 chat_id，确保是有效整数
	chatID, err := strconv.Atoi(c.Query("chat_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 chat_id"})
		return
	}

	fmt.Println("ceshi")

	// 设置 SSE 响应头
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Flush()

	// 创建 SSE 消息通道
	mu.Lock()
	messageChan := make(chan string, 10)
	clients[chatID] = messageChan
	mu.Unlock()

	// 监听通道中的消息并发送给前端
	for message := range messageChan {
		fmt.Fprintf(c.Writer, "data: %s\n\n", message)
		fmt.Println("2222")
		fmt.Println(message)
		c.Writer.Flush()
	}
}

//func processAI(chatID int, userID int, localFilePath string, url string, conversation string) {
//	// 发送消息：开始分析
//	sendMessage(chatID, "正在分析图片，请稍等...")
//
//	// 调用 AI 预测
//	//modelResponse, err := CallLargeModelAPIForImageAndZip(localFilePath, conversation)
//	//if err != nil {
//	//	sendMessage(chatID, "AI 预测失败，请重试")
//	//	closeSSE(chatID)
//	//	return
//	//}
//	//
//	//// 解析 AI 预测结果
//	//var response BaseResponse
//	//if err := json.Unmarshal([]byte(modelResponse), &response); err != nil {
//	//	sendMessage(chatID, "解析 AI 响应失败")
//	//	closeSSE(chatID)
//	//	return
//	//}
//	//
//	//confidence := math.Round(response.Confidence * 100)
//	//if confidence < 80 {
//	//	sendMessage(chatID, "置信度过低，预测结果不可靠")
//	//	closeSSE(chatID)
//	//	return
//	//}
//	//
//	//// AI 结果推送给前端
//	//sendMessage(chatID, fmt.Sprintf("预测结果: %s", response.Prediction))
//	//sendMessage(chatID, fmt.Sprintf("置信度: %.2f%%", confidence))
//
//	// 生成 AI 详细回复
//	responseText, err := utils.CallAIStudioAPIStream(conversation,)
//	if err != nil {
//		sendMessage(chatID, "生成 AI 回复失败")
//		closeSSE(chatID)
//		return
//	}
//
//	// 发送最终 AI 结果
//	sendMessage(chatID, fmt.Sprintf("AI 回复: %s", responseText))
//	sendMessage(chatID, "分析完成 ✅")
//
//	// 关闭 SSE 连接
//	closeSSE(chatID)
//
//	// 清理临时文件
//	if localFilePath != "" {
//		_ = os.Remove(localFilePath)
//	}
//}

// 发送 SSE 消息
func sendMessage(chatID int, message string) {
	mu.Lock()
	defer mu.Unlock()

	if ch, exists := clients[chatID]; exists {
		ch <- message
	}
}

// 关闭 SSE 连接
func closeSSE(chatID int) {
	mu.Lock()
	defer mu.Unlock()

	if ch, exists := clients[chatID]; exists {
		close(ch)
		delete(clients, chatID)
	}
}

// 如果用户仅输入文字？？？
func PredictImage(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	// 获取文本参数
	conversation := c.PostForm("conversation")
	if conversation == "" {
		c.JSON(400, gin.H{"error": "Text parameter is required"})
		return
	}

	var localFilePath string
	var url string
	// urls 是本次上传的每一张图片的 OSS 地址，前端用它保存历史记录。
	urls := make([]string, 0, 3)

	var files []*multipart.FileHeader
	if c.ContentType() == "multipart/form-data" {
		form, formErr := c.MultipartForm()
		if formErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 100, "error": fmt.Sprintf("解析上传文件失败: %v", formErr)})
			return
		}
		files = form.File["file"]
	}

	// 前端已限制 3 张，这里再做一次服务端校验，防止绕过前端提交超量图片。
	if len(files) > 3 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 100, "error": "最多只能上传3张图片"})
		return
	}

	if len(files) > 0 {
		// 容器或全新环境首次运行时 ./temp 可能不存在，先确保父目录存在再建临时目录。
		_ = os.MkdirAll("./temp", 0o755)
		tempDir, dirErr := os.MkdirTemp("./temp", "predict-")
		if dirErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("创建临时目录失败: %v", dirErr)})
			return
		}
		// 无论后续成功还是被质量/置信度门槛拦下，都清理本次请求的临时文件，
		// 避免 code:201/202 提前返回时在 ./temp 里堆积图片。
		defer os.RemoveAll(tempDir)

		savedPaths := make([]string, 0, len(files))
		for index, file := range files {
			fileName := filepath.Base(file.Filename)
			localPath := filepath.Join(tempDir, fmt.Sprintf("%d_%s", index+1, fileName))
			if saveErr := c.SaveUploadedFile(file, localPath); saveErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("保存文件失败: %v", saveErr)})
				return
			}
			savedPaths = append(savedPaths, localPath)
		}

		// 兼容用户直接上传单个压缩包的情况。
		if len(files) == 1 && strings.ToLower(filepath.Ext(files[0].Filename)) == ".zip" {
			destPath := filepath.Join(tempDir, "unzipped")
			if unzipErr := unzipFile(savedPaths[0], destPath); unzipErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 100,
					"error": fmt.Sprintf("解压文件失败: %v", unzipErr),
				})
				return
			}

			ossPath := fmt.Sprintf("%s/zip/%d_%s", strconv.Itoa(userID), time.Now().UnixNano(), filepath.Base(files[0].Filename))
			fileUrl, uploadErr := UploadToOSS(savedPaths[0], ossPath)
			if uploadErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":  100,
					"error": fmt.Sprintf("文件上传到OSS失败: %v", uploadErr),
				})
				return
			}
			url = fileUrl
			localFilePath = savedPaths[0]
		} else {
			// 每张图片单独上传 OSS，返回可展示的 urls；
			// 多图时再额外打包 zip 给 Python，复用其多图聚合逻辑。
			for index, file := range files {
				ossPath := fmt.Sprintf("%s/image/%d_%d_%s", strconv.Itoa(userID), time.Now().UnixNano(), index+1, filepath.Base(file.Filename))
				fileUrl, uploadErr := UploadToOSS(savedPaths[index], ossPath)
				if uploadErr != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":  100,
						"error": fmt.Sprintf("图片上传到OSS失败: %v", uploadErr),
					})
					return
				}
				urls = append(urls, fileUrl)
			}
			url = urls[0]

			if len(files) == 1 {
				localFilePath = savedPaths[0]
			} else {
				zipPath := filepath.Join(tempDir, "images.zip")
				if zipErr := createZipFromFiles(savedPaths, zipPath); zipErr != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("打包图片失败: %v", zipErr)})
					return
				}
				localFilePath = zipPath
			}
		}
	}

	// 调用大模型接口进行图片预测
	modelResponse, modelErr := CallLargeModelAPIForImageAndZip(localFilePath, conversation)
	if modelErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("调用图片大模型接口失败：%v", modelErr)})
		return
	}
	// 解析图片预测结果
	var response BaseResponse
	err = json.Unmarshal([]byte(modelResponse), &response)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "解析图片预测响应失败"})
		return
	}
	// 图片质量不合格时直接提示重拍，不再让模型硬猜；
	// QualityOK 为 nil 说明是旧版 Python 服务，按合格处理，保持兼容。
	if response.QualityOK != nil && !*response.QualityOK {
		reason := strings.Join(response.QualityReasons, "；")
		if reason == "" {
			reason = "图片不够清晰或没有明显皮肤区域"
		}
		c.JSON(http.StatusOK, gin.H{
			"code":  202,
			"error": "图片质量不足，请重新拍摄：" + reason,
		})
		return
	}

	confidence := response.Confidence * 100
	confidence = math.Round(confidence*100) / 100 // 保留两位小数
	response.Confidence = confidence
	// 候选病种的置信度也统一换算成百分比，前端可以直接展示。
	for i := range response.Candidates {
		response.Candidates[i].Confidence = math.Round(response.Candidates[i].Confidence*10000) / 100
	}
	fmt.Println(response.Confidence)
	fmt.Println(confidence)
	fmt.Println(response)
	// 置信度低于 20% 时判定图片完全不可用；20%~80% 仍继续回答，
	// 由前端在结果旁标注“结果仅供参考”，避免正常照片全被 80% 门槛拦下。
	if confidence < 20 {
		c.JSON(http.StatusOK, gin.H{
			"code":  201,
			"error": "模型对该图片的判断把握不大，预测结果不可靠"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"data":    response,
		"user_id": userID,
		"url":     url,
		"urls":    urls,
	}) // 临时文件由函数开头的 defer 统一清理
}

// createZipFromFiles 把多张图片打包成一个 zip，供 Python 的多图分析流程使用。
func createZipFromFiles(paths []string, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	writer := zip.NewWriter(zipFile)
	for index, path := range paths {
		src, openErr := os.Open(path)
		if openErr != nil {
			writer.Close()
			return openErr
		}
		info, statErr := src.Stat()
		if statErr != nil {
			src.Close()
			writer.Close()
			return statErr
		}
		header, headerErr := zip.FileInfoHeader(info)
		if headerErr != nil {
			src.Close()
			writer.Close()
			return headerErr
		}
		header.Name = fmt.Sprintf("%02d_%s", index+1, filepath.Base(path))
		header.Method = zip.Deflate
		dst, createErr := writer.CreateHeader(header)
		if createErr != nil {
			src.Close()
			writer.Close()
			return createErr
		}
		if _, copyErr := io.Copy(dst, src); copyErr != nil {
			src.Close()
			writer.Close()
			return copyErr
		}
		src.Close()
	}
	return writer.Close()
}

// BaseResponse 结构体定义，严格按照图片中的字段
type AskData struct {
	Conversation     string                `json:"conversation"`
	Prediction       string                `json:"prediction"`        // 对应图片中的 "prediction"
	Confidence       float64               `json:"confidence"`        // 对应图片中的 "confidence"
	ResultSearch     string                `json:"result_search"`     // 对应图片中的 "result_search"
	DescSearch       string                `json:"desc_search"`       // 对应图片中的 "desc_search"`
	Weight           string                `json:"weight"`            // 对应图片中的 "weight"`
	Candidates       []PredictionCandidate `json:"candidates"`        // 图片模型的 top-k 候选，前端原样透传
	StructureSummary string                `json:"structure_summary"` // 视觉模型提取的形态学结构摘要
	Url              string                `json:"url"`               //图片地址
	Chat_id          int                   `json:"chat_id"`
	User_id          int                   `json:"user_id"`
	Model            string                `json:"model"`
}

// truncateRunes 按字符截断，避免把中文从中间截成乱码。
func truncateRunes(text string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max]) + "…"
}

// buildMemoryContext 汇总两层记忆，交给大模型参考：
//  1. 短期：同一 chat_id 最近几轮问答，直接从只读查询 image_ask_messages 得到；
//  2. 长期：mem0 保存的用户自述事实，按当前问题语义检索。
//
// 任何一步失败都只返回已有部分，绝不让记忆功能拖垮正常问诊。
func buildMemoryContext(ctx context.Context, userID, chatID int, question string, limit int) string {
	if userID <= 0 {
		return ""
	}
	parts := make([]string, 0, 2)

	// 短期记忆：回答过的问题才会出现在表里，当前这一问尚未保存，不会重复。
	if chatID > 0 {
		rows, err := models.Check_recent_chat_history(dao.DB, userID, chatID, limit)
		if err != nil {
			log.Printf("读取最近对话失败（用户 %d，对话 %d）: %v", userID, chatID, err)
		} else if len(rows) > 0 {
			var builder strings.Builder
			builder.WriteString("【最近对话】同一对话最近几轮（仅用于理解上下文，不要复述）：\n")
			turns := 0
			for _, row := range rows {
				ask := truncateRunes(strings.TrimSpace(row.Ask), 120)
				answer := truncateRunes(strings.TrimSpace(row.Message), 240)
				if ask == "" || answer == "" {
					continue
				}
				builder.WriteString("用户：" + ask + "\n助手：" + answer + "\n")
				turns++
			}
			if turns > 0 {
				parts = append(parts, builder.String())
			}
		}
	}

	// 长期记忆：检索失败时 SearchMemories 返回 nil，这里自然跳过。
	question = strings.TrimSpace(question)
	if question != "" {
		memories := utils.SearchMemories(ctx, userID, question)
		if len(memories) > 0 {
			var builder strings.Builder
			builder.WriteString("【长期记忆】用户过去主动提到的信息（仅相关时参考，不要逐条复述）：\n")
			for _, memory := range memories {
				memory = truncateRunes(strings.TrimSpace(memory), 120)
				if memory != "" {
					builder.WriteString("- " + memory + "\n")
				}
			}
			parts = append(parts, builder.String())
		}
	}

	return strings.Join(parts, "\n")
}

func PredictText(c *gin.Context) {
	base64Data := c.Query("data")
	if base64Data == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 100, "error": "missing data"})
		return
	}

	jsonData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 100, "error": "invalid base64 data"})
		return
	}

	var input AskData
	if err := json.Unmarshal(jsonData, &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 100, "error": "invalid JSON"})
		return
	}

	conversationKey := fmt.Sprintf("conversation:%d", input.User_id)
	// 图片问诊的历史不再从 Redis 取（原来读了只打印、没有用），
	// 统一改由 buildMemoryContext 读取同对话最近问答 + mem0 长期记忆。
	memoryContext := buildMemoryContext(c.Request.Context(), input.User_id, input.Chat_id, input.Conversation, 6)
	memoryBlock := ""
	if memoryContext != "" {
		memoryBlock = memoryContext + "\n"
	}
	// 把候选病名一起交给大模型，避免只盯着 top-1 一个病名。
	// 只传概率达到 10% 的病名、不传概率，既避免复述百分比，也避免把 8% 之类的弱候选当成建议。
	candidateText := "无"
	if len(input.Candidates) > 0 {
		parts := make([]string, 0, len(input.Candidates))
		for _, candidate := range input.Candidates {
			if candidate.Confidence < 10 {
				continue
			}
			parts = append(parts, candidate.Prediction)
		}
		if len(parts) > 0 {
			candidateText = strings.Join(parts, "、")
		}
	}
	// 视觉模型只提供形态描述；为空时明确写“无”，避免提示词出现空行歧义。
	structureText := strings.TrimSpace(input.StructureSummary)
	if structureText == "" {
		structureText = "无"
	}
	// 不把百分比交给大模型，只用一句“把握不大”让它保留必要的谨慎表述。
	confidenceHint := ""
	if input.Confidence < 80 {
		confidenceHint = "模型对这张照片把握不大，第一句请写成“从照片看，可能是XX，但不太确定”，并建议就医确认。\n"
	}
	fullConversation := fmt.Sprintf(
		"\n模型检索: 用户的皮肤病图片经过模型分类后，模型倾向的皮肤病是【%s】。\n"+
			"模型候选结果（按可能性从高到低）: %s\n"+
			"图像结构观察（仅形态描述，不含诊断）: %s\n"+
			"通过知识库检索，获取到以下信息:\n"+
			"🔹 预测结果检索内容: %s\n"+
			"🔹 用户描述检索内容: %s\n"+
			"🔹 结果与描述的检索权重占比: %s\n"+
			"回答要求：本次是图片问诊，第一句话必须逐字以“从照片看，可能是”开头，"+
			"不能写成“从描述看”“根据图片看”“看起来”等变体，也不要加任何开场白；"+
			"这条规则优先于“信息不足时先说无法判断”的风格要求，即使信息不足也必须先给出图片判断；"+
			"如果还有其他可能，紧接着补一句“也可能是YY”；"+
			"不要用“根据您提供的信息”“感谢您的信任”之类的开场白；"+
			"不要提及置信度、概率、百分比等模型指标。"+
			"回答风格：先给倾向性判断，再简要说明依据，然后分点给出护理建议和就医时机；"+
			"语气专业、克制，统一使用“您”；信息不足时先追问，不要把推测写成确诊；"+
			"不给处方药剂量，普通回答结尾写“以上仅供参考，不能替代面诊，具体请遵医嘱。”"+
			"下面是同一对话的最近问答和用户过去主动说过的信息，属于已知用户档案："+
			"如果与当前问题相关必须使用，必要时用“您之前提到……”自然说明；"+
			"如果档案里明确写有过敏原且当前问题涉及用药，回答必须点名该过敏原，不能只写笼统的“注意过敏史”。\n"+
			"%s"+
			"%s"+
			"结合用户描述:\n%s",
		input.Prediction,
		candidateText,
		structureText,
		input.ResultSearch,
		input.DescSearch,
		input.Weight,
		confidenceHint,
		memoryBlock,
		input.Conversation,
	)

	// 图片问诊优先让最终回答模型直接看图。
	// URL 解析失败、视觉接口不可用或未开启时，会自动回退到下面的纯文本链路。
	visionImageURLs := prepareVisionImageURLs(input.Url)

	// 图片问诊的预测信息也登记下来，刷新后气泡上方的预测结果和结构描述可以恢复。
	// 和前端保持一致：只展示“可能是某个病名”，不展示置信度百分比。
	metaPrediction := strings.TrimSpace(input.Prediction)
	if metaPrediction == "" {
		metaPrediction = "暂时无法判断"
	}
	metaText := "从照片看，可能是：" + metaPrediction
	candidateParts := make([]string, 0, 2)
	for _, candidate := range input.Candidates {
		// 只把概率达到 10% 的其他候选作为“也可能是”的补充，最多两个。
		if len(candidateParts) >= 2 {
			break
		}
		if candidate.Prediction == input.Prediction || candidate.Confidence < 10 {
			continue
		}
		candidateParts = append(candidateParts, candidate.Prediction)
	}
	if len(candidateParts) > 0 {
		metaText += "；也可能是：" + strings.Join(candidateParts, "、")
	}
	if input.Confidence < 80 {
		metaText += "；模型把握不大，仅供参考"
	}
	live := newLiveStream(
		input.Chat_id,
		input.User_id,
		input.Conversation,
		input.Url,
		input.Prediction,
		fmt.Sprintf("%.2f", input.Confidence),
	)
	live.setMeta(metaText, strings.TrimSpace(input.StructureSummary))

	// **关键点：使用流式响应**
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// 增量同时写入 live 和浏览器；刷新页面后 emit 仍会更新 live，只是不再写响应。
	emit := func(event string, payload interface{}) bool {
		live.apply(event, payload)
		if c.Request.Context().Err() != nil {
			return false
		}
		c.SSEvent(event, payload)
		c.Writer.Flush()
		return true
	}

	var fullResponseText string
	usedVision := false
	if len(visionImageURLs) > 0 && visionAnswerEnabled() {
		// 真正把图片传给最终回答模型：图片在前，问题、分类器提示和知识库在后。
		visionConversation := buildVisionConversation(input, candidateText, structureText, memoryBlock, len(visionImageURLs))
		fullResponseText, err = utils.CallVisionModelStream(visionImageURLs, visionConversation, emit)
		if err == nil {
			usedVision = true
		} else {
			log.Printf("图片视觉回答失败（图片 %d 张），回退纯文本链路: %v", len(visionImageURLs), err)
			// 已经输出过内容时不能再走一遍文字链路，否则前端会出现两段回答。
			if strings.TrimSpace(fullResponseText) != "" {
				live.finish("error", "回答生成中断，请重试", nil)
				return
			}
			fullResponseText = ""
			// 多图失败时用第一张再试一次；部分视觉模型不支持一次传多图。
			if len(visionImageURLs) > 1 {
				fullResponseText, err = utils.CallVisionModelStream(visionImageURLs[:1], visionConversation, emit)
				if err == nil {
					usedVision = true
				} else {
					log.Printf("单图视觉回答仍失败，回退纯文本链路: %v", err)
					if strings.TrimSpace(fullResponseText) != "" {
						live.finish("error", "回答生成中断，请重试", nil)
						return
					}
					fullResponseText = ""
				}
			}
		}
	} else if len(visionImageURLs) > 0 {
		log.Printf("图片问答：VISION_ANSWER_ENABLED=false，使用纯文本链路")
	} else {
		log.Printf("图片问答：没有可用的图片 URL，使用纯文本链路")
	}

	if !usedVision {
		if input.Model == "DeepSeek-R1" {
			// **调用DeepSeek-R1流式 API 并获取完整回复**
			fullResponseText, err = utils.CallDeepSeekR1AIStudioAPIStream(fullConversation, emit)
			if err != nil {
				live.finish("error", err.Error(), nil)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else {
			// **调用DeepSeek-V3流式 API 并获取完整回复**
			fullResponseText, err = utils.CallDeepSeekV3AIStudioAPIStream(fullConversation, emit)
			if err != nil {
				live.finish("error", err.Error(), nil)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}

	// **异步存储完整回复**
	saveResponse := models.Response{
		Chat_id:    input.Chat_id,
		User_id:    input.User_id,
		Ask:        input.Conversation,
		Materials:  input.Url,
		Type:       input.Prediction,
		Message:    fullResponseText, // **存储完整 AI 回复**
		Confidence: fmt.Sprintf("%.2f", input.Confidence),
	}
	saveImageMessageAsync(&saveResponse)
	// 回答已经完整生成后，再异步从用户原话里抽取长期记忆。
	// 不传 AI 回答，避免把模型判断当成用户事实；失败只记日志。
	utils.AddMemoryAsync(input.User_id, input.Chat_id, input.Conversation, "image")

	// 回答结束后生成追问，事件流结束前下发给前端渲染成可点击的问题。
	// 用后台上下文：刷新页面会导致请求上下文取消，但追问仍应生成并写进登记表。
	followupCtx, cancelFollowup := context.WithTimeout(context.Background(), 20*time.Second)
	suggestions := utils.GenerateFollowups(followupCtx, input.Conversation, fullResponseText)
	cancelFollowup()

	// 先切到 done 再写 Redis：Redis 失败不影响已经生成的回答在刷新后恢复。
	live.finish("done", "", suggestions)
	if len(suggestions) > 0 && c.Request.Context().Err() == nil {
		c.SSEvent("suggestions", gin.H{"suggestions": suggestions})
		c.Writer.Flush()
	}

	// **存入 Redis**
	err = utils.SaveConversationToRedis(ctx, models.Rdb, conversationKey, input.Conversation, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
}

// GetHotQuestions 返回首页空状态的 AI 热门皮肤科问题。
// 该路由挂在 /users 下，已由 Authenticate 中间件校验登录态。
// 生成失败时 data 为空，前端会继续使用内置的兜底问题。
func GetHotQuestions(c *gin.Context) {
	questions := utils.GenerateHotQuestions(c.Request.Context(), 4)
	// 每次刷新都可能抽取不同的问题，禁止浏览器缓存该响应。
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": questions})
}

// PredictOnlyText 处理“只输入文字、不上传图片”的问诊。
// 前端 home.vue 用 GET /predict/only/text?conversation=...&chat_id=... 调用，
// 期望返回 {code:200, data:{response:"..."}}。
func PredictOnlyText(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	conversation := strings.TrimSpace(c.Query("conversation"))
	if conversation == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 100, "error": "conversation 不能为空"})
		return
	}
	chatID, _ := strconv.Atoi(c.Query("chat_id"))

	// 把用户的病史、过敏情况一起带给大模型，回答才会因人而异。
	var user models.Disease_information
	if err := dao.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": 100, "error": "用户信息未找到"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "查询用户信息失败"})
		}
		return
	}

	fullConversation := "Q: " + conversation
	fullConversation += "\n我的患病史: " + user.Medical_history
	fullConversation += "\n过敏情况: " + user.Allergies
	// 短期记忆按 chat_id 取最近几轮，长期记忆按当前问题语义检索。
	memoryContext := buildMemoryContext(c.Request.Context(), userID, chatID, conversation, 6)

	modelResponse, err := CallLargeModelAPIForText(ConversationData{
		Conversation:  fullConversation,
		MemoryContext: memoryContext,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": fmt.Sprintf("调用文字描述大模型接口失败：%v", err),
		})
		return
	}

	var responseForText ModelResponse
	if err := json.Unmarshal([]byte(modelResponse), &responseForText); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "解析文字描述响应失败"})
		return
	}

	// 保存本次问答，历史记录页面才能看到纯文字问诊。
	saveImageMessageAsync(&models.Response{
		Chat_id:    chatID,
		User_id:    userID,
		Ask:        conversation,
		Type:       responseForText.PredictedClass,
		Message:    responseForText.GeneratedText,
		Confidence: fmt.Sprintf("%.2f", responseForText.Confidence),
	})
	// 只把用户原话交给 mem0 做事实抽取，回答内容不写进长期记忆。
	utils.AddMemoryAsync(userID, chatID, conversation, "text")

	conversationKey := fmt.Sprintf("conversation:%d", userID)
	currentMessage := fmt.Sprintf("Q: %s\nA: %s", conversation, responseForText.GeneratedText)
	if err := models.Rdb.RPush(ctx, conversationKey, currentMessage).Err(); err == nil {
		models.Rdb.LTrim(ctx, conversationKey, -2, -1)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"response": responseForText.GeneratedText,
		},
	})
}

// PredictOnlyTextStream 是 PredictOnlyText 的流式版本。
// 前端通过 fetch 带 Authorization 请求，Go 把 Python 的 SSE chunk 原样转发给浏览器。
func PredictOnlyTextStream(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	conversation := strings.TrimSpace(c.Query("conversation"))
	if conversation == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 100, "error": "conversation 不能为空"})
		return
	}
	chatID, _ := strconv.Atoi(c.Query("chat_id"))

	// 和普通文字问诊保持一致，把病史、过敏情况一起带给大模型。
	var user models.Disease_information
	if err := dao.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": 100, "error": "用户信息未找到"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "查询用户信息失败"})
		}
		return
	}

	fullConversation := "Q: " + conversation
	fullConversation += "\n我的患病史: " + user.Medical_history
	fullConversation += "\n过敏情况: " + user.Allergies
	// 历史上下文单独传给 Python，不拼进 conversation，避免污染知识库检索。
	memoryContext := buildMemoryContext(c.Request.Context(), userID, chatID, conversation, 6)

	// 开始生成前登记这条回答。浏览器刷新后前端会丢掉内存状态，
	// 但这里仍会持续接收并保存增量，刷新后的页面可通过状态接口接回来。
	live := newLiveStream(chatID, userID, conversation, "", "", "")

	// 所有校验通过后再切换成 SSE，避免错误响应变成半截事件流。
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	_, _ = c.Writer.Write([]byte(": connected\n\n"))
	c.Writer.Flush()

	// 用后台上下文而不是 c.Request.Context()：用户切换对话/关闭页面时浏览器会断开，
	// 但上游大模型请求仍要继续读完，回答才能保存进历史记录。
	streamCtx, cancelStream := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelStream()

	// 客户端断开后 emit 返回 false；CallTextStream 会忽略返回值继续读取上游，
	// 因此这里只负责“还连着就写”，不再承担“决定是否继续生成”的职责。
	// 无论浏览器是否还连着，增量都先写进 live，刷新后才能恢复进度。
	emit := func(event string, payload interface{}) bool {
		live.apply(event, payload)
		if c.Request.Context().Err() != nil {
			return false
		}
		c.SSEvent(event, payload)
		c.Writer.Flush()
		return true
	}

	fullText, streamErr := utils.CallTextStream(streamCtx, fullConversation, memoryContext, emit)
	if streamErr != nil {
		log.Printf("文字流式回答失败: %v", streamErr)
		live.finish("error", "AI 服务暂时不可用，请稍后再试", nil)
		// 浏览器已经断开时不用再推错误事件，保存逻辑也不执行（没有完整回答）。
		if c.Request.Context().Err() == nil {
			c.SSEvent("error", gin.H{"message": "AI 服务暂时不可用，请稍后再试"})
			c.Writer.Flush()
		}
		return
	}

	if strings.TrimSpace(fullText) == "" {
		live.finish("error", "AI 未返回内容，请稍后再试", nil)
		if c.Request.Context().Err() == nil {
			c.SSEvent("error", gin.H{"message": "AI 未返回内容，请稍后再试"})
			c.Writer.Flush()
		}
		return
	}

	// 流结束后再保存，历史记录里看到的是完整回答。
	// saveImageMessageAsync 内部会再次确认对话还在，避免用户已删除对话时写入孤儿数据。
	saveImageMessageAsync(&models.Response{
		Chat_id:    chatID,
		User_id:    userID,
		Ask:        conversation,
		Message:    fullText,
		Confidence: "0.00",
	})
	// 流式回答完整落库后再异步写长期记忆，不阻塞 SSE 的收尾事件。
	utils.AddMemoryAsync(userID, chatID, conversation, "text")

	conversationKey := fmt.Sprintf("conversation:%d", userID)
	currentMessage := fmt.Sprintf("Q: %s\nA: %s", conversation, fullText)
	if err := models.Rdb.RPush(ctx, conversationKey, currentMessage).Err(); err == nil {
		models.Rdb.LTrim(ctx, conversationKey, -2, -1)
	}

	// 完整回答已生成并提交保存，登记表切到 done，刷新后的页面能拿到完整正文。
	live.finish("done", "", nil)

	// 明确告诉前端流已经正常结束；浏览器断开时跳过写响应。
	if c.Request.Context().Err() == nil {
		c.SSEvent("done", gin.H{"ok": true})
		c.Writer.Flush()
	}
}

// SpeechToText 接收浏览器录音并转发给 Python 语音识别服务。
// 前端以 multipart/form-data 的 file 字段提交 webm/ogg 音频；
// Python 服务地址用 AI_ASR_API_URL 配置，默认本机 8001 端口。
func SpeechToText(c *gin.Context) {
	if _, err := ValidateToken(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	// 限制请求体大小，避免异常大文件把内存和 Python 服务拖垮。
	const maxAudioBytes = 20 << 20
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAudioBytes)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": 413, "error": "录音过长，请控制在 60 秒以内"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "未收到音频文件"})
		return
	}
	if fileHeader.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "音频为空，请重新录制"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "读取音频失败"})
		return
	}
	defer file.Close()

	// 重新组装 multipart，把浏览器上传的音频原样转给 Python 服务。
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(fileHeader.Filename))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "处理音频失败"})
		return
	}
	if _, err := io.Copy(part, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "读取音频失败"})
		return
	}
	if err := writer.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "处理音频失败"})
		return
	}

	apiURL := getEnvOrDefault("AI_ASR_API_URL", "http://127.0.0.1:8001/asr")
	req, err := http.NewRequest("POST", apiURL, &body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "创建语音识别请求失败"})
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 首次请求可能要加载/下载 Whisper 模型，超时给足 300 秒。
	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("语音识别服务请求失败: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "error": "语音识别服务不可用，请确认 Python 服务已启动"})
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "error": "读取语音识别结果失败"})
		return
	}

	var result struct {
		Text  string `json:"text"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		log.Printf("语音识别响应解析失败: %v, 内容: %s", err, string(responseBody))
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "error": "语音识别服务返回异常"})
		return
	}

	if resp.StatusCode != http.StatusOK || result.Text == "" {
		message := strings.TrimSpace(result.Error)
		if message == "" {
			message = "没有识别到语音，请靠近麦克风再说一次"
		}
		c.JSON(resp.StatusCode, gin.H{"code": resp.StatusCode, "error": message})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "text": result.Text, "message": "识别成功"})
}
