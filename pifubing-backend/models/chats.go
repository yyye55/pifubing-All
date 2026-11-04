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

	// 分页查询：按 id 倒序，最新对话稳定排在最前面。
	offset := (page - 1) * size
	if err := db.Where("user_id = ?", user_id).
		Order("id DESC").
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

// DeleteChat 删除指定用户的对话，并在同一事务里删除该对话下的聊天消息。
// 返回值 deleted 是实际删除的对话条数：重复删除同一对话时为 0，接口仍按成功处理。
func DeleteChat(db *gorm.DB, chat *Delete_Chat_input) (int64, error) {
	var deleted int64
	err := db.Transaction(func(tx *gorm.DB) error {
		// 必须同时校验 user_id，避免通过猜测 chat_id 删除其他用户的对话。
		result := tx.Where("id = ? AND user_id = ?", chat.Chat_id, chat.User_id).Delete(&Chat{})
		if result.Error != nil {
			return result.Error
		}
		deleted = result.RowsAffected

		// 对话主体删掉后，消息表里的记录也要清掉，否则会留下孤儿数据。
		return tx.Where("chat_id = ? AND user_id = ?", chat.Chat_id, chat.User_id).
			Delete(&Response{}).Error
	})
	return deleted, err
}

// DeleteEmptyChats 清理当前用户名下没有任何消息的空对话。
// 只处理 image_ask_messages 中查不到记录的 chats，有内容的对话不会受影响。
// 返回实际删除的 chat id 列表，前端据此同步选中状态。
func DeleteEmptyChats(db *gorm.DB, userID int) ([]int, error) {
	var ids []int
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Chat{}).
			Where("user_id = ?", userID).
			Where("NOT EXISTS (SELECT 1 FROM image_ask_messages m WHERE m.chat_id = chats.id)").
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		return tx.Where("user_id = ? AND id IN ?", userID, ids).Delete(&Chat{}).Error
	})
	return ids, err
}
