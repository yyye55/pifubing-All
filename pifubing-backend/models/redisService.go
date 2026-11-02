package models

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

var ctx = context.Background()
var Rdb *redis.Client

func InitRedis() {
	redisHost := os.Getenv("REDIS_HOST")
	redisPORT := os.Getenv("REDIS_PORT")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := os.Getenv("REDIS_DB")

	db, err := strconv.Atoi(redisDB)
	if err != nil {
		log.Fatalf("转换Redis Db 号失败：%v", err)
	}

	//初始化Redis数据库号为整数
	Rdb = redis.NewClient(&redis.Options{
		Addr:     redisHost + ":" + redisPORT,
		Password: redisPassword,
		DB:       db,
	})

	_, err = Rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis:%v", err)
	}
}

// 将 token 添加到黑名单
func AddToBlacklist(tokenString string) error {
	// 去除 'Bearer ' 前缀
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	expiration := time.Hour * 24 // 假设 token 有效期是 24 小时
	err := Rdb.SetEX(ctx, tokenString, "blacklisted", expiration).Err()
	if err != nil {
		log.Println("Error adding token to blacklist:", err)
		return err
	}
	return nil
}

// 检查 token 是否在黑名单中
func IsTokenInBlacklist(tokenString string) bool {
	result, err := Rdb.Get(ctx, tokenString).Result()
	// 如果 Redis 中没有该 token，说明它不在黑名单
	if err == redis.Nil {
		return false
	} else if err != nil {
		// 如果遇到 Redis 错误，可以打印日志，返回 false
		log.Println("Redis error:", err)
		return false
	}

	// 如果返回值为 "blacklisted"，说明 token 被拉入黑名单
	return result == "blacklisted"
}
