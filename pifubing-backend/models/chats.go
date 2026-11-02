package models

import "gorm.io/gorm"

type Chat struct {
	ID      int    `json:"id"`
	User_id int    `json:"user_id"`
	Name    string `json:"name"`
}

func (Chat) TableName() string {
	return "chats"
}

func CreateChat(db *gorm.DB, chat *Chat) error {
	if err := db.Create(chat).Error; err != nil {
		return err
	}
	return nil
}

func UpdateChat(db *gorm.DB, chat *Chat) error {
	if err := db.Where("id = ?", chat.ID).Updates(chat).Error; err != nil {
		return err
	}
	return nil
}

func CheckChat(db *gorm.DB, user_id int, page int, size int) ([]Chat, int64, error) {
	var chats []Chat
	var total int64

	// 查询符合条件的总记录数
	if err := db.Model(&Chat{}).Where("user_id = ?", user_id).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * size
	if err := db.Where("user_id = ?", user_id).
		Offset(offset).
		Limit(size).
		Find(&chats).Error; err != nil {
		return nil, 0, err
	}

	return chats, total, nil
}

type Delete_Chat_input struct {
	User_id int `json:"user_id"`
	Chat_id int `json:"chat_id"`
}

func DeleteChat(db *gorm.DB, chat *Delete_Chat_input) error {
	if err := db.Table("chats").Where("id = ?", chat.Chat_id).Delete(chat).Error; err != nil {
		return err
	}
	return nil
}
