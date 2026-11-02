package models

import "gorm.io/gorm"

func (AskMessage) TableName() string {
	return "ask_messages"
}

type AskMessage struct {
	User_id int    `json:"user_id"`
	Message string `json:"message"`
}

func SaveTongYiToDataBase(db *gorm.DB, response *AskMessage) error {
	if err := db.Create(response).Error; err != nil {
		return err
	} else {
		return nil
	}
}
