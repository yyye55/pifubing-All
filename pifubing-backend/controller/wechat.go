// // Go语言的示例代码
package controller

//
import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
)

// 获取微信的 session_key 和 openid
func getWeChatSession(code string) (string, string, error) {
	// 微信API接口URL
	apiURL := "https://api.weixin.qq.com/sns/jscode2session"
	// 你的 appid 和 secret
	appID := "REDACTED_WECHAT_APP_ID"
	secret := "REDACTED_WECHAT_APP_SECRET"

	// 构造请求URL
	reqURL := fmt.Sprintf("%s?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", apiURL, appID, secret, code)

	// 发送请求
	resp, err := http.Get(reqURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	// 读取返回数据
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	// 假设返回数据为 JSON 格式
	// {"openid":"user_openid", "session_key":"session_key", "unionid":"user_unionid"}
	// 这里需要解析 JSON 数据，获取 openid 和 session_key
	var result map[string]string
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", "", err
	}

	return result["openid"], result["session_key"], nil
}

// 定义结构体
type Code struct {
	Code string `json:"code"`
}

type Key struct {
	Openid      string `json:"openid"`
	Session_key string `json:"session_key"`
}

func Wxlogin(c *gin.Context) {
	var code Code
	if err := c.ShouldBindJSON(&code); err != nil {
		fmt.Println(err)
		c.JSON(500, gin.H{
			"code":  500,
			"error": "获取微信code失败",
		})
		return
	}

	var key Key
	openid, session_key, err := getWeChatSession(code.Code)
	if err != nil {
		fmt.Println("获取session_key失败:", err)
		return
	}
	key.Openid = openid
	key.Session_key = session_key

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "登录成功", "data": key})

}

// 定义一个结构体来存储请求的数据
type UserInfo struct {
	Openid     string `json:"openid"`      // 微信的openid
	SessionKey string `json:"session_key"` // 微信的session_key
	UserInfo   struct {
		NickName  string `json:"nickName"`  // 用户昵称
		AvatarUrl string `json:"avatarUrl"` // 用户头像
		Gender    int    `json:"gender"`    // 用户性别
	} `json:"userInfo"` // 用户的基本信息
}

// 模拟一个数据库结构（这里使用map模拟数据库）
var mockDB = make(map[string]UserInfo)

// 注册接口
func RegisterUser(c *gin.Context) {
	var userInfo UserInfo
	// 绑定请求体中的 JSON 数据到 userInfo 结构体
	if err := c.ShouldBindJSON(&userInfo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data"})
		return
	}

	// 打印请求体中的数据（调试用）
	fmt.Printf("Received user info: %+v\n", userInfo)

	// 判断用户是否已注册
	if _, exists := mockDB[userInfo.Openid]; exists {
		// 用户已注册，返回用户信息
		c.JSON(http.StatusOK, gin.H{"message": "User already registered", "data": userInfo})
		return
	}

	// 用户未注册，保存到数据库（这里用模拟的 map 存储）
	mockDB[userInfo.Openid] = userInfo

	// 返回注册成功的响应
	c.JSON(http.StatusOK, gin.H{"message": "User registered successfully", "data": userInfo})
}
