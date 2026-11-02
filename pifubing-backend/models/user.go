package models

import "gorm.io/gorm"

type User struct {
	ID           int    `json:"id"`
	Account      string `json:"account"`
	Password     string `json:"password"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Age          int    `json:"age"`
	Gender       string `json:"gender"`
	Phone_number string `json:"phone_number"`
}

//type Medical struct {
//	ID             int    `json:"id"`
//	MedicalHistory string `json:"medical_history"`
//	Allergies      string `json:"allergies"`
//}

func (User) TableName() string {
	return "users"
}

func CreateUser(db *gorm.DB, user *User) error {
	if err := db.Create(user).Error; err != nil {
		return err
	}
	return nil
}

func CheckAccount(db *gorm.DB, user *User) error {
	if err := db.Where("account = ?", user.Account).First(&user).Error; err != nil {
		return err
	} else {
		return nil
	}
}

type Disease_information struct {
	Medical_history string `json:"medical_history"`
	Allergies       string `json:"allergies"`
}

func (Disease_information) TableName() string {
	return "users"
}

func Update_disease_information(db *gorm.DB, disease_information *Disease_information, id int) error {
	// 根据 id 查找用户并更新其病史和过敏信息
	if err := db.Where("id = ?", id).Updates(disease_information).Error; err != nil {
		return err
	}
	return nil
}

type Users_information struct {
	Account      string `json:"account"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Age          int    `json:"age"`
	Gender       string `json:"gender"`
	Phone_number string `json:"phone_number"`
}

func (Users_information) TableName() string {
	return "users"
}

func Update_users_information(db *gorm.DB, users_information *Users_information, id int) error {
	// 根据 id 查找用户并更新其病史和过敏信息
	if err := db.Where("id = ?", id).Updates(users_information).Error; err != nil {
		return err
	}
	return nil
}

type Users_avatar struct {
	Avatar_url string `json:"avatar_url"`
}

func (Users_avatar) TableName() string {
	return "users"
}

func Updates_users_avatar(db *gorm.DB, users_avatar *Users_avatar, id int) error {
	// 根据 id 查找用户并更新其病史和过敏信息
	if err := db.Where("id = ?", id).Updates(users_avatar).Error; err != nil {
		return err
	}
	return nil
}

type Users_all_information struct {
	Account         string `json:"account"`
	Email           string `json:"email"`
	Name            string `json:"name"`
	Age             int    `json:"age"`
	Gender          string `json:"gender"`
	Phone_number    string `json:"phone_number"`
	Medical_history string `json:"medical_history"`
	Allergies       string `json:"allergies"`
	Avatar_url      string `json:"avatar_url"`
}

func (Users_all_information) TableName() string {
	return "users"
}

func Check_users_information(db *gorm.DB, users_all_information *Users_all_information, id int) error {
	// 根据 id 查找用户信息
	if err := db.Where("id = ?", id).First(users_all_information).Error; err != nil {
		return err
	}
	return nil
}
