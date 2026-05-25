# bridage

**bridage** 是一个兼容 OpenAI 接口的 LLM API 网关代理，支持统一管理多个 AI 提供商的 API Key，并向下游用户分发 `brg_` 前缀的访问密钥。

**核心功能：**
- 上游提供商凭证 AES-256-GCM 加密存储
- 向客户端分发 `brg_` 前缀的下游 API Key
- 支持按 Key 设置模型白名单/黑名单、速率限制、Token/请求配额、有效期
- 完整的用量日志与 Token 统计
- 内置 14 个主流提供商预设，支持任意 OpenAI 兼容接口

---

## 支持的提供商

| 预设 Slug | 提供商 |
|---|---|
| `openai` | OpenAI |
| `anthropic` | Anthropic / Claude |
| `gemini` | Google Gemini |
| `doubao` | 火山引擎 / 豆包 |
| `deepseek` | DeepSeek |
| `qwen` | 阿里云百炼 / 通义千问 |
| `kimi` | Moonshot / Kimi |
| `zhipu` | 智谱 AI (GLM) |
| `baidu` | 百度千帆 / ERNIE |
| `hunyuan` | 腾讯混元 |
| `minimax` | MiniMax |
| `baichuan` | 百川 AI |
| `yi` | 零一万物 / Yi |
| `siliconflow` | SiliconFlow |

---

## 快速开始

### 1. 配置环境变量

```bash
cp .env.example .env
```

编辑 `.env`，至少设置以下三项：

```env
DATABASE_URL=postgres://bridage:bridage@localhost:5432/bridage?sslmode=disable
MASTER_KEY=至少32位的随机字符串用于加密API-Key
JWT_SECRET=JWT签名密钥
```

### 2. 启动数据库（Docker）

```bash
docker compose up -d postgres
```

### 3. 启动服务器

**Windows（PowerShell）：**
```powershell
$env:DATABASE_URL="postgres://bridage:bridage@localhost:5432/bridage?sslmode=disable"
$env:MASTER_KEY="your-master-key-at-least-32-chars"
$env:JWT_SECRET="your-jwt-secret"
go run ./cmd/bridage-server
```

**Linux / macOS：**
```bash
go run ./cmd/bridage-server
```

服务器默认监听 `http://localhost:8080`。

### 4. 创建管理员账号（仅首次）

```bash
curl -X POST http://localhost:8080/admin/bootstrap \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"Admin@123456"}'
```

> ⚠️ 此接口只能在没有任何管理员时调用一次。

### 5. 登录获取 JWT Token

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/admin/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"Admin@123456"}' | jq -r .token)
```

**Windows PowerShell：**
```powershell
$TOKEN = (Invoke-RestMethod -Uri "http://localhost:8080/admin/login" `
  -Method Post -ContentType "application/json" `
  -Body '{"username":"admin","password":"Admin@123456"}').token
```

### 6. 添加 AI 提供商（以 DeepSeek 为例）

```bash
curl -X POST http://localhost:8080/admin/presets/deepseek/bootstrap \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"api_key":"sk-你的DeepSeek密钥", "enabled": true}'
```

会自动创建提供商及其默认模型（`deepseek-chat`、`deepseek-reasoner`）。

### 7. 创建下游 API Key

```bash
curl -X POST http://localhost:8080/admin/keys \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"my-app", "rate_limit": 60}'
```

响应中包含明文 `brg_...` Key —— **请立即保存，此后不再显示**。

### 8. 使用 API Key 调用模型

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer brg_你的密钥" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "你好！"}]
  }'
```

完全兼容 OpenAI SDK，只需替换 `base_url` 和 `api_key`：

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="brg_你的密钥",
)

resp = client.chat.completions.create(
    model="deepseek-chat",
    messages=[{"role": "user", "content": "你好！"}],
)
print(resp.choices[0].message.content)
```

---

## Web 管理界面

访问 **http://localhost:8080/web/admin**，使用管理员账号登录，可在浏览器中管理提供商、模型和 API Key。

---

## 管理 API 参考

所有 `/admin/*` 接口需要 `Authorization: Bearer <JWT Token>`。

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/admin/bootstrap` | 创建首个管理员（仅一次） |
| `POST` | `/admin/login` | 登录获取 JWT Token |
| `GET` | `/admin/presets` | 查看内置提供商预设列表 |
| `POST` | `/admin/presets/:slug/bootstrap` | 从预设创建提供商及默认模型 |
| `GET` | `/admin/providers` | 查看已配置的提供商 |
| `POST` | `/admin/providers` | 手动创建自定义提供商 |
| `GET/PUT/DELETE` | `/admin/providers/:id` | 查看 / 更新 / 删除提供商 |
| `GET` | `/admin/models` | 查看所有可用模型 |
| `POST` | `/admin/models` | 手动添加模型 |
| `GET/PUT/DELETE` | `/admin/models/:id` | 查看 / 更新 / 删除模型 |
| `GET` | `/admin/keys` | 查看下游 API Key 列表 |
| `POST` | `/admin/keys` | 创建下游 API Key |
| `GET/PUT/DELETE` | `/admin/keys/:id` | 查看 / 更新 / 删除 Key |
| `GET` | `/admin/keys/:id/usage` | 查看 Key 用量日志 |

---

## 公开 API 参考（客户端使用）

所有 `/v1/*` 接口需要 `Authorization: Bearer brg_...`。

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/v1/models` | 查看可用模型列表 |
| `POST` | `/v1/chat/completions` | 聊天补全（支持流式） |
| `POST` | `/v1/responses` | Responses API |
| `POST` | `/v1/embeddings` | 文本向量嵌入 |
| `POST` | `/v1/images/generations` | 图像生成 |
| `GET` | `/v1/account/key` | 查看当前 Key 信息 |
| `GET` | `/v1/account/usage` | 查看当前 Key 用量 |

---

## 创建下游 Key 参数说明

```json
{
  "name": "my-app",
  "allowed_models": ["deepseek-chat"],
  "rate_limit": 60,
  "max_tokens": 1000000,
  "max_requests": 10000,
  "expires_at": "2026-12-31T23:59:59Z"
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `name` | string | Key 备注名称（必填） |
| `allowed_models` | []string | 允许使用的模型列表，为空则不限制 |
| `rate_limit` | int | 每分钟最大请求数，0 表示不限制 |
| `max_tokens` | int | 累计最大 Token 数，0 表示不限制 |
| `max_requests` | int | 累计最大请求数，0 表示不限制 |
| `expires_at` | datetime | Key 过期时间，不填则永不过期 |

---

## 环境变量

| 变量 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| `DATABASE_URL` | ✅ | — | PostgreSQL 连接字符串 |
| `MASTER_KEY` | ✅ | — | API Key 加密主密钥（≥32 字符） |
| `JWT_SECRET` | ✅ | — | JWT 签名密钥 |
| `LISTEN_ADDR` | — | `0.0.0.0:8080` | 监听地址 |
| `LOG_LEVEL` | — | `info` | 日志级别：debug/info/warn/error |
| `CORS_ORIGINS` | — | `*` | 允许的 CORS 来源 |
| `JWT_EXPIRY` | — | `24h` | JWT 有效期 |
| `PROVIDER_TIMEOUT` | — | `120s` | 上游请求超时时间 |
| `PROVIDER_RETRIES` | — | `2` | 上游请求失败重试次数 |

---

## 开发

```bash
# 直接运行
go run ./cmd/bridage-server

# 编译
go build -o bridage-server ./cmd/bridage-server

# 运行测试
go test ./...
```
