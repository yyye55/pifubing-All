package main

import (
	"log"
	"os"
	"strings"

	"SkinDisease/controller"
	"SkinDisease/dao"
	"SkinDisease/models"
	"SkinDisease/routers"

	"github.com/alicebob/miniredis/v2"
	"github.com/joho/godotenv"
)

// envBool 读取布尔型环境变量，兼容 true/TRUE/1 等写法。
func envBool(name string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	return value == "true" || value == "1" || value == "yes"
}

func main() {
	// 本地开发读取 .env；部署到云平台时通常没有 .env，改用平台环境变量。
	if err := godotenv.Load(); err != nil {
		log.Printf("未找到 .env 文件，将使用系统环境变量: %v", err)
	}
	controller.InitJWTKey()

	// 本地开发使用进程内 Redis，避免依赖外部 Redis。
	// miniredis 只监听本机随机端口，进程退出后数据自动清空。
	if envBool("USE_MEMORY_REDIS") {
		mr, err := miniredis.Run()
		if err != nil {
			log.Fatalf("启动进程内 Redis 失败: %v", err)
		}
		defer mr.Close()

		// 在 models.InitRedis() 读取配置前覆盖 Redis 地址。
		os.Setenv("REDIS_HOST", mr.Host())
		os.Setenv("REDIS_PORT", mr.Port())
		os.Setenv("REDIS_PASSWORD", "")
		os.Setenv("REDIS_DB", "0")
		log.Printf("使用进程内 Redis: %s", mr.Addr())
	}

	models.InitRedis()

	if err := dao.InitMySQL(); err != nil {
		panic(err)
	}
	defer dao.Close()

	// 本地 SQLite 首次启动时自动建表。
	// 生产环境默认关闭，避免误改已有数据库结构。
	if envBool("AUTO_MIGRATE") {
		if err := models.AutoMigrate(); err != nil {
			log.Fatalf("数据库自动迁移失败: %v", err)
		}
		log.Println("数据库自动迁移完成")
	}

	// 端口可通过 PORT 覆盖，方便部署到云平台。
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "9094"
	}

	r := routers.SetupRouter()
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("启动 HTTP 服务失败: %v", err)
	}
}
