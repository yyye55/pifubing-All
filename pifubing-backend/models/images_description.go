package models

import "gorm.io/gorm"

type Images_description struct {
	ID          int    `json:"id"`
	Image_Url   string `json:"image_url"`
	Description string `json:"description"`
}

func (Images_description) TableName() string {
	return "images_description"
}

func CreateImagesDescription(db *gorm.DB, description *Images_description) error {
	if err := db.Create(description).Error; err != nil {
		return err
	}
	return nil
}

func CheckImagesDescription(db *gorm.DB, imagesDescriptions *[]Images_description) error {
	// 查询所有 images_description 数据
	if err := db.Find(imagesDescriptions).Error; err != nil {
		return err
	}
	return nil
}
