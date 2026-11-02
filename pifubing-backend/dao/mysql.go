package dao

import (
	"fmt"
	"os"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DB 是全局数据库连接，controller 和 models 都通过它访问数据库。
var DB *gorm.DB

// InitMySQL 初始化数据库连接。
//
// 默认保持原来的 MySQL 行为；当环境变量 DB_DRIVER=sqlite 时，
// 改为打开本地 SQLite 文件，这样不依赖任何远程数据库。
func InitMySQL() (err error) {
	driver := strings.ToLower(strings.TrimSpace(os.Getenv("DB_DRIVER")))
	if driver == "" {
		driver = "mysql"
	}

	switch driver {
	case "sqlite":
		// SQLITE_PATH 为空时，默认在启动目录生成 skin_disease.db。
		path := strings.TrimSpace(os.Getenv("SQLITE_PATH"))
		if path == "" {
			path = "./skin_disease.db"
		}
		DB, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
		if err != nil {
			return err
		}

	case "mysql":
		// 原来的 MySQL 连接逻辑保持不变，方便以后切回云数据库。
		dbUser := os.Getenv("DB_USER")
		dbPassword := os.Getenv("DB_PASSWORD")
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbName := os.Getenv("DB_NAME")
		dbCharset := os.Getenv("DB_CHARSET")

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
			dbUser, dbPassword, dbHost, dbPort, dbName, dbCharset)

		DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("不支持的 DB_DRIVER=%q，可选值：mysql、sqlite", driver)
	}

	// 取到底层 *sql.DB 并 Ping 一次，确保连接真的可用。
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Close 关闭数据库连接。
func Close() {
	if DB == nil {
		return
	}
	sqlDB, err := DB.DB()
	if err != nil {
		fmt.Println("Error getting raw SQL DB:", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		fmt.Println("Error closing DB:", err)
	}
}
