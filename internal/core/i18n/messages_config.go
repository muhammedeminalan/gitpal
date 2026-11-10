package i18n

// Messages about loading and validating configuration.
const (
	MsgConfigUnknownKey       Key = "config.warn.unknown_key"
	MsgConfigRepoKeyLocked    Key = "config.warn.repo_key_locked"
	MsgConfigRepoWeaker       Key = "config.warn.repo_weaker"
	MsgConfigRepoFileIgnored  Key = "config.warn.repo_file_ignored"
	MsgConfigInvalidChoice    Key = "config.error.invalid_choice"
	MsgConfigInvalidRange     Key = "config.error.invalid_range"
	MsgConfigUnreadable       Key = "config.error.unreadable"
	MsgConfigNoHome           Key = "config.error.no_home"
	MsgConfigRepoFileLocation Key = "config.repo_file_location"
)

var configMessages = map[Key]message{
	MsgConfigUnknownKey: {
		tr: "%s: bilinmeyen ayar \"%s\" yok sayıldı",
		en: "%s: unknown setting \"%s\" was ignored",
	},
	MsgConfigRepoKeyLocked: {
		tr: "%s: \"%s\" repo dosyasından okunmaz, yalnızca kendi global ayarından okunur",
		en: "%s: \"%s\" is never read from a repository file, only from your global config",
	},
	MsgConfigRepoWeaker: {
		tr: "%s: \"%s\" korumayı zayıflattığı için yok sayıldı (global ayarın geçerli)",
		en: "%s: \"%s\" was ignored because it would weaken protection (your global setting stays in effect)",
	},
	MsgConfigRepoFileIgnored: {
		tr: "%s tümüyle yok sayıldı: %s",
		en: "%s was ignored entirely: %s",
	},
	MsgConfigInvalidChoice: {
		tr: "geçersiz ayar: %s = \"%s\" (izin verilenler: %s)",
		en: "invalid setting: %s = \"%s\" (allowed: %s)",
	},
	MsgConfigInvalidRange: {
		tr: "geçersiz ayar: %s = %d (%d ile %d arasında olmalı)",
		en: "invalid setting: %s = %d (must be between %d and %d)",
	},
	MsgConfigUnreadable: {
		tr: "ayar dosyası kullanılamıyor (%s): %s",
		en: "cannot use the config file (%s): %s",
	},
	MsgConfigNoHome: {
		tr: "ev dizini bulunamadı, ayar dosyasının yeri belirlenemiyor: %s",
		en: "could not find the home directory, so the config file location is unknown: %s",
	},
	MsgConfigRepoFileLocation: {
		tr: "repo ayarı: %s",
		en: "repository config: %s",
	},
}
