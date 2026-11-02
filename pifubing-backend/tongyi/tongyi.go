package tongyi

//bytes：
//作用：提供缓冲区功能，特别是 bytes.NewBuffer 用于创建一个缓冲区，以便发送 HTTP 请求的请求体。
//context：
//作用：提供上下文管理，用于控制请求的取消、超时等。
//encoding/json：
//作用：处理 JSON 编码和解码，将 Go 结构体转换为 JSON 格式，或将 JSON 数据解析为 Go 结构体。
//net/http：
//作用：提供 HTTP 客户端和服务器实现，用于发送 HTTP 请求和处理 HTTP 响应。
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type TongYiClient struct {
	apiKey string
}

type TongYiRsp struct {
	Output struct {
		Text         string `json:"text"`          //生成的文本内容。
		FinishReason string `json:"finish_reason"` //生成文本的结束原因，如完成、长度限制等。
	} `json:"output"`
	Usage struct {
		OutputTokens int `json:"output_tokens"` //生成文本所使用的令牌数。什么是令牌数
		InputTokens  int `json:"input_tokens"`  //输入提示所使用的令牌数。
	} `json:"usage"`
	RequestID string `json:"request_id"` //API 请求的唯一标识符，用于追踪和调试。
}

func NewTongYiClient(apiKey string) *TongYiClient {
	return &TongYiClient{
		apiKey: apiKey,
	}
}

// 方法
// (c *TongYiClient)：表示该方法属于 TongYiClient 结构体，可以通过 c 访问其字段和其他方法。
// ctx context.Context：上下文对象，用于控制请求的取消、超时等。
// prompt string：文本生成的输入提示。
// history ...map[string]string：可变参数，用于传递历史对话记录或上下文（可选）。
func (c *TongYiClient) GenerateText(ctx context.Context, prompt string, history ...map[string]string) (*TongYiRsp, error) {
	data := map[string]interface{}{
		"model":      "qwen-turbo",             //model：指定使用的模型名称，这里为 "qwen-turbo"。
		"parameters": map[string]interface{}{}, //parameters：用于传递其他参数，这里为空的 map。
		"input": map[string]interface{}{
			"prompt":  prompt,  //prompt：用户输入的提示，用于生成相关文本。
			"history": history, //history：传递的历史对话记录（可选），有助于上下文理解。？？？怎么实现
		},
	}

	payload, err := json.Marshal(data) //json.Marshal：将 Go 数据结构编码为 JSON 字符串。
	if err != nil {
		return nil, err
	}

	//作用：定义要发送 HTTP POST 请求的 API 端点 URL。
	//解释：该 URL 是阿里云 TongYi API 的文本生成服务的具体端点。
	url := "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"

	//作用：创建一个新的 HTTP POST 请求。
	//参数：
	//"POST"：HTTP 方法，表示这是一个 POST 请求。
	//url：API 端点的 URL。
	//bytes.NewBuffer(payload)：将 JSON 编码后的请求体作为请求的主体。
	//返回值：
	//req：指向 http.Request 结构体的指针，表示 HTTP 请求。
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	//Authorization：
	//格式：Bearer <apiKey>。
	//作用：用于 API 请求的认证，确保请求者有权限访问 API。
	//Content-Type：
	//值：application/json。
	//作用：告知服务器请求体的数据格式为 JSON。
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("Content-Type", "application/json")

	//创建 HTTP 客户端：
	//client := &http.Client{}：创建一个默认的 HTTP 客户端，用于发送请求。
	client := &http.Client{}
	//发送请求：
	//resp, err := client.Do(req)：发送请求并等待响应。???
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	//延迟关闭响应体：
	//defer resp.Body.Close()：确保在函数返回前关闭响应体，释放资源。
	defer resp.Body.Close()

	//状态码不正确时
	if resp.StatusCode != http.StatusOK {
		var errorResponse struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		}
		//?
		err = json.NewDecoder(resp.Body).Decode(&errorResponse)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("API error: %s - %s", errorResponse.Code, errorResponse.Message)
	}

	//解析成功响应
	response := &TongYiRsp{}
	//?
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return response, nil
}
