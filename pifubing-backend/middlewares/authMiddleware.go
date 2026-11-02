package middlewares

import (
	"SkinDisease/controller"
	"SkinDisease/models"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
)

// JWT 验证中间件
func Authenticate(c *gin.Context) {
	tokenString := c.GetHeader("Authorization")
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "100", "error": "token为空"})
		c.Abort()
		return
	}

	claims := &controller.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return controller.JwtKey, nil
	})

	if err != nil || !token.Valid {
		log.Println("Token parsing error:", err)
		c.JSON(http.StatusUnauthorized, gin.H{"code": "100", "error": "token格式错误、密钥错误或已过期"})
		c.Abort()
		return
	}

	// 检查 token 是否在黑名单中
	if models.IsTokenInBlacklist(tokenString) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "100", "error": "token已经被撤销"})
		c.Abort()
		return
	}

	// 继续处理请求
	c.Next()
}
