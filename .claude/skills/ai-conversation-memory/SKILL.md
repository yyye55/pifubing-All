---
name: ai-conversation-memory
description: 维护、调试和测试本项目基于 mem0 的 AI 问诊对话记忆（同一对话最近问答 + 跨对话用户医学事实）。当需要修改记忆逻辑、配置 qdrant/embedding、启动记忆服务、排查“AI 记不住上一轮”或清理用户记忆时使用。
---

# AI 问诊对话记忆

本项目有两条问诊链路（Go 图片问诊、Go 文字问诊），记忆统一由 Go 汇总后注入：

1. **短期记忆**：按 `chat_id` 从 `image_ask_messages` 只读查询最近 3 轮问答，解决“这个药”“那个部位”指代不清。
2. **长期记忆**：mem0 按 `user_id` 保存用户主动说过的医学事实，跨对话生效，例如“对青霉素过敏”“湿疹反复三年”。

## 关键文件

- `pifubing-pythonCode/memory_service.py`：mem0 懒加载封装，本地 qdrant + SQLite。
- `pifubing-pythonCode/main.py`：
  - `/memory/search`、`/memory/add`、`/memory/list`、`DELETE /memory/user/{user_id}`
  - `/process_text`、`/process_text_stream` 的 `memory_context` 表单字段
  - `build_llm_messages` 把历史放进 system prompt，**不参与 FAISS 检索**
- `pifubing-backend/utils/memory.go`：Go 调用 Python 记忆接口，检索 3 秒超时，写入异步。
- `pifubing-backend/controller/WdwController.go`：`buildMemoryContext` 汇总两层记忆，接入三个入口。
- `pifubing-backend/models/image_message.go`：`Check_recent_chat_history` 只读查询。

## 启动顺序

```bash
# 1. Python（8001）
cd pifubing-pythonCode
python main.py

# 2. Go（9094，本地 SQLite）
cd pifubing-backend
go run main.go
```

本地最简配置：

```env
# Python
MEMORY_ENABLED=true
MEMORY_QDRANT_PATH=./memory_data/qdrant
MEMORY_HISTORY_DB=./memory_data/history.db
MEMORY_EMBEDDING_MODEL=embedding-3
MEMORY_EMBEDDING_DIMS=2048
MEMORY_API_TOKEN=local-dev-token

# Go
MEMORY_API_URL=http://127.0.0.1:8001/memory
MEMORY_API_TOKEN=local-dev-token
MEMORY_SEARCH_LIMIT=5
```

## 接口契约

```http
POST /memory/search
{"user_id": 5, "query": "我能不能用青霉素", "limit": 5}
→ {"results": [{"memory": "用户对青霉素过敏", "score": 0.48}]}

POST /memory/add
{"user_id": 5, "question": "我对青霉素过敏", "chat_id": 166, "source": "text"}
→ {"results": [{"event": "ADD", "memory": "用户对青霉素过敏"}]}

GET /memory/list?user_id=5&limit=50
DELETE /memory/user/5
```

配置了 `MEMORY_API_TOKEN` 时，所有请求必须带 `X-Memory-Token`。Go 端会自动带上。

## 设计约束（改动时不要破坏）

1. **只把用户原话交给 mem0**，不要把 AI 回答传进去；否则模型会把“可能是湿疹”存成事实。
2. **历史上下文必须走 `memory_context` 字段**，不要拼进 `conversation`；Python 用 `conversation` 做知识库检索。
3. **写入异步、检索可失败**：记忆是增强项，任何 mem0/网络故障都不能让问诊失败。
4. **不改数据库表结构**：短期记忆只读 `image_ask_messages`；长期记忆只写本地 `memory_data/`。
5. **qdrant 本地模式只能一个 Python 进程**；不要给 uvicorn 开多 worker。
6. 用户记忆含病史，`memory_data/` 已 gitignore，不能通过静态路由暴露，也不要提交。
7. **记忆块用“已知用户档案、相关时必须使用”的强措辞**。GLM-4-Flash 对“仅供参考”这类弱指令经常直接忽略；改成档案措辞后才会稳定引用。

## 常见排查

| 现象 | 检查 |
|---|---|
| `/health` 里 `memory_enabled=false` | `MEMORY_ENABLED`、`LLM_API_KEY` 是否为空 |
| 搜不到记忆 | 是否同一 `user_id`；`MEMORY_THRESHOLD` 是否过高；qdrant 目录是否被换掉 |
| 记忆是英文 | `CUSTOM_INSTRUCTIONS` 是否被改动；必须保留“用简体中文” |
| 写入很慢/重复 | 正常 `add` 约 3~7 秒；相同事实返回空结果，但换一种说法仍可能新增近似记忆，可用 `DELETE /memory/user/{id}` 清理 |
| 模型回答没提记忆 | 先确认 `/memory/search` 能搜到、`memory_context` 非空；再检查是否被“已知用户档案”强指令覆盖。GLM-4-Flash 仍可能偶发忽略或误触发无关问题固定话术 |
| 启动报 protobuf 冲突 | mem0 2.2.1 要求 protobuf<7，需要降到 6.x |
| 想彻底关闭 | Python 设 `MEMORY_ENABLED=false`，Go 端无需改动 |

## 验证用例

1. 用户 1 说“我对青霉素过敏，湿疹三年”，新开对话问“我能不能用青霉素”，回答应提到过敏。
2. 同一对话追问“那平时注意什么”，应能接上湿疹。
3. 用户 2 搜不到用户 1 的过敏史。
4. 重启 Python 后记忆仍可检索。
5. 把 `LLM_API_KEY` 改错，问诊仍能回答，只是没有记忆。
6. `/memory/list` 只看到用户自述事实，没有 AI 诊断。
