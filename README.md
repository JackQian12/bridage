# bridage

**bridage** is an OpenAI-compatible LLM API gateway proxy. It centralizes management of upstream provider API keys and distributes `brg_`-prefixed access keys to downstream users.

**Key features:**
- AES-256-GCM encrypted storage of upstream provider credentials
- Issues `brg_`-prefixed downstream API keys to clients
- Per-key model allowlists/blocklists, rate limits, token/request quotas, and expiry
- Full usage logging and token statistics
- 14 built-in provider presets; supports any OpenAI-compatible endpoint

---

## Supported Providers

| Preset Slug | Provider |
|---|---|
| `openai` | OpenAI |
| `anthropic` | Anthropic / Claude |
| `gemini` | Google Gemini |
| `doubao` | Volcengine / Doubao |
| `deepseek` | DeepSeek |
| `qwen` | Alibaba Cloud / Qwen |
| `kimi` | Moonshot / Kimi |
| `zhipu` | Zhipu AI (GLM) |
| `baidu` | Baidu Qianfan / ERNIE |
| `hunyuan` | Tencent Hunyuan |
| `minimax` | MiniMax |
| `baichuan` | Baichuan AI |
| `yi` | 01.AI / Yi |
| `siliconflow` | SiliconFlow |

---

## Quick Start

### 1. Configure environment variables

```bash
cp .env.example .env
```

Edit `.env` and set at minimum:

```env
DATABASE_URL=postgres://bridage:bridage@localhost:5432/bridage?sslmode=disable
MASTER_KEY=a-random-string-of-at-least-32-characters
JWT_SECRET=your-jwt-signing-secret
```

### 2. Start the database (Docker)

```bash
docker compose up -d postgres
```

### 3. Start the server

**Windows (PowerShell):**
```powershell
$env:DATABASE_URL="postgres://bridage:bridage@localhost:5432/bridage?sslmode=disable"
$env:MASTER_KEY="your-master-key-at-least-32-chars"
$env:JWT_SECRET="your-jwt-secret"
go run ./cmd/bridage-server
```

**Linux / macOS:**
```bash
go run ./cmd/bridage-server
```

The server listens on `http://localhost:8080` by default.

### 4. Create the first admin account (one-time)

```bash
curl -X POST http://localhost:8080/admin/bootstrap \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"Admin@123456"}'
```

> ⚠️ This endpoint can only be called once, when no admin accounts exist yet.

### 5. Log in and obtain a JWT token

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/admin/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"Admin@123456"}' | jq -r .token)
```

**Windows PowerShell:**
```powershell
$TOKEN = (Invoke-RestMethod -Uri "http://localhost:8080/admin/login" `
  -Method Post -ContentType "application/json" `
  -Body '{"username":"admin","password":"Admin@123456"}').token
```

### 6. Add an AI provider (DeepSeek as an example)

```bash
curl -X POST http://localhost:8080/admin/presets/deepseek/bootstrap \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"api_key":"sk-your-deepseek-key", "enabled": true}'
```

This automatically creates the provider and its default models (`deepseek-chat`, `deepseek-reasoner`).

### 7. Create a downstream API key

```bash
curl -X POST http://localhost:8080/admin/keys \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"my-app", "rate_limit": 60}'
```

The response contains the plaintext `brg_...` key — **save it immediately, it will not be shown again**.

### 8. Call a model with the API key

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer brg_your-key" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

Fully compatible with the OpenAI SDK — just replace `base_url` and `api_key`:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="brg_your-key",
)

resp = client.chat.completions.create(
    model="deepseek-chat",
    messages=[{"role": "user", "content": "Hello!"}],
)
print(resp.choices[0].message.content)
```

---

## Web Admin UI

Visit **http://localhost:8080/web/admin** and log in with your admin credentials to manage providers, models, and API keys from a browser.

---

## Admin API Reference

All `/admin/*` endpoints require `Authorization: Bearer <JWT Token>`.

| Method | Path | Description |
|---|---|---|
| `POST` | `/admin/bootstrap` | Create the first admin account (one-time) |
| `POST` | `/admin/login` | Log in and get a JWT token |
| `GET` | `/admin/presets` | List built-in provider presets |
| `POST` | `/admin/presets/:slug/bootstrap` | Create a provider and its default models from a preset |
| `GET` | `/admin/providers` | List configured providers |
| `POST` | `/admin/providers` | Manually create a custom provider |
| `GET/PUT/DELETE` | `/admin/providers/:id` | Get / update / delete a provider |
| `GET` | `/admin/models` | List all available models |
| `POST` | `/admin/models` | Manually add a model |
| `GET/PUT/DELETE` | `/admin/models/:id` | Get / update / delete a model |
| `GET` | `/admin/keys` | List downstream API keys |
| `POST` | `/admin/keys` | Create a downstream API key |
| `GET/PUT/DELETE` | `/admin/keys/:id` | Get / update / delete a key |
| `GET` | `/admin/keys/:id/usage` | View usage log for a key |

---

## Public API Reference (client-facing)

All `/v1/*` endpoints require `Authorization: Bearer brg_...`.

| Method | Path | Description |
|---|---|---|
| `GET` | `/v1/models` | List available models |
| `POST` | `/v1/chat/completions` | Chat completions (streaming supported) |
| `POST` | `/v1/responses` | Responses API |
| `POST` | `/v1/embeddings` | Text embeddings |
| `POST` | `/v1/images/generations` | Image generation |
| `GET` | `/v1/account/key` | Get current key info |
| `GET` | `/v1/account/usage` | Get current key usage |

---

## Downstream Key Parameters

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

| Field | Type | Description |
|---|---|---|
| `name` | string | Key label (required) |
| `allowed_models` | []string | Models this key may access; empty means no restriction |
| `rate_limit` | int | Max requests per minute; 0 means unlimited |
| `max_tokens` | int | Cumulative token cap; 0 means unlimited |
| `max_requests` | int | Cumulative request cap; 0 means unlimited |
| `expires_at` | datetime | Key expiry time; omit for no expiry |

---

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | ✅ | — | PostgreSQL connection string |
| `MASTER_KEY` | ✅ | — | Master key for encrypting provider credentials (≥32 chars) |
| `JWT_SECRET` | ✅ | — | JWT signing secret |
| `LISTEN_ADDR` | — | `0.0.0.0:8080` | Server listen address |
| `LOG_LEVEL` | — | `info` | Log level: debug / info / warn / error |
| `CORS_ORIGINS` | — | `*` | Allowed CORS origins |
| `JWT_EXPIRY` | — | `24h` | JWT token lifetime |
| `PROVIDER_TIMEOUT` | — | `120s` | Upstream request timeout |
| `PROVIDER_RETRIES` | — | `2` | Upstream request retry attempts |

---

## Development

```bash
# Run directly
go run ./cmd/bridage-server

# Build
go build -o bridage-server ./cmd/bridage-server

# Run tests
go test ./...
```
