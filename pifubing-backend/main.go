package main

import (
	"SkinDisease/dao"
	"SkinDisease/models"
	"SkinDisease/routers"
	"github.com/joho/godotenv"
	"log"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("加载.env文件失败:%v", err)
	}
	models.InitRedis()
	err = dao.InitMySQL()
	if err != nil {
		panic(err)
	}
	defer dao.Close()
	r := routers.SetupRouter()

	r.Run(":9094")
}
