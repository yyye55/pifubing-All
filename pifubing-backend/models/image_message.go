package models

import "gorm.io/gorm"

type Response struct {
	Chat_id    int    `json:"chat_id"`
	User_id    int    `json:"user_id"`
	Type       string `json:"type"`
	Materials  string `json:"url"`
	Ask        string `json:"ask"`
	Message    string `json:"message"`
	Confidence string `json:"confidence"`
}

func (Response) TableName() string {
	return "image_ask_messages"
}

func SaveToDataBase(db *gorm.DB, response *Response) error {
	if err := db.Create(response).Error; err != nil {
		return err
	} else {
		return nil
	}
}

func Check_users_history(db *gorm.DB, userID int) ([]Response, error) {
	var history []Response

	if err := db.Where("user_id = ?", userID).Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// Check_recent_users_history 只取最后 limit 条问诊记录。
// image_ask_messages 表没有自增 id / 时间字段，无法在 SQL 层按“最新”排序，
// 因此按数据库默认返回顺序截取末尾记录；限制条数可以控制提示词长度和模型耗时。
func Check_recent_users_history(db *gorm.DB, userID int, limit int) ([]Response, error) {
	var history []Response

	if limit <= 0 {
		limit = 20
	}
	if err := db.Where("user_id = ?", userID).Find(&history).Error; err != nil {
		return nil, err
	}
	if len(history) > limit {
		history = history[len(history)-limit:]
	}
	return history, nil
}

type Check_image_ask_messages_input struct {
	User_id int `json:"user_id"`
	Chat_id int `json:"chat_id"`
	Page    int `json:"page"`
	Size    int `json:"size"`
}

func CheckImageAskMessages(db *gorm.DB, input *Check_image_ask_messages_input) ([]Response, int64, error) {
	var response []Response
	var total int64

	// 查询符合条件的总记录数
	if err := db.Model(&Response{}).Where("user_id = ?", input.User_id).Where("chat_id = ?", input.Chat_id).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (input.Page - 1) * input.Size
	if err := db.Where("user_id = ?", input.User_id).Where("chat_id = ?", input.Chat_id).
		Offset(offset).
		Limit(input.Size).
		Find(&response).Error; err != nil {
		return nil, 0, err
	}

	return response, total, nil
}
