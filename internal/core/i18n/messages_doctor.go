package i18n

// Messages printed by `gitpal doctor`.
const (
	MsgDoctorShort   Key = "doctor.short"
	MsgDoctorTitle   Key = "doctor.title"
	MsgDoctorSummary Key = "doctor.summary"

	MsgDoctorGitOK          Key = "doctor.git.ok"
	MsgDoctorGitMissing     Key = "doctor.git.missing"
	MsgDoctorGitMissingHint Key = "doctor.git.missing.hint"
	MsgDoctorGitTooOld      Key = "doctor.git.too_old"
	MsgDoctorGitTooOldHint  Key = "doctor.git.too_old.hint"
	MsgDoctorGitError       Key = "doctor.git.error"

	MsgDoctorIdentityOK          Key = "doctor.identity.ok"
	MsgDoctorIdentityMissing     Key = "doctor.identity.missing"
	MsgDoctorIdentityMissingHint Key = "doctor.identity.missing.hint"
	MsgDoctorIdentityError       Key = "doctor.identity.error"

	MsgDoctorConfigOK       Key = "doctor.config.ok"
	MsgDoctorConfigDefaults Key = "doctor.config.defaults"
	MsgDoctorConfigWarnings Key = "doctor.config.warnings"
	MsgDoctorConfigInvalid  Key = "doctor.config.invalid"
	MsgDoctorConfigHint     Key = "doctor.config.hint"

	MsgDoctorGitHubTokenOK      Key = "doctor.github.token.ok"
	MsgDoctorGitHubTokenMissing Key = "doctor.github.token.missing"
	MsgDoctorGitHubCLIOK        Key = "doctor.github.cli.ok"
	MsgDoctorGitHubCLIMissing   Key = "doctor.github.cli.missing"
	MsgDoctorGitHubCLIHint      Key = "doctor.github.cli.hint"

	MsgDoctorAIOff        Key = "doctor.ai.off"
	MsgDoctorAILocal      Key = "doctor.ai.local"
	MsgDoctorAIKeyOK      Key = "doctor.ai.key.ok"
	MsgDoctorAIKeyMissing Key = "doctor.ai.key.missing"

	MsgDoctorKeychainError Key = "doctor.keychain.error"
)

var doctorMessages = map[Key]message{
	MsgDoctorShort: {
		tr: "Ortamı kontrol et: git sürümü, kimlik, ayarlar, GitHub token'ı, AI anahtarı",
		en: "Check your setup: git version, identity, config, GitHub token, AI key",
	},
	MsgDoctorTitle: {
		tr: "gitpal doctor — ortam kontrolü",
		en: "gitpal doctor — environment check",
	},
	MsgDoctorSummary: {
		tr: "%d tamam, %d uyarı, %d hata",
		en: "%d ok, %d warnings, %d failed",
	},

	MsgDoctorGitOK: {
		tr: "git %s (en az %s gerekli)",
		en: "git %s (%s or newer required)",
	},
	MsgDoctorGitMissing: {
		tr: "git bulunamadı",
		en: "git was not found",
	},
	MsgDoctorGitMissingHint: {
		tr: "git'i kur ve PATH'te olduğundan emin ol: https://git-scm.com/downloads",
		en: "Install git and make sure it is on your PATH: https://git-scm.com/downloads",
	},
	MsgDoctorGitTooOld: {
		tr: "git %s çok eski; en az %s gerekli",
		en: "git %s is too old; %s or newer is required",
	},
	MsgDoctorGitTooOldHint: {
		tr: "git'i güncelle: https://git-scm.com/downloads",
		en: "Upgrade git: https://git-scm.com/downloads",
	},
	MsgDoctorGitError: {
		tr: "git sürümü okunamadı: %s",
		en: "could not read the git version: %s",
	},

	MsgDoctorIdentityOK: {
		tr: "Git kimliği: %s <%s>",
		en: "Git identity: %s <%s>",
	},
	MsgDoctorIdentityMissing: {
		tr: "Git kimliği eksik: %s ayarlı değil (commit atılamaz)",
		en: "Git identity is incomplete: %s is not set (commits will fail)",
	},
	MsgDoctorIdentityMissingHint: {
		tr: "git config --global user.name \"Ad Soyad\" && git config --global user.email \"sen@ornek.com\"",
		en: "git config --global user.name \"Your Name\" && git config --global user.email \"you@example.com\"",
	},
	MsgDoctorIdentityError: {
		tr: "Git kimliği okunamadı: %s",
		en: "could not read the git identity: %s",
	},

	MsgDoctorConfigOK: {
		tr: "Ayarlar: %s",
		en: "Config: %s",
	},
	MsgDoctorConfigDefaults: {
		tr: "Ayarlar: varsayılanlar kullanılıyor (dosya yok: %s)",
		en: "Config: using defaults (no file at %s)",
	},
	MsgDoctorConfigWarnings: {
		tr: "Ayarlar: %s — %d uyarı",
		en: "Config: %s — %d warnings",
	},
	MsgDoctorConfigInvalid: {
		tr: "Ayarlar yüklenemedi; varsayılanlarla devam ediliyor",
		en: "Config could not be loaded; continuing with defaults",
	},
	MsgDoctorConfigHint: {
		tr: "dosyayı düzelt: %s",
		en: "fix the file: %s",
	},

	MsgDoctorGitHubTokenOK: {
		tr: "GitHub: token keychain'de kayıtlı",
		en: "GitHub: token found in the keychain",
	},
	MsgDoctorGitHubTokenMissing: {
		tr: "GitHub: token yok — GitHub özellikleri devre dışı (temel git akışı etkilenmez)",
		en: "GitHub: no token — GitHub features are disabled (the core git workflow is unaffected)",
	},
	MsgDoctorGitHubCLIOK: {
		tr: "GitHub: gh CLI bulundu (%s); token gh üzerinden okunacak",
		en: "GitHub: gh CLI found (%s); the token will be read through gh",
	},
	MsgDoctorGitHubCLIMissing: {
		tr: "GitHub: gh CLI bulunamadı — GitHub özellikleri devre dışı (temel git akışı etkilenmez)",
		en: "GitHub: gh CLI was not found — GitHub features are disabled (the core git workflow is unaffected)",
	},
	MsgDoctorGitHubCLIHint: {
		tr: "gh kur (https://cli.github.com) ya da ayarlarda `github.auth: device-flow` kullan",
		en: "Install gh (https://cli.github.com) or set `github.auth: device-flow` in your config",
	},

	MsgDoctorAIOff: {
		tr: "AI: kapalı (ai.provider: none) — commit mesajını elle yazarsın",
		en: "AI: disabled (ai.provider: none) — you write commit messages by hand",
	},
	MsgDoctorAILocal: {
		tr: "AI: %s (yerel model; anahtar gerekmez, veri dışarı çıkmaz)",
		en: "AI: %s (local model; no key needed, no data leaves your machine)",
	},
	MsgDoctorAIKeyOK: {
		tr: "AI: %s anahtarı keychain'de kayıtlı",
		en: "AI: %s API key found in the keychain",
	},
	MsgDoctorAIKeyMissing: {
		tr: "AI: %s anahtarı yok — AI commit mesajı devre dışı, mesajı elle yazabilirsin",
		en: "AI: no %s API key — AI commit messages are disabled; you can still write them by hand",
	},

	MsgDoctorKeychainError: {
		tr: "Keychain'e erişilemedi: %s",
		en: "Could not access the keychain: %s",
	},
}
