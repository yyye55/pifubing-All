# 慧语星河 · AI 皮肤病医疗顾问

面向皮肤病患者的一站式智能问诊系统：用户上传患处图片或输入文字描述，系统完成皮肤病分类、知识库检索，并调用大模型生成诊断建议；同时提供用户注册登录、问诊历史、个人健康档案与关怀模式。

> 本项目为传智杯参赛作品，代码仅供学习与竞赛评审使用，不能替代专业医生的诊断。

## 目录

- [功能概览](#功能概览)
- [技术架构](#技术架构)
- [目录结构](#目录结构)
- [快速开始](#快速开始)
- [环境变量](#环境变量)
- [接口说明](#接口说明)
- [部署上线](#部署上线)
- [常见问题](#常见问题)
- [团队信息](#团队信息)

## 功能概览

| 模块 | 功能 |
| --- | --- |
| 账号 | 邮箱验证码注册、登录、退出、忘记密码、微信登录（可选） |
| 图片问诊 | 上传患处图片/压缩包 → CNN+Transformer 分类 → 知识库检索 → 大模型生成建议 |
| 文字问诊 | 仅输入症状描述，结合个人病史与过敏史生成建议 |
| 历史记录 | 聊天框的创建、重命名、删除，历史问诊记录查询 |
| 个人中心 | 头像、昵称、年龄、性别、联系方式、病史、过敏史维护 |
| 关怀模式 | 大字号界面与内容朗读，方便老年用户使用 |

## 技术架构

```
┌────────────────────────┐
│  Vue 3 + Vite 前端      │  用户界面、路由、状态管理
│  Element Plus / Axios  │
└───────────┬────────────┘
            │ HTTP / SSE
┌───────────▼────────────┐
│  Go + Gin 后端          │  鉴权、业务逻辑、文件存储
│  GORM / JWT / Redis    │
└───────┬────────────┬───┘
        │            │
┌───────▼──────┐ ┌───▼────────────────────┐
│ MySQL/SQLite │ │ FastAPI AI 服务         │
│ 用户/问诊数据 │ │ PyTorch + FAISS + LLM  │
└──────────────┘ └────────────────────────┘
```

| 层 | 技术栈 |
| --- | --- |
| 前端 | JavaScript、Vue 3.5、Vite 5、Vue Router 4、Vuex 4、Element Plus 2.9、Axios 1.7、Marked、JSZip |
| 后端 | Go 1.21+、Gin 1.10、GORM 1.25、JWT、bcrypt、go-redis |
| 数据库 | MySQL 8（生产）/ SQLite（本地开发，纯 Go 驱动，免安装） |
| 缓存 | Redis（生产）/ miniredis 进程内缓存（本地开发，免安装） |
| AI 服务 | Python 3.10、FastAPI、PyTorch、torchvision、Transformers、FAISS、LangChain |
| 文件存储 | 本地 `uploads/` 目录 或 阿里云 OSS |

### 问诊数据流

1. 前端上传图片到 Go 后端 `POST /users/predict/image`。
2. Go 后端保存文件，调用 Python `/analyze` 做图片分类与知识库检索。
3. Python 返回 `prediction`、`confidence`、`result_search`、`desc_search`、`weight`。
4. 前端携带上述结果 `POST /predict/text`（JSON 请求体，SSE 流式返回）。
5. Go 后端调用大模型并把完整回答写入数据库，供历史记录查询。

## 目录结构

```
.
├── pifubing-frontend/          # Vue 3 前端
│   ├── src/
│   │   ├── views/              # 页面
│   │   ├── components/         # 历史记录等组件
│   │   ├── router/             # 路由（Hash 模式）
│   │   ├── apiBase.js          # 后端地址统一配置
│   │   └── main.js             # Axios baseURL 初始化
│   ├── .env.example            # 前端环境变量模板
│   └── package.json
├── pifubing-backend/           # Go 后端
│   ├── controller/             # HTTP 处理与 AI/OSS/邮件调用
│   ├── models/                 # GORM 模型与数据访问
│   ├── dao/                    # 数据库初始化（MySQL/SQLite）
│   ├── routers/                # 路由注册
│   ├── middlewares/            # JWT 鉴权、CORS
│   ├── .env.example            # 后端环境变量模板
│   └── main.go
├── pifubing-pythonCode/        # Python AI 服务
│   ├── main.py                 # FastAPI 入口
│   ├── test.py                 # 图片分类模型
│   ├── des.py / lang_faiss.py  # FAISS 知识库检索
│   ├── B3_Transformer.pth      # 图片分类权重
│   ├── requirements.txt
│   └── .env.example
└── docker-compose.yml          # 一键部署配置（可选）
```

## 快速开始

### 0. 环境要求

| 软件 | 版本 | 是否必需 |
| --- | --- | --- |
| Go | 1.21+（开发时使用 1.27） | 必需 |
| Node.js | 18+（开发时使用 24） | 必需 |
| Python | 3.10+ | 可选，仅 AI 问诊需要 |
| MySQL | 8.x | 可选，默认使用 SQLite |
| Redis | 6+ | 可选，默认使用进程内 miniredis |

### 1. 启动后端（本地默认 SQLite + miniredis，无需安装数据库）

```bash
cd pifubing-backend
cp .env.example .env        # Windows: copy .env.example .env
go mod download
go run .
```

默认监听 `http://localhost:9094`，首次启动会自动创建 `skin_disease.db` 并建表。

本地调试时建议在 `.env` 中设置：

```env
DB_DRIVER=sqlite
USE_MEMORY_REDIS=true
AUTO_MIGRATE=true
STORAGE_MODE=local
SKIP_EMAIL_SEND=true        # 不真正发邮件，验证码打印在后端控制台
```

> 验证码查看：调用 `POST /send_email` 后，在后端控制台日志中查找 `[开发模式] 邮箱 ... 的验证码：xxxxxx`。

### 2. 启动前端

```bash
cd pifubing-frontend
npm install
npm run dev
```

默认访问 `http://localhost:5173`。前端通过 `VITE_API_BASE` 指向后端，本地默认 `http://localhost:9094`。

### 3. 启动 Python AI 服务（可选）

```bash
cd pifubing-pythonCode
python -m venv .venv
# Windows: .venv\Scripts\activate
# macOS/Linux: source .venv/bin/activate
pip install -r requirements.txt
cp .env.example .env        # 按需填写 LLM_API_KEY 等
python main.py
```

默认监听 `http://localhost:8001`，健康检查：`GET /health`。

**AI 服务的数据依赖（未包含在仓库中）：**

- 图片分类权重 `B3_Transformer.pth` 已随仓库提供，开箱即可做图片分类。
- 知识库检索需要自行提供 `EMBEDDING_MODEL_PATH`（文本向量模型目录）与 `DATA_FILE_PATH`（皮肤病知识库 Excel）。未提供时检索字段为空，图片分类仍可正常使用。
- 文字生成需要有效的 `LLM_API_KEY`；未配置时 `/process_text` 返回明确错误。

## 环境变量

### 后端 `pifubing-backend/.env`

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `9094` | HTTP 监听端口 |
| `JWT_SECRET` | 无 | JWT 签名密钥，部署前务必改为随机长字符串 |
| `DB_DRIVER` | `mysql` | `mysql` 或 `sqlite` |
| `SQLITE_PATH` | `./skin_disease.db` | SQLite 文件路径 |
| `AUTO_MIGRATE` | `false` | 启动时自动建表（本地建议 `true`） |
| `DB_HOST/PORT/USER/PASSWORD/NAME/CHARSET` | 无 | MySQL 连接配置 |
| `USE_MEMORY_REDIS` | `false` | `true` 时启动进程内 miniredis |
| `REDIS_HOST/PORT/PASSWORD/DB` | 无 | Redis 连接配置 |
| `STORAGE_MODE` | `oss` | `local` 保存到 `./uploads`；`oss` 上传阿里云 |
| `PUBLIC_BASE_URL` | `http://localhost:9094` | local 模式下文件访问地址前缀 |
| `OSS_ENDPOINT/ACCESS_KEY_ID/ACCESS_KEY_SECRET/BUCKET_NAME` | 无 | 阿里云 OSS 配置 |
| `EMAIL_FROM/PASSWORD`、`SMTP_HOST/PORT/SUBJECT` | 无 | 邮箱验证码 SMTP 配置 |
| `SKIP_EMAIL_SEND` | `false` | `true` 时不发邮件，只打印验证码 |
| `AI_IMAGE_API_URL` | 远程地址 | Python 图片分析接口 |
| `AI_TEXT_API_URL` | 远程地址 | Python 文字问诊接口 |
| `API_KEY`、`DeepSeek_R1_STUDIO_API_KEY`、`DeepSeek_V3_STUDIO_API_KEY` | 无 | 大模型密钥 |
| `VISION_ANSWER_ENABLED` | `true` | 图片问诊时让最终回答模型直接看图；`false` 回到纯文本回答 |
| `VISION_ANSWER_MODEL` | `glm-4v-plus` | 多模态模型；账号不支持时改成 `glm-4v-flash` |
| `VISION_API_KEY` | 空 | 留空时依次复用 `LLM_API_KEY` / `DeepSeek_R1_STUDIO_API_KEY` |
| `VISION_BASE_URL` | 智谱地址 | 多模态接口地址 |
| `VISION_IMAGE_URL_TTL` | `600` | OSS 签名 URL 有效期（秒） |
| `VISION_MAX_IMAGES` | `3` | 单次最多传给模型几张图 |
| `VISION_IMAGE_SOURCE` | `auto` | `auto` 本机有文件优先 base64；`url` 让模型自己下载；`base64` 优先读本地文件 |
| `VISION_IMAGE_HOSTS` | 空 | 额外允许的图片域名，逗号分隔，支持 `*.example.com` |
| `SMS_*`、`WECHAT_*` | 无 | 短信、微信登录（可选） |

### 前端 `pifubing-frontend/.env.development`

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `VITE_API_BASE` | `http://localhost:9094` | 后端地址，部署时改为公网地址或以 `/api` 反向代理 |

### Python `pifubing-pythonCode/.env`

见 [pifubing-pythonCode/.env.example](pifubing-pythonCode/.env.example)。

## 接口说明

### 公开接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/send_email` | 发送邮箱验证码 |
| POST | `/users/register` | 注册 |
| POST | `/users/login` | 登录，返回 `token` 与 `user_id` |
| POST | `/users/logout` | 退出登录（token 加入黑名单） |
| POST | `/users/forget/password` | 重置密码 |
| POST | `/predict/text` | SSE 流式问答，问诊数据放 JSON 请求体（推荐） |
| GET | `/predict/text?data=<base64>` | 旧版调用方式，仅为兼容保留 |
| GET | `/predict/only/text` | 纯文字问诊（需 Bearer Token） |
| GET | `/sse` | 通用 SSE 测试接口 |

### 需要鉴权的接口（`Authorization: Bearer <token>`）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/users/check_users_information` | 查询个人信息 |
| POST | `/users/updates_users_information` | 修改个人信息 |
| POST | `/users/create_disease_information` | 保存病史与过敏史 |
| POST | `/users/updates_avatar` | 更新头像 |
| GET | `/users/check_users_history` | 查询问诊历史 |
| POST | `/users/Create_chat` | 创建聊天框 |
| GET | `/users/Check_chat?page=1&size=10` | 聊天框列表（分页） |
| POST | `/users/Update_chat` | 重命名聊天框 |
| DELETE | `/users/Delete_chat` | 删除聊天框 |
| POST | `/users/Create_image_ask_messages` | 保存问诊记录 |
| GET | `/users/Check_image_ask_messages?chat_id=1&page=1&size=10` | 聊天框内容（分页） |
| POST | `/users/predict/image` | 上传图片问诊（multipart） |
| GET | `/users/Check/Images_Description` | 图库数据 |
| POST | `/users/Create/Images_Description` | 新增图库数据 |

## 部署上线

### 方案一：云服务器 + Docker Compose（推荐）

适用于阿里云/腾讯云等 2 核 4G 及以上 Linux 服务器。仓库根目录提供 `docker-compose.yml`：

```bash
git clone <仓库地址> && cd <仓库目录>
cp deploy.env.example .env          # 填写域名、密钥与 AI 服务地址
docker compose up -d --build
```

- 前端 Nginx 监听 `80`，`/api/` 反向代理到 Go 后端，其余路径回退到 `index.html`。
- SQLite 数据与上传文件通过 Docker volume 持久化。
- 生产环境建议将 `STORAGE_MODE` 设为 `oss`，或把 `uploads` 挂载到持久磁盘。
- 图片问诊的最终回答模型会直接读取图片：`local` 模式默认优先读本机 `./uploads` 转 base64，模型不需要访问公网；`PUBLIC_BASE_URL` 仍要设为浏览器访问站点的公网地址（域名或公网 IP），用于前端显示图片。`oss` 模式在私有 bucket 下会自动生成短时效签名 URL。腾讯云服务器访问阿里云 OSS 只需放行出站 443。
- 使用 Nginx/Caddy 配置 HTTPS 域名，并把 `PUBLIC_BASE_URL` 改为 `https://你的域名`。
- 前端构建参数 `VITE_API_BASE=/api` 已在 Compose 中设置。

### 方案二：前后端分离托管

| 部分 | 推荐平台 | 说明 |
| --- | --- | --- |
| 前端 | Vercel / Netlify / GitHub Pages | `npm run build` 后部署 `dist`，设置 `VITE_API_BASE=https://后端域名` |
| 后端 | Render / Railway / Fly.io / 云服务器 | 设置环境变量后运行 `./SkinDisease`，挂载持久磁盘保存 SQLite 与 uploads |
| 数据库 | 云 MySQL（阿里云 RDS 等） | 多实例部署时必须使用 MySQL，不建议依赖容器内 SQLite |
| Python | 云服务器 / 带 GPU 的平台 | 需要 PyTorch 与模型权重；至少 4G 内存 |

> 前端使用 Hash 路由，静态托管无需额外配置 404 回退；若改用 History 路由则需配置 rewrite。

### 部署检查清单

- [ ] `JWT_SECRET` 已改为随机长字符串
- [ ] 数据库使用持久化存储，`AUTO_MIGRATE` 已按需关闭
- [ ] `PUBLIC_BASE_URL` 与 `VITE_API_BASE` 指向公网 HTTPS 地址
- [ ] 验证码邮件使用真实可用的 SMTP 账号
- [ ] 大模型密钥有效
- [ ] `uploads` 目录可写且不会随容器重建丢失
- [ ] 不需要短信/微信登录时，相关变量留空即可

## 常见问题

**Q：本地必须安装 MySQL 和 Redis 吗？**
不需要。`DB_DRIVER=sqlite` + `USE_MEMORY_REDIS=true` 即可运行，适合开发和演示。生产环境建议 MySQL + Redis。

**Q：注册时提示“验证码不存在或已经过期”？**
确认 `USE_MEMORY_REDIS=true`，并查看后端控制台是否打印了验证码；验证码默认 5 分钟有效。若使用真实邮箱，还需确认 SMTP 授权码正确。

**Q：登录接口为什么要返回 `user_id`？**
前端后续查询个人信息、聊天记录时需要它，已与 `token` 一起返回。

**Q：图片问诊报 OSS 错误？**
本地开发在 `.env` 设置 `STORAGE_MODE=local`，文件会保存到 `pifubing-backend/uploads/` 并通过 `/uploads/...` 访问，不依赖阿里云 OSS。

**Q：只有公网 IP、没有域名，`PUBLIC_BASE_URL` 怎么填？**
填浏览器实际访问的站点根地址，例如 `http://43.142.39.16`；如果已经配置 HTTPS 域名，就填 `https://你的域名`。不要保留示例值 `https://your-domain.com`，也不要填 `http://localhost`，否则前端图片地址会指向错误位置。

**Q：腾讯云服务器可以使用阿里云 OSS 吗？**
可以。OSS 是独立的 HTTPS 存储服务，与服务器所在云无关。部署时设置 `STORAGE_MODE=oss`、`OSS_ENDPOINT`、`OSS_ACCESS_KEY_ID`、`OSS_ACCESS_KEY_SECRET`、`OSS_BUCKET_NAME`，并确保腾讯云安全组/NAT 允许出站 443。私有 bucket 下后端会为视觉模型生成短时效签名 URL；如果前端也要显示私有图片，需要额外做签名或代理。

**Q：OSS 返回 `InvalidAccessKeyId` / `AccessKey is disabled`？**
说明当前 AccessKey 已被禁用或删除。请到阿里云 RAM 控制台创建/启用一个只授权该 bucket 的 AccessKey，至少授予 `oss:GetObject`（上传还需要 `oss:PutObject`），然后把新值写入服务器环境变量，不要提交到仓库。

**Q：文字问诊返回“token 已经用完毕，请充值后使用”？**
大模型 API 余额不足，请更换有效的 `API_KEY` / `DeepSeek_*_STUDIO_API_KEY`。

**Q：Python 启动时提示检索初始化失败？**
未配置 `EMBEDDING_MODEL_PATH` 或 `DATA_FILE_PATH`。这不影响图片分类，但知识库检索字段会为空。

## 团队信息

| 项目 | 内容 |
| --- | --- |
| 团队名称 | （待填写） |
| 参赛赛道 | 传智杯（待填写具体赛道） |
| 成员 | （待填写姓名、学校、专业、分工） |
| 指导教师 | （待填写） |
| 联系邮箱 | （待填写） |

## 许可证

本项目仅用于学习交流与竞赛评审，未经授权不得用于商业用途。医学相关内容仅供参考，请及时就医。
