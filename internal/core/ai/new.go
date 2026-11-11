package ai

// Settings select a provider.
type Settings struct {
	// Provider is "anthropic", "openai", "ollama" or "none".
	Provider string
	Model    string
	// Key is the API key; Ollama needs none.
	Key string
	// BaseURL overrides the provider's address (an OpenAI-compatible
	// service, an Ollama on another machine, a test server).
	BaseURL string
}

// New returns the provider the settings name.
func New(s Settings) (Provider, error) {
	switch s.Provider {
	case "anthropic":
		if s.Key == "" {
			return nil, &Error{Kind: KindNoKey, Provider: s.Provider}
		}
		return NewAnthropic(s.Key, s.Model, s.BaseURL), nil
	case "openai":
		if s.Key == "" {
			return nil, &Error{Kind: KindNoKey, Provider: s.Provider}
		}
		if s.Model == "" {
			return nil, &Error{Kind: KindNoModel, Provider: s.Provider}
		}
		return NewOpenAI(s.Key, s.Model, s.BaseURL), nil
	case "ollama":
		if s.Model == "" {
			return nil, &Error{Kind: KindNoModel, Provider: s.Provider}
		}
		return NewOllama(s.Model, s.BaseURL), nil
	}
	return nil, &Error{Kind: KindDisabled, Provider: s.Provider}
}

// NeedsKey reports whether a provider authenticates with an API key.
func NeedsKey(provider string) bool { return provider == "anthropic" || provider == "openai" }
