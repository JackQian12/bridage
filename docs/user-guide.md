# 用户接入指南

本文档面向使用 bridage 网关的终端用户。管理员会为你分配一个 `brg_` 开头的 API Key，按照本文档接入即可。

---

## 接入信息

| 项目 | 值 |
|---|---|
| **Base URL** | `http://<服务器地址>:8080/v1` |
| **认证方式** | `Authorization: Bearer brg_你的密钥` |
| **协议** | 与 OpenAI API 完全兼容 |

> 将 `<服务器地址>` 替换为管理员提供的实际地址。

---

## 查看可用模型

```bash
curl http://<服务器地址>:8080/v1/models \
  -H "Authorization: Bearer brg_你的密钥"
```

---

## 调用示例

### curl

```bash
curl http://<服务器地址>:8080/v1/chat/completions \
  -H "Authorization: Bearer brg_你的密钥" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat",
    "messages": [
      {"role": "user", "content": "你好！"}
    ]
  }'
```

**流式输出（stream）：**

```bash
curl http://<服务器地址>:8080/v1/chat/completions \
  -H "Authorization: Bearer brg_你的密钥" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat",
    "stream": true,
    "messages": [
      {"role": "user", "content": "写一首短诗"}
    ]
  }'
```

---

### Python（openai 库）

```bash
pip install openai
```

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://<服务器地址>:8080/v1",
    api_key="brg_你的密钥",
)

# 普通调用
response = client.chat.completions.create(
    model="deepseek-chat",
    messages=[
        {"role": "system", "content": "你是一个有帮助的助手。"},
        {"role": "user", "content": "你好！"},
    ],
)
print(response.choices[0].message.content)
```

**流式输出：**

```python
stream = client.chat.completions.create(
    model="deepseek-chat",
    messages=[{"role": "user", "content": "讲个故事"}],
    stream=True,
)
for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
```

---

### Node.js / TypeScript（openai 库）

```bash
npm install openai
```

```typescript
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://<服务器地址>:8080/v1",
  apiKey: "brg_你的密钥",
});

const response = await client.chat.completions.create({
  model: "deepseek-chat",
  messages: [{ role: "user", content: "你好！" }],
});
console.log(response.choices[0].message.content);
```

---

### LangChain（Python）

```python
from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    base_url="http://<服务器地址>:8080/v1",
    api_key="brg_你的密钥",
    model="deepseek-chat",
)

result = llm.invoke("你好！")
print(result.content)
```

---

### 文本嵌入（Embeddings）

```python
response = client.embeddings.create(
    model="text-embedding-3-small",  # 使用管理员开放的嵌入模型
    input="需要向量化的文本",
)
print(response.data[0].embedding)
```

---

## 查看账号与用量

**查看当前 Key 信息：**

```bash
curl http://<服务器地址>:8080/v1/account/key \
  -H "Authorization: Bearer brg_你的密钥"
```

**查看用量统计：**

```bash
curl http://<服务器地址>:8080/v1/account/usage \
  -H "Authorization: Bearer brg_你的密钥"
```

---

## 常见问题

**Q: 报错 `401 Unauthorized`**
- 检查 `Authorization` 头格式是否为 `Bearer brg_你的密钥`
- 确认 Key 未过期且未被禁用，联系管理员确认

**Q: 报错 `403 Forbidden` 或提示模型不可用**
- 你的 Key 可能未被授权使用该模型，联系管理员开通

**Q: 报错 `429 Too Many Requests`**
- 已触发速率限制（每分钟请求数上限），稍后重试或联系管理员调整配额

**Q: 请求超时**
- 复杂推理模型（如 `deepseek-reasoner`）响应较慢，建议开启流式输出（`stream: true`）或增大客户端超时时间

**Q: 如何在 ChatBox / Cherry Studio / LobeChat 等客户端使用？**
- 在客户端设置中，选择 `OpenAI` 兼容模式
- Base URL 填写 `http://<服务器地址>:8080/v1`
- API Key 填写 `brg_你的密钥`
