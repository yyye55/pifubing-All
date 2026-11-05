package routers

import (
	"SkinDisease/controller"
	"SkinDisease/middlewares"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()
	r.Use(middlewares.CORSMiddleware())

	// STORAGE_MODE=local 时，上传的图片保存在 ./uploads，通过该静态路由访问。
	r.Static("/uploads", "./uploads")
	//r.POST("/oss", controller.Oss)

	r.POST("/sms_verification", controller.SMS_verification)
	//wx登录
	r.POST("/wxusers/login", controller.Wxlogin)
	// 注册 POST 接口来处理用户登录/注册请求
	r.POST("/userInfo", controller.RegisterUser)

	//测试后端将返回内容传给前端，实现点击内容朗读功能
	//r.GET("/ceshi", controller.Ceshi)
	//用户注册
	r.POST("/users/register", controller.Register)
	//发送验证码
	r.POST("send_email", controller.SendEmailHandler)
	//用户登录
	r.POST("/users/login", controller.Login)
	//用户登出
	r.POST("/users/logout", controller.Logout)
	//忘记密码
	r.POST("/users/forget/password", controller.Forget_password)
	r.GET("/sse", controller.SSEHandler)

	//调用deepseek形成诊疗方案
	r.GET("/predict/text", controller.PredictText)
	//仅文字问诊（不上传图片）
	r.GET("/predict/only/text", controller.PredictOnlyText)
	//仅文字问诊的流式版本，响应为 text/event-stream
	r.GET("/predict/only/text/stream", controller.PredictOnlyTextStream)
	// 刷新页面后恢复正在生成/刚生成完的 AI 回答进度（进程内登记表）
	r.GET("/predict/stream/status", controller.GetLiveStreamStatus)
	// 创建学生相关的路由组
	studentsGroup := r.Group("/users")
	studentsGroup.Use(middlewares.Authenticate) // 为学生相关路由组添加公共的中间件

	studentsGroup.POST("/Create/Images_Description", controller.CreateImagesDescription)
	studentsGroup.GET("/Check/Images_Description", controller.CheckImagesDescription)

	//用户上传头像
	studentsGroup.POST("/updates_avatar", controller.Updates_avatar)

	//用户查询所有个人相关信息
	studentsGroup.GET("/check_users_information", controller.Check_users_information)

	//用户修改个人信息
	studentsGroup.POST("/updates_users_information", controller.Updates_users_information)

	//用户添加/修改病史，过敏情况等个人信息
	studentsGroup.POST("/create_disease_information", controller.Create_disease_information)

	//用户查询所有个人历史记录
	studentsGroup.GET("/check_users_history", controller.Check_users_history)

	//根据个人皮肤病情况和问诊记录生成 AI 健康档案
	studentsGroup.GET("/health_archive", controller.GetHealthArchive)

	// 通过图片预测接口，进入后端
	//studentsGroup.POST("/predict/image", controller.PredictImage)

	// 新增聊天框接口
	studentsGroup.POST("/Create_chat", controller.Create_chat)

	// 查看所有聊天框接口
	studentsGroup.GET("/Check_chat", controller.Check_chat)

	// 修改聊天框名称接口
	studentsGroup.POST("/Update_chat", controller.Update_chat)

	// 首条提问后自动生成对话标题（仅标题仍为“新对话”时生效）
	studentsGroup.POST("/Auto_title_chat", controller.Auto_title_chat)

	// 删除聊天框接口
	studentsGroup.DELETE("/Delete_chat", controller.Delete_chat)

	// 清理当前用户的空对话（没有任何消息的聊天框）
	studentsGroup.DELETE("/Delete_empty_chats", controller.Delete_empty_chats)

	// 文字描述预测接口/聊天框中历史记录
	studentsGroup.POST("/Create_image_ask_messages", controller.Create_image_ask_messages)

	// 查询聊天框中的历史记录聊天框中历史记录
	studentsGroup.GET("/Check_image_ask_messages", controller.Check_image_ask_messages)

	//上传图片检索
	studentsGroup.POST("/predict/image", controller.PredictImage)

	//首页空状态的 AI 热门皮肤科问题
	studentsGroup.GET("/hot_questions", controller.GetHotQuestions)

	return r
}
