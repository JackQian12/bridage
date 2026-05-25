package providers_test

import (
	"strings"
	"testing"

	"github.com/nuts/bridage/internal/models"
	"github.com/nuts/bridage/internal/providers"
)

// knownSlugs are the 14 built-in presets that must always be present.
var knownSlugs = []string{
	"openai", "anthropic", "gemini", "doubao", "deepseek",
	"qwen", "kimi", "zhipu", "baidu", "hunyuan",
	"minimax", "baichuan", "yi", "siliconflow",
}

func TestPresets_Count(t *testing.T) {
	all := providers.Presets()
	if len(all) != 14 {
		t.Errorf("expected 14 presets, got %d", len(all))
	}
}

func TestPresets_AllKnownSlugsPresent(t *testing.T) {
	for _, slug := range knownSlugs {
		if _, ok := providers.PresetBySlug(slug); !ok {
			t.Errorf("missing preset slug: %q", slug)
		}
	}
}

func TestPresets_NoDuplicateSlugs(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range providers.Presets() {
		if seen[p.Slug] {
			t.Errorf("duplicate slug: %q", p.Slug)
		}
		seen[p.Slug] = true
	}
}

func TestPresets_RequiredFields(t *testing.T) {
	for _, p := range providers.Presets() {
		if p.Slug == "" {
			t.Error("preset has empty slug")
		}
		if p.DisplayName == "" {
			t.Errorf("preset %q has empty display name", p.Slug)
		}
		if p.BaseURL == "" {
			t.Errorf("preset %q has empty base URL", p.Slug)
		}
		if !strings.HasPrefix(p.BaseURL, "https://") {
			t.Errorf("preset %q base URL not HTTPS: %s", p.Slug, p.BaseURL)
		}
		if p.AdapterType == "" {
			t.Errorf("preset %q has empty adapter type", p.Slug)
		}
		validAdapter := p.AdapterType == models.AdapterOpenAICompatible ||
			p.AdapterType == models.AdapterAnthropic ||
			p.AdapterType == models.AdapterGemini
		if !validAdapter {
			t.Errorf("preset %q has unknown adapter type: %q", p.Slug, p.AdapterType)
		}
		if p.AuthHeader == "" {
			t.Errorf("preset %q has empty auth header", p.Slug)
		}
	}
}

func TestPresets_EachHasModels(t *testing.T) {
	for _, p := range providers.Presets() {
		if len(p.DefaultModels) == 0 {
			t.Errorf("preset %q has no default models", p.Slug)
		}
		for _, m := range p.DefaultModels {
			if m.Name == "" {
				t.Errorf("preset %q model has empty name", p.Slug)
			}
			if m.ProviderModel == "" {
				t.Errorf("preset %q model %q has empty provider_model", p.Slug, m.Name)
			}
		}
	}
}

func TestPresets_ChatCapability(t *testing.T) {
	// All presets must support chat
	for _, p := range providers.Presets() {
		if !p.Capabilities.Chat {
			t.Errorf("preset %q does not declare chat capability", p.Slug)
		}
	}
}

func TestPresetBySlug_NotFound(t *testing.T) {
	_, ok := providers.PresetBySlug("nonexistent-provider-slug")
	if ok {
		t.Error("PresetBySlug should return false for unknown slug")
	}
}

func TestPresetBySlug_CaseSensitive(t *testing.T) {
	_, ok := providers.PresetBySlug("OpenAI")
	if ok {
		t.Error("PresetBySlug should be case-sensitive; 'OpenAI' should not match 'openai'")
	}
}
