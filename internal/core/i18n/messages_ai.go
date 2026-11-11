package i18n

// Texts of the AI features (spec §6).
const (
	MsgAINoKey       Key = "ai.no_key"
	MsgAINoModel     Key = "ai.no_model"
	MsgAIAuth        Key = "ai.auth"
	MsgAIRateLimit   Key = "ai.rate_limit"
	MsgAIUnavailable Key = "ai.unavailable"
	MsgAINetwork     Key = "ai.network"
	MsgAIRefused     Key = "ai.refused"
	MsgAIEmpty       Key = "ai.empty"
	MsgAIDisabled    Key = "ai.disabled"

	MsgAIConsent        Key = "ai.consent"
	MsgAIConsentLocal   Key = "ai.consent_local"
	MsgAIConsentDenied  Key = "ai.consent_denied"
	MsgAIConsentNeeded  Key = "ai.consent_needed"
	MsgAIWorking        Key = "ai.working"
	MsgAISent           Key = "ai.sent"
	MsgAIWithheld       Key = "ai.withheld"
	MsgAITruncated      Key = "ai.truncated"
	MsgAISuggestions    Key = "ai.suggestions"
	MsgAISplit          Key = "ai.split"
	MsgAIChoose         Key = "ai.choose"
	MsgAIFallbackManual Key = "ai.fallback_manual"

	MsgAuthShort       Key = "cli.auth.short"
	MsgAuthAIShort     Key = "cli.auth.ai.short"
	MsgAuthPrompt      Key = "cli.auth.prompt"
	MsgAuthSaved       Key = "cli.auth.saved"
	MsgAuthEmpty       Key = "cli.auth.empty"
	MsgAuthNotNeeded   Key = "cli.auth.not_needed"
	MsgAuthLogoutShort Key = "cli.auth.logout.short"
	MsgAuthRemoved     Key = "cli.auth.removed"
)

var aiMessages = map[Key]message{
	MsgAINoKey: {
		tr: "%s için API anahtarı yok. `gitpal auth ai` ile ekle.",
		en: "There is no API key for %s. Add one with `gitpal auth ai`.",
	},
	MsgAINoModel: {
		tr: "%s için model seçilmemiş. `gitpal config set ai.model <model>` ile ayarla.",
		en: "No model is set for %s. Set one with `gitpal config set ai.model <model>`.",
	},
	MsgAIAuth: {
		tr: "%s API anahtarını kabul etmedi. `gitpal auth ai` ile yenisini ekle.",
		en: "%s did not accept the API key. Add a new one with `gitpal auth ai`.",
	},
	MsgAIRateLimit: {
		tr: "%s şu an çok fazla istek alıyor (rate limit). Biraz sonra yeniden dene.",
		en: "%s is receiving too many requests right now (rate limit). Try again shortly.",
	},
	MsgAIUnavailable: {
		tr: "%s şu an cevap veremiyor. Mesajı elle yazabilirsin.",
		en: "%s cannot answer right now. You can write the message yourself.",
	},
	MsgAINetwork: {
		tr: "%s sunucusuna ulaşılamadı. İnternet bağlantını kontrol et.",
		en: "Could not reach %s. Check your internet connection.",
	},
	MsgAIRefused: {
		tr: "%s bu isteği yanıtlamadı. Mesajı elle yazabilirsin.",
		en: "%s declined to answer. You can write the message yourself.",
	},
	MsgAIEmpty: {
		tr: "%s kullanılabilir bir cevap vermedi. Mesajı elle yazabilirsin.",
		en: "%s gave no usable answer. You can write the message yourself.",
	},
	MsgAIDisabled: {
		tr: "Yapay zekâ kapalı (ai.provider = %s). `gitpal config set ai.provider anthropic` ile aç.",
		en: "AI is turned off (ai.provider = %s). Turn it on with `gitpal config set ai.provider anthropic`.",
	},
	MsgAIConsent: {
		tr: "Commit mesajı önerisi için stage'deki değişikliklerin (sırlar temizlenerek) %s sağlayıcısına gönderilecek. " +
			"Kabul ediyor musun? Bu onay kalıcı olarak kaydedilir. [e/H] ",
		en: "To suggest a commit message, your staged changes (with secrets removed) are sent to %s. " +
			"Do you agree? Your answer is saved. [y/N] ",
	},
	MsgAIConsentLocal: {
		tr: "Değişikliklerin %s adresindeki Ollama'ya gönderilecek. Kabul ediyor musun? [e/H] ",
		en: "Your changes are sent to the Ollama at %s. Do you agree? [y/N] ",
	},
	MsgAIConsentDenied: {
		tr: "Onay verilmedi; diff hiçbir yere gönderilmedi.",
		en: "No consent given; the diff was not sent anywhere.",
	},
	MsgAIConsentNeeded: {
		tr: "Diff'in yapay zekâya gönderilmesine henüz onay verilmedi. Bir terminalde `gitpal commit --ai` çalıştır " +
			"ya da `gitpal config set ai.consent_given true` ile onay ver.",
		en: "You have not agreed yet to sending your diff to the AI. Run `gitpal commit --ai` in a terminal " +
			"or agree with `gitpal config set ai.consent_given true`.",
	},
	MsgAIWorking: {
		tr: "%s commit mesajı öneriyor…",
		en: "%s is suggesting a commit message…",
	},
	MsgAISent: {
		tr: "Gönderilen: %d karakter (~%d token)",
		en: "Sent: %d characters (~%d tokens)",
	},
	MsgAIWithheld: {
		tr: "Gönderilmeyen (sır içerebilir): %s",
		en: "Not sent (may contain secrets): %s",
	},
	MsgAITruncated: {
		tr: "Diff büyük olduğu için her dosyadan bir kısmı gönderildi.",
		en: "The diff was large, so only part of each file was sent.",
	},
	MsgAISuggestions: {
		tr: "Öneriler:",
		en: "Suggestions:",
	},
	MsgAISplit: {
		tr: "Öneri: bu değişiklik ayrı commit'lere bölünebilir — %s",
		en: "Advice: this change could be split into separate commits — %s",
	},
	MsgAIChoose: {
		tr: "Seç (numara), y yeniden üret, m elle yaz: ",
		en: "Choose (number), r regenerate, m write your own: ",
	},
	MsgAIFallbackManual: {
		tr: "Mesajı elle yazmaya geçiliyor.",
		en: "Falling back to writing the message yourself.",
	},
	MsgAuthShort: {
		tr: "API anahtarlarını ve token'ları güvenli şekilde (keychain) kaydet",
		en: "store API keys and tokens safely (keychain)",
	},
	MsgAuthAIShort: {
		tr: "yapay zekâ sağlayıcısının API anahtarını kaydet",
		en: "store the AI provider's API key",
	},
	MsgAuthPrompt: {
		tr: "%s anahtarı (ekranda görünmez): ",
		en: "%s key (not shown): ",
	},
	MsgAuthSaved: {
		tr: "Anahtar işletim sisteminin keychain'ine kaydedildi.",
		en: "The key was saved in the system keychain.",
	},
	MsgAuthEmpty: {
		tr: "Anahtar boş olamaz.",
		en: "The key cannot be empty.",
	},
	MsgAuthNotNeeded: {
		tr: "%s için anahtar gerekmez.",
		en: "%s needs no key.",
	},
	MsgAuthLogoutShort: {
		tr: "kayıtlı anahtarı sil",
		en: "delete the stored key",
	},
	MsgAuthRemoved: {
		tr: "Anahtar silindi.",
		en: "The key was deleted.",
	},
}
