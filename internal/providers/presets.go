package providers

import "github.com/nuts/bridage/internal/models"

// Presets returns all built-in provider templates.
func Presets() []models.ProviderPreset {
	return []models.ProviderPreset{
		openAIPreset(),
		anthropicPreset(),
		geminiPreset(),
		doubaoPreset(),
		deepSeekPreset(),
		qwenPreset(),
		kimiPreset(),
		zhipuPreset(),
		baiduQianfanPreset(),
		tencentHunyuanPreset(),
		miniMaxPreset(),
		baichuanPreset(),
		yiPreset(),
		siliconFlowPreset(),
	}
}

// PresetBySlug finds a preset by its slug.
func PresetBySlug(slug string) (models.ProviderPreset, bool) {
	for _, p := range Presets() {
		if p.Slug == slug {
			return p, true
		}
	}
	return models.ProviderPreset{}, false
}

// ─── Individual preset definitions ───────────────────────────────────────────

func openAIPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "openai",
		DisplayName: "OpenAI",
		AdapterType: models.AdapterOpenAICompatible,
		BaseURL:     "https://api.openai.com/v1",
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Responses: true, Embeddings: true, Images: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "gpt-4o", ProviderModel: "gpt-4o", Description: "GPT-4o", ContextWindow: 128000, SupportsImages: true},
			{Name: "gpt-4o-mini", ProviderModel: "gpt-4o-mini", Description: "GPT-4o mini", ContextWindow: 128000, SupportsImages: true},
			{Name: "gpt-4-turbo", ProviderModel: "gpt-4-turbo", Description: "GPT-4 Turbo", ContextWindow: 128000, SupportsImages: true},
			{Name: "gpt-3.5-turbo", ProviderModel: "gpt-3.5-turbo", Description: "GPT-3.5 Turbo", ContextWindow: 16385},
			{Name: "o1", ProviderModel: "o1", Description: "o1 reasoning", ContextWindow: 200000, SupportsImages: true},
			{Name: "o3-mini", ProviderModel: "o3-mini", Description: "o3-mini reasoning", ContextWindow: 200000},
			{Name: "text-embedding-3-small", ProviderModel: "text-embedding-3-small", Description: "Embedding 3 small", ContextWindow: 8191},
			{Name: "text-embedding-3-large", ProviderModel: "text-embedding-3-large", Description: "Embedding 3 large", ContextWindow: 8191},
			{Name: "dall-e-3", ProviderModel: "dall-e-3", Description: "DALL·E 3 image generation"},
		},
	}
}

func anthropicPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "anthropic",
		DisplayName: "Anthropic",
		AdapterType: models.AdapterAnthropic,
		BaseURL:     "https://api.anthropic.com",
		AuthHeader:  "x-api-key",
		AuthScheme:  "",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "claude-opus-4-5", ProviderModel: "claude-opus-4-5-20251101", Description: "Claude Opus 4.5", ContextWindow: 200000, SupportsImages: true},
			{Name: "claude-sonnet-4-5", ProviderModel: "claude-sonnet-4-5-20251001", Description: "Claude Sonnet 4.5", ContextWindow: 200000, SupportsImages: true},
			{Name: "claude-3-5-sonnet", ProviderModel: "claude-3-5-sonnet-20241022", Description: "Claude 3.5 Sonnet", ContextWindow: 200000, SupportsImages: true},
			{Name: "claude-3-5-haiku", ProviderModel: "claude-3-5-haiku-20241022", Description: "Claude 3.5 Haiku", ContextWindow: 200000, SupportsImages: true},
			{Name: "claude-3-opus", ProviderModel: "claude-3-opus-20240229", Description: "Claude 3 Opus", ContextWindow: 200000, SupportsImages: true},
			{Name: "claude-3-haiku", ProviderModel: "claude-3-haiku-20240307", Description: "Claude 3 Haiku", ContextWindow: 200000, SupportsImages: true},
		},
	}
}

func geminiPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "gemini",
		DisplayName: "Google Gemini",
		AdapterType: models.AdapterGemini,
		BaseURL:     "https://generativelanguage.googleapis.com",
		AuthHeader:  "x-goog-api-key",
		AuthScheme:  "",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Embeddings: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "gemini-2.0-flash", ProviderModel: "gemini-2.0-flash-001", Description: "Gemini 2.0 Flash", ContextWindow: 1000000, SupportsImages: true},
			{Name: "gemini-1.5-pro", ProviderModel: "gemini-1.5-pro-002", Description: "Gemini 1.5 Pro", ContextWindow: 2000000, SupportsImages: true},
			{Name: "gemini-1.5-flash", ProviderModel: "gemini-1.5-flash-002", Description: "Gemini 1.5 Flash", ContextWindow: 1000000, SupportsImages: true},
			{Name: "text-embedding-004", ProviderModel: "text-embedding-004", Description: "Gemini Text Embedding", ContextWindow: 2048},
		},
	}
}

func doubaoPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "doubao",
		DisplayName: "字节跳动 豆包 (Volcengine Ark)",
		AdapterType: models.AdapterOpenAICompatible,
		BaseURL:     "https://ark.cn-beijing.volces.com/api/v3",
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Embeddings: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "doubao-pro-4k", ProviderModel: "doubao-pro-4k", Description: "豆包 Pro 4K", ContextWindow: 4096},
			{Name: "doubao-pro-32k", ProviderModel: "doubao-pro-32k", Description: "豆包 Pro 32K", ContextWindow: 32768},
			{Name: "doubao-pro-128k", ProviderModel: "doubao-pro-128k", Description: "豆包 Pro 128K", ContextWindow: 131072},
			{Name: "doubao-lite-4k", ProviderModel: "doubao-lite-4k", Description: "豆包 Lite 4K", ContextWindow: 4096},
			{Name: "doubao-lite-32k", ProviderModel: "doubao-lite-32k", Description: "豆包 Lite 32K", ContextWindow: 32768},
			{Name: "doubao-vision-pro-32k", ProviderModel: "doubao-vision-pro-32k", Description: "豆包视觉 Pro 32K", ContextWindow: 32768, SupportsImages: true},
		},
	}
}

func deepSeekPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "deepseek",
		DisplayName: "DeepSeek",
		AdapterType: models.AdapterOpenAICompatible,
		BaseURL:     "https://api.deepseek.com",
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "deepseek-chat", ProviderModel: "deepseek-chat", Description: "DeepSeek V3", ContextWindow: 65536},
			{Name: "deepseek-reasoner", ProviderModel: "deepseek-reasoner", Description: "DeepSeek R1", ContextWindow: 65536},
		},
	}
}

func qwenPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "qwen",
		DisplayName: "阿里云 通义千问 (Qwen / DashScope)",
		AdapterType: models.AdapterOpenAICompatible,
		BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Embeddings: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "qwen-turbo", ProviderModel: "qwen-turbo", Description: "通义千问 Turbo", ContextWindow: 131072},
			{Name: "qwen-plus", ProviderModel: "qwen-plus", Description: "通义千问 Plus", ContextWindow: 131072},
			{Name: "qwen-max", ProviderModel: "qwen-max", Description: "通义千问 Max", ContextWindow: 32768},
			{Name: "qwen-long", ProviderModel: "qwen-long", Description: "通义千问 Long", ContextWindow: 10000000},
			{Name: "qwen-vl-plus", ProviderModel: "qwen-vl-plus", Description: "通义千问 VL Plus", ContextWindow: 32768, SupportsImages: true},
			{Name: "qwen-vl-max", ProviderModel: "qwen-vl-max", Description: "通义千问 VL Max", ContextWindow: 32768, SupportsImages: true},
			{Name: "text-embedding-v3", ProviderModel: "text-embedding-v3", Description: "Qwen Embedding v3", ContextWindow: 8192},
		},
	}
}

func kimiPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "kimi",
		DisplayName: "Moonshot AI Kimi",
		AdapterType: models.AdapterOpenAICompatible,
		BaseURL:     "https://api.moonshot.cn/v1",
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "moonshot-v1-8k", ProviderModel: "moonshot-v1-8k", Description: "Kimi 8K", ContextWindow: 8192},
			{Name: "moonshot-v1-32k", ProviderModel: "moonshot-v1-32k", Description: "Kimi 32K", ContextWindow: 32768},
			{Name: "moonshot-v1-128k", ProviderModel: "moonshot-v1-128k", Description: "Kimi 128K", ContextWindow: 131072},
		},
	}
}

func zhipuPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "zhipu",
		DisplayName: "智谱 GLM (Zhipu AI)",
		AdapterType: models.AdapterOpenAICompatible,
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4",
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Embeddings: true, Images: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "glm-4", ProviderModel: "glm-4", Description: "GLM-4", ContextWindow: 128000},
			{Name: "glm-4-air", ProviderModel: "glm-4-air", Description: "GLM-4 Air", ContextWindow: 128000},
			{Name: "glm-4-flash", ProviderModel: "glm-4-flash", Description: "GLM-4 Flash (免费)", ContextWindow: 128000},
			{Name: "glm-4v", ProviderModel: "glm-4v", Description: "GLM-4V 视觉", ContextWindow: 2000, SupportsImages: true},
			{Name: "embedding-3", ProviderModel: "embedding-3", Description: "GLM Embedding-3", ContextWindow: 8192},
			{Name: "cogview-3-flash", ProviderModel: "cogview-3-flash", Description: "CogView-3 Flash (图像生成)"},
		},
	}
}

func baiduQianfanPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "baidu",
		DisplayName: "百度 千帆 / 文心一言 (Qianfan)",
		AdapterType: models.AdapterOpenAICompatible,
		BaseURL:     "https://qianfan.baidubce.com/v2",
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Embeddings: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "ernie-4.0-turbo-8k", ProviderModel: "ernie-4.0-turbo-8k", Description: "文心 4.0 Turbo 8K", ContextWindow: 8192},
			{Name: "ernie-3.5-8k", ProviderModel: "ernie-3.5-8k", Description: "文心 3.5 8K", ContextWindow: 8192},
			{Name: "ernie-speed-8k", ProviderModel: "ernie-speed-8k", Description: "文心 Speed 8K", ContextWindow: 8192},
			{Name: "ernie-lite-8k", ProviderModel: "ernie-lite-8k", Description: "文心 Lite 8K", ContextWindow: 8192},
		},
	}
}

func tencentHunyuanPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "hunyuan",
		DisplayName: "腾讯 混元 (Hunyuan)",
		AdapterType: models.AdapterOpenAICompatible,
		BaseURL:     "https://api.hunyuan.cloud.tencent.com/v1",
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Embeddings: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "hunyuan-turbo", ProviderModel: "hunyuan-turbo", Description: "混元 Turbo", ContextWindow: 32768},
			{Name: "hunyuan-pro", ProviderModel: "hunyuan-pro", Description: "混元 Pro", ContextWindow: 32768},
			{Name: "hunyuan-standard", ProviderModel: "hunyuan-standard", Description: "混元 Standard", ContextWindow: 32768},
			{Name: "hunyuan-lite", ProviderModel: "hunyuan-lite", Description: "混元 Lite", ContextWindow: 262144},
			{Name: "hunyuan-embedding", ProviderModel: "hunyuan-embedding", Description: "混元 Embedding", ContextWindow: 1024},
		},
	}
}

func miniMaxPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "minimax",
		DisplayName: "MiniMax",
		AdapterType: models.AdapterOpenAICompatible,
		BaseURL:     "https://api.minimax.chat/v1",
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "abab6.5s-chat", ProviderModel: "abab6.5s-chat", Description: "MiniMax abab6.5s", ContextWindow: 245760},
			{Name: "abab6.5-chat", ProviderModel: "abab6.5-chat", Description: "MiniMax abab6.5", ContextWindow: 245760},
			{Name: "abab5.5s-chat", ProviderModel: "abab5.5s-chat", Description: "MiniMax abab5.5s", ContextWindow: 245760},
		},
	}
}

func baichuanPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "baichuan",
		DisplayName: "百川智能 (Baichuan AI)",
		AdapterType: models.AdapterOpenAICompatible,
		BaseURL:     "https://api.baichuan-ai.com/v1",
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Embeddings: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "Baichuan4", ProviderModel: "Baichuan4", Description: "Baichuan4", ContextWindow: 32768},
			{Name: "Baichuan3-Turbo", ProviderModel: "Baichuan3-Turbo", Description: "Baichuan3 Turbo", ContextWindow: 32768},
			{Name: "Baichuan3-Turbo-128k", ProviderModel: "Baichuan3-Turbo-128k", Description: "Baichuan3 Turbo 128K", ContextWindow: 131072},
			{Name: "Baichuan-Text-Embedding", ProviderModel: "Baichuan-Text-Embedding", Description: "Baichuan Embedding", ContextWindow: 512},
		},
	}
}

func yiPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "yi",
		DisplayName: "零一万物 Yi (01.AI)",
		AdapterType: models.AdapterOpenAICompatible,
		BaseURL:     "https://api.lingyiwanwu.com/v1",
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "yi-lightning", ProviderModel: "yi-lightning", Description: "Yi Lightning", ContextWindow: 16384},
			{Name: "yi-large", ProviderModel: "yi-large", Description: "Yi Large", ContextWindow: 32768},
			{Name: "yi-medium", ProviderModel: "yi-medium", Description: "Yi Medium", ContextWindow: 16384},
			{Name: "yi-large-rag", ProviderModel: "yi-large-rag", Description: "Yi Large RAG", ContextWindow: 16384},
			{Name: "yi-vision", ProviderModel: "yi-vision", Description: "Yi Vision", ContextWindow: 4096, SupportsImages: true},
		},
	}
}

func siliconFlowPreset() models.ProviderPreset {
	return models.ProviderPreset{
		Slug:        "siliconflow",
		DisplayName: "SiliconFlow",
		AdapterType: models.AdapterOpenAICompatible,
		BaseURL:     "https://api.siliconflow.cn/v1",
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
		Capabilities: models.ProviderCapabilities{
			Chat: true, Embeddings: true, Images: true, Streaming: true,
		},
		DefaultModels: []models.PresetModel{
			{Name: "deepseek-ai/DeepSeek-V3", ProviderModel: "deepseek-ai/DeepSeek-V3", Description: "DeepSeek V3 via SiliconFlow", ContextWindow: 65536},
			{Name: "deepseek-ai/DeepSeek-R1", ProviderModel: "deepseek-ai/DeepSeek-R1", Description: "DeepSeek R1 via SiliconFlow", ContextWindow: 65536},
			{Name: "Qwen/Qwen2.5-72B-Instruct", ProviderModel: "Qwen/Qwen2.5-72B-Instruct", Description: "Qwen2.5 72B via SiliconFlow", ContextWindow: 131072},
			{Name: "meta-llama/Meta-Llama-3.1-70B-Instruct", ProviderModel: "meta-llama/Meta-Llama-3.1-70B-Instruct", Description: "Llama 3.1 70B", ContextWindow: 131072},
			{Name: "BAAI/bge-m3", ProviderModel: "BAAI/bge-m3", Description: "BGE-M3 Embedding", ContextWindow: 8192},
			{Name: "black-forest-labs/FLUX.1-schnell", ProviderModel: "black-forest-labs/FLUX.1-schnell", Description: "FLUX.1 Schnell 图像"},
		},
	}
}
