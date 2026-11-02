package models

import "SkinDisease/dao"

// userMigration 专门用于建表/补列，合并了 users 表的所有字段。
//
// 为什么不用 User、Disease_information、Users_information 等多个结构体
// 依次 AutoMigrate？因为它们都映射到同一张 users 表，SQLite 在逐次迁移时
// 可能按“当前结构体”重建表，导致 id、password 等不属于当前结构体的列丢失。
// 用一个合并结构体一次迁移，最稳妥。
type userMigration struct {
	ID              int `gorm:"primaryKey;autoIncrement"`
	Account         string
	Password        string
	Email           string
	Name            string
	Age             int
	Gender          string
	Phone_number    string
	Medical_history string
	Allergies       string
	Avatar_url      string
}

// TableName 让迁移结构体也映射到 users 表。
func (userMigration) TableName() string {
	return "users"
}

// AutoMigrate 创建或补齐 GORM 模型对应的数据表。
//
// 仓库里没有 .sql 建表文件，所以本地首次启动 SQLite 时靠这里建表。
// AutoMigrate 只会加列，不会删除已有列；生产环境默认关闭，避免误改线上库。
func AutoMigrate() error {
	return dao.DB.AutoMigrate(
		&userMigration{},
		&Chat{},
		&AskMessage{},
		&Response{},
		&Images_description{},
	)
}
