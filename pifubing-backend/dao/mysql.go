package dao

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"os"
)

var DB *gorm.DB

func InitMySQL() (err error) {

	//从环境变量获取数据库配置信息
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
		return
	}
	//检查数据库连接是否正常
	sqlDB, err := DB.DB()
	if err != nil {
		return
	}
	//测试数据库连接
	err = sqlDB.Ping()
	return
}

func Close() {
	sqlDB, err := DB.DB()
	if err != nil {
		fmt.Println("Error getting raw SQL DB:", err)
		return
	}
	err = sqlDB.Close()
	if err != nil {
		fmt.Println("Error closing DB:", err)
	}
}
