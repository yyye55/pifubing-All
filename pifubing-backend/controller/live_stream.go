package controller

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// liveStream 保存一条正在生成或刚生成完的 AI 回答。
//
// 为什么需要它：文字流和图片流都与浏览器连接解耦，用户刷新页面后后端仍会
// 继续生成并把完整回答写进数据库；但前端内存里的 backgroundStreams 会随刷新丢失，
// 刷新后的新页面看不到“正在生成”的进度。把进度暂存在后端进程内，
// 新页面通过 GetLiveStreamStatus 就能接回来。
//
// 该结构只用于单实例部署的实时进度恢复；完整回答仍以数据库为准。
type liveStream struct {
	mu         sync.Mutex
	ChatID     int
	UserID     int
	Ask        string
	Images     string
	Type       string
	Confidence string
	Meta       string
	Structure  string
	Text       strings.Builder
	Thoughts   strings.Builder
	// Suggestions 在回答结束后由 Python/Go 补发，刷新后也能一起恢复。
	Suggestions []string
	Status      string // streaming｜done｜error
	Error       string
	UpdatedAt   time.Time
}

// liveStreams 以 chat_id 为键。同一对话同一时间只允许一条正在生成的回答，
// 新流会覆盖旧流，符合前端“回答中禁止再次发送”的约束。
var liveStreams = struct {
	sync.Mutex
	m map[int]*liveStream
}{m: make(map[int]*liveStream)}

// newLiveStream 在调用大模型前登记一条流；chat_id 无效时返回 nil，调用方需判空。
func newLiveStream(chatID, userID int, ask, images, typ, confidence string) *liveStream {
	if chatID <= 0 {
		return nil
	}
	s := &liveStream{
		ChatID:     chatID,
		UserID:     userID,
		Ask:        ask,
		Images:     images,
		Type:       typ,
		Confidence: confidence,
		Status:     "streaming",
		UpdatedAt:  time.Now(),
	}
	liveStreams.Lock()
	liveStreams.m[chatID] = s
	liveStreams.Unlock()
	return s
}

func getLiveStream(chatID int) *liveStream {
	liveStreams.Lock()
	defer liveStreams.Unlock()
	return liveStreams.m[chatID]
}

// apply 把流式增量写进登记表。浏览器是否还连着不影响这里，
// 因此刷新页面后仍能拿到断开前已经生成的正文。
func (s *liveStream) apply(event string, payload interface{}) {
	if s == nil {
		return
	}
	values, ok := payload.(gin.H)
	if !ok {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpdatedAt = time.Now()

	switch event {
	case "message":
		if content, ok := values["content"].(string); ok {
			s.Text.WriteString(content)
		}
	case "thoughts":
		if thoughts, ok := values["thoughts"].(string); ok {
			s.Thoughts.WriteString(thoughts)
		}
	case "suggestions":
		if suggestions, ok := values["suggestions"].([]string); ok {
			s.Suggestions = append([]string(nil), suggestions...)
		}
	}
}

// setMeta 记录图片问诊的预测结果和图像结构，刷新后 AI 气泡上方的信息不会丢。
func (s *liveStream) setMeta(meta, structure string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.Meta = meta
	s.Structure = structure
	s.mu.Unlock()
}

// finish 结束流并保留一段时间，覆盖“刚生成完、数据库异步写入还没完成”的窗口。
func (s *liveStream) finish(status, errMessage string, suggestions []string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	// 只允许从 streaming 进入结束态，避免错误处理与 finally 重复调用时覆盖成功状态。
	if s.Status == "streaming" {
		s.Status = status
		s.Error = errMessage
	}
	if suggestions != nil {
		s.Suggestions = append([]string(nil), suggestions...)
	}
	s.UpdatedAt = time.Now()
	s.mu.Unlock()

	time.AfterFunc(5*time.Minute, func() {
		liveStreams.Lock()
		// 同一条对话可能已经开始了新的回答，只清理自己这一条。
		if liveStreams.m[s.ChatID] == s {
			delete(liveStreams.m, s.ChatID)
		}
		liveStreams.Unlock()
	})
}

// snapshot 返回给前端的状态快照。返回前拷贝切片，避免轮询期间被流式写入改动。
func (s *liveStream) snapshot() gin.H {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := s.Status
	if status == "" {
		status = "streaming"
	}
	return gin.H{
		"chat_id":     s.ChatID,
		"ask":         s.Ask,
		"images":      s.Images,
		"type":        s.Type,
		"confidence":  s.Confidence,
		"meta":        s.Meta,
		"structure":   s.Structure,
		"text":        s.Text.String(),
		"thoughts":    s.Thoughts.String(),
		"suggestions": append([]string(nil), s.Suggestions...),
		"status":      status,
		"active":      status == "streaming",
		"error":       s.Error,
	}
}

// GetLiveStreamStatus 供刷新后的前端恢复当前对话的回答进度。
// 只返回属于当前登录用户的流；没有记录时返回 idle，前端保持数据库里的历史即可。
func GetLiveStreamStatus(c *gin.Context) {
	userID, err := ValidateToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": err.Error()})
		return
	}

	chatID, _ := strconv.Atoi(c.Query("chat_id"))
	if chatID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 100, "error": "chat_id 不能为空"})
		return
	}

	live := getLiveStream(chatID)
	// 对话 ID 可能被猜到，必须校验归属，不能把其他用户的问诊内容返回出去。
	if live == nil || live.UserID != userID {
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"data": gin.H{"chat_id": chatID, "status": "idle", "active": false},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": live.snapshot()})
}
