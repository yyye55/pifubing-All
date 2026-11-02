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
	"mime/multipart"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

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

	// 构造邮件内容
	message := []byte("From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
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
	}

	code, err := GenerateVerificationCode(6)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 100,
			"err":  "生成验证码出错",
		})
	}

	//设置验证码的过期时间为当前时间的五分钟
	expirationTime := time.Now().Add(5 * time.Minute)

	//将验证码存入redis,并设置过期时间
	err = models.Rdb.SetEX(ctx, req.To, code, 5*time.Minute).Err()

	if err != nil {
		c.JSON(500, gin.H{
			"code": 100,
			"err":  "存储验证码失败",
		})
	}

	body := "验证码：" + code + "\n" + "请在五分钟内验证！"

	err = sendEmail(req.To, body)

	c.JSON(200, gin.H{
		"code":           200,
		"message":        "邮件发送成功!",
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

	//绑定表单数据
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": err.Error(),
		})
	}

	//获取验证码
	code := input.Code
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": "验证码不为空",
		})
	}

	//验证验证码是否正确
	_, err := VerifyCode(input.Email, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": err.Error(),
		})
		return
	}

	//密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)

	user := models.User{
		Account:      input.Account,
		Password:     string(hashedPassword),
		Email:        input.Email,
		Name:         input.Name,
		Age:          input.Age,
		Gender:       input.Gender,
		Phone_number: input.Phone_number,
	}

	//创建用户
	if err := models.CreateUser(dao.DB, &user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法注册用户",
		})
	}

	// 成功响应
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "用户注册成功"})
}

type LoginInput struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

var JwtKey = []byte("secret_key")

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
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "登录成功", "token": tokenString})
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

func ValidateToken(c *gin.Context) (int, error) {
	// 获取 Authorization 头部
	tokenString := c.GetHeader("Authorization")
	//去除“Bearer”
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": "token为空"})
		c.Abort()
		return 0, nil
	}

	userID, err := ParseToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": "token为空"})
		c.Abort()
		return 0, err
	}

	return userID, nil
}

func UploadToOSS(localFilePath string, ossPath string) (string, error) {

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

	// 定义大模型 API 地址
	apiURL := "http://8.137.81.23:8001/analyze" // 替换为图片预测的实际接口地址

	// 创建 HTTP POST 请求
	req, err := http.NewRequest("POST", apiURL, &body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 发送请求
	client := &http.Client{}
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

	// 结束 multipart 数据写入
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭 multipart writer 失败: %v", err)
	}

	// 定义大模型 API 地址
	apiURL := "http://8.137.81.23:8001/process_text" // 替换为文字描述的实际接口地址

	// 创建 HTTP POST 请求
	req, err := http.NewRequest("POST", apiURL, &body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 发送请求
	client := &http.Client{}
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

// 使用异步处理,减少前端响应时间
func saveImageMessageAsync(saveResponse *models.Response) {
	go func() {
		//异步保存数据到数据库
		if dbErr := models.SaveToDataBase(dao.DB, saveResponse); dbErr != nil {
			log.Println("保存到数据库失败%v", dbErr)
		} else {
			log.Println("保存成功！")
		}
	}()
}

type ConversationData struct {
	Conversation   string  `json:"conversation"`
	PredictedClass string  `json:"predicted_class"`
	Confidence     float64 `json:"confidence"`
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
//		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
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
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
		return
	}

	//手动提取查询参数
	page, _ := strconv.Atoi(c.Query("page"))
	size, _ := strconv.Atoi(c.Query("size"))
	chat_id, _ := strconv.Atoi(c.Query("chat_id"))

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
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
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
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
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
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "修改个人聊天框名称成功",
	})

}

type Delete_Chat_input struct {
	User_id int `json:"user_id"`
	Chat_id int `json:"chat_id"`
}

func Delete_chat(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
		return
	}

	//手动提取查询参数
	chat_id, _ := strconv.Atoi(c.Query("chat_id"))

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

	if err := models.DeleteChat(dao.DB, (*models.Delete_Chat_input)(&input)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法删除个人聊天框名称",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "删除个人聊天框名称成功",
	})

}

// 阿里云短信配置
const (
	AccessKeyID     = "REDACTED_ALIYUN_ACCESS_KEY_ID"
	AccessKeySecret = "REDACTED_ALIYUN_ACCESS_KEY_SECRET"
	SignName        = "AI皮肤病顾问"        // 短信签名
	TemplateCode    = "SMS_478510736 " // 短信模板
)

// 发送短信验证码的函数
func sendSms(phoneNumber, code string) error {
	client, err := dysmsapi.NewClientWithAccessKey("cn-hangzhou", AccessKeyID, AccessKeySecret)
	if err != nil {
		return err
	}

	request := dysmsapi.CreateSendSmsRequest()
	request.Scheme = "https"
	request.PhoneNumbers = phoneNumber
	request.SignName = SignName
	request.TemplateCode = TemplateCode
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
	}

	//将验证码存入redis,并设置过期时间
	err = models.Rdb.SetEX(ctx, Number.PhoneNumber, code, 5*time.Minute).Err()

	if err != nil {
		c.JSON(500, gin.H{
			"code": 100,
			"err":  "存储验证码失败",
		})
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
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
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
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
		return
	}

	if err := models.Check_users_information(dao.DB, (*models.Users_all_information)(&input), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法查询个人信息",
		})
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
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
		return
	}

	//绑定表单数据
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": err.Error(),
		})
	}

	if err := models.Update_users_information(dao.DB, (*models.Users_information)(&input), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法更新个人信息",
		})
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
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
		return
	}

	//绑定表单数据
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  100,
			"error": err.Error(),
		})
	}

	if err := models.Update_disease_information(dao.DB, (*models.Disease_information)(&input), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  100,
			"error": "无法创建个人信息病史，过敏情况",
		})
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

// BaseResponse 结构体定义，严格按照图片中的字段
type BaseResponse struct {
	Prediction   string  `json:"prediction"`    // 对应图片中的 "prediction"
	Confidence   float64 `json:"confidence"`    // 对应图片中的 "confidence"
	ResultSearch string  `json:"result_search"` // 对应图片中的 "result_search"
	DescSearch   string  `json:"desc_search"`   // 对应图片中的 "desc_search"`
	Weight       string  `json:"weight"`        // 对应图片中的 "weight"`
}

//func Ceshi(c *gin.Context) {
//	userID, err := ValidateToken(c)
//	if err != nil {
//		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
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
//		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
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
//		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
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
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err})
		return
	}
	var localFilePath string
	var url string
	var baseName string
	file, err := c.FormFile("file")

	// 获取文本参数
	conversation := c.PostForm("conversation")
	if conversation == "" {
		c.JSON(400, gin.H{"error": "Text parameter is required"})
		return
	}
	if err == nil {
		// 获取文件名（不带路径和扩展名）
		fileName := file.Filename
		// 去除文件扩展名
		ext := filepath.Ext(fileName)
		baseName = fileName[:len(fileName)-len(ext)]

		// 保存上传的文件到本地
		localFilePath = "./temp/" + fileName

		// 保存上传的文件到本地
		if err := c.SaveUploadedFile(file, localFilePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 100, "error": fmt.Sprintf("保存文件失败: %v", err)})
			return
		}

		// 判断是否是一个压缩包
		if ext == ".zip" {
			// 解压文件
			destPath := "./temp/unzipped"
			err = unzipFile(localFilePath, destPath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 100,
					"error": fmt.Sprintf("解压文件失败: %v", err),
				})
				return
			}

			ossPath := fmt.Sprintf("%s/zip/%s", strconv.Itoa(userID), fileName)

			fileUrl, err := UploadToOSS(localFilePath, ossPath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":  100,
					"error": fmt.Sprintf("文件上传到OSS失败: %v", err),
				})
				return
			}
			url = fileUrl
		} else {
			// 如果上传的文件是单个图片，直接上传到 OSS
			// 上传图片到 OSS
			ossPath := fmt.Sprintf("%s/image/%s", strconv.Itoa(userID), fileName)
			fileUrl, err := UploadToOSS(localFilePath, ossPath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":  100,
					"error": fmt.Sprintf("图片上传到OSS失败: %v", err),
				})
				return
			}
			url = fileUrl
		}
	} else {
		// 如果未上传文件，记录空的文件路径和URL
		localFilePath = ""
		url = ""
	}
	fmt.Println(baseName)
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
	confidence := response.Confidence * 100
	confidence = math.Round(confidence*100) / 100 // 保留两位小数
	response.Confidence = confidence
	fmt.Println(response.Confidence)
	fmt.Println(confidence)
	fmt.Println(response)
	//置信度小于80时直接退出，告诉前端置信度过低，弹出弹窗
	if confidence < 80 {
		c.JSON(http.StatusOK, gin.H{
			"code":  201,
			"error": "Confidence 小于 80，预测结果不可靠"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"data":    response,
		"user_id": userID,
		"url":     url,
	})
	//删除上传的文件
	if localFilePath != "" {
		err := os.Remove(localFilePath)
		if err != nil {
			fmt.Printf("删除上传文件失败: %v\n", err)
		}
	}
}

// BaseResponse 结构体定义，严格按照图片中的字段
type AskData struct {
	Conversation string  `json:"conversation"`
	Prediction   string  `json:"prediction"`    // 对应图片中的 "prediction"
	Confidence   float64 `json:"confidence"`    // 对应图片中的 "confidence"
	ResultSearch string  `json:"result_search"` // 对应图片中的 "result_search"
	DescSearch   string  `json:"desc_search"`   // 对应图片中的 "desc_search"`
	Weight       string  `json:"weight"`        // 对应图片中的 "weight"`
	Url          string  `json:"url"`           //图片地址
	Chat_id      int     `json:"chat_id"`
	User_id      int     `json:"user_id"`
	Model        string  `json:"model"`
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
	history, _ := models.Rdb.LRange(ctx, conversationKey, -2, -1).Result()
	historyText := strings.Join(history, "\n")
	fmt.Println(historyText)
	fullConversation := fmt.Sprintf(
		"\n模型检索: 用户的皮肤病图片经过模型分类后，得到预测结果【%s】和预测置信度【%.2f%%】。\n"+
			"通过知识库检索，获取到以下信息:\n"+
			"🔹 预测结果检索内容: %s\n"+
			"🔹 用户描述检索内容: %s\n"+
			"🔹 结果与描述的检索权重占比: %s\n"+
			"结合用户描述:\n%s",
		input.Prediction,
		input.Confidence,
		input.ResultSearch,
		input.DescSearch,
		input.Weight,
		input.Conversation,
	)

	// **关键点：使用流式响应**
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	var fullResponseText string
	if input.Model == "DeepSeek-R1" {
		// **调用DeepSeek-R1流式 API 并获取完整回复**
		fullResponseText, err = utils.CallDeepSeekR1AIStudioAPIStream(fullConversation, c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		// **调用DeepSeek-V3流式 API 并获取完整回复**
		fullResponseText, err = utils.CallDeepSeekV3AIStudioAPIStream(fullConversation, c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
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

	// **存入 Redis**
	err = utils.SaveConversationToRedis(ctx, models.Rdb, conversationKey, input.Conversation, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": err.Error()})
		return
	}
}
