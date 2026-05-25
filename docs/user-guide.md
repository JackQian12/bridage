# User Integration Guide

This guide is for end users of the bridage gateway. Your administrator will provide you with a `brg_`-prefixed API key. Follow the steps below to get started.

---

## Connection Details

| Item | Value |
|---|---|
| **Base URL** | `http://<server-address>:8080/v1` |
| **Authentication** | `Authorization: Bearer brg_your-key` |
| **Protocol** | Fully compatible with the OpenAI API |

> Replace `<server-address>` with the actual address provided by your administrator.

---

## List Available Models

```bash
curl http://<server-address>:8080/v1/models \
  -H "Authorization: Bearer brg_your-key"
```

---

## Usage Examples

### curl

```bash
curl http://<server-address>:8080/v1/chat/completions \
  -H "Authorization: Bearer brg_your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

**Streaming:**

```bash
curl http://<server-address>:8080/v1/chat/completions \
  -H "Authorization: Bearer brg_your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat",
    "stream": true,
    "messages": [
      {"role": "user", "content": "Write a short poem."}
    ]
  }'
```

---

### Python (openai library)

```bash
pip install openai
```

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://<server-address>:8080/v1",
    api_key="brg_your-key",
)

# Standard call
response = client.chat.completions.create(
    model="deepseek-chat",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Hello!"},
    ],
)
print(response.choices[0].message.content)
```

**Streaming:**

```python
stream = client.chat.completions.create(
    model="deepseek-chat",
    messages=[{"role": "user", "content": "Tell me a story."}],
    stream=True,
)
for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
```

---

### Node.js / TypeScript (openai library)

```bash
npm install openai
```

```typescript
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://<server-address>:8080/v1",
  apiKey: "brg_your-key",
});

const response = await client.chat.completions.create({
  model: "deepseek-chat",
  messages: [{ role: "user", content: "Hello!" }],
});
console.log(response.choices[0].message.content);
```

---

### LangChain (Python)

```python
from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    base_url="http://<server-address>:8080/v1",
    api_key="brg_your-key",
    model="deepseek-chat",
)

result = llm.invoke("Hello!")
print(result.content)
```

---

### Embeddings

```python
response = client.embeddings.create(
    model="text-embedding-3-small",  # use a model enabled by your administrator
    input="Text to embed",
)
print(response.data[0].embedding)
```

---

## Account & Usage

**Get current key info:**

```bash
curl http://<server-address>:8080/v1/account/key \
  -H "Authorization: Bearer brg_your-key"
```

**Get usage statistics:**

```bash
curl http://<server-address>:8080/v1/account/usage \
  -H "Authorization: Bearer brg_your-key"
```

---

## FAQ

**Q: I get `401 Unauthorized`**
- Verify the `Authorization` header is formatted as `Bearer brg_your-key`
- Confirm the key has not expired or been disabled — contact your administrator

**Q: I get `403 Forbidden` or "model not available"**
- Your key may not be authorized for that model — ask your administrator to enable access

**Q: I get `429 Too Many Requests`**
- You have hit the rate limit (max requests per minute) — wait and retry, or ask your administrator to raise your quota

**Q: Requests time out**
- Complex reasoning models (e.g. `deepseek-reasoner`) can be slow — enable streaming (`stream: true`) or increase your client timeout

**Q: How do I use this with ChatBox / Cherry Studio / LobeChat?**
- In the client settings, select **OpenAI compatible** mode
- Set the Base URL to `http://<server-address>:8080/v1`
- Set the API Key to `brg_your-key`
