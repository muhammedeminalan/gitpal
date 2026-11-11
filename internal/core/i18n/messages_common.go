package i18n

// Messages shared by every command.
const (
	MsgRootShort    Key = "cli.root.short"
	MsgRootLong     Key = "cli.root.long"
	MsgFlagJSON     Key = "cli.flag.json"
	MsgFlagNoColor  Key = "cli.flag.no_color"
	MsgFlagExplain  Key = "cli.flag.explain"
	MsgFlagVersion  Key = "cli.flag.version"
	MsgFlagHelp     Key = "cli.flag.help"
	MsgErrorPrefix  Key = "cli.error.prefix"
	MsgHintPrefix   Key = "cli.hint.prefix"
	MsgCanceled     Key = "cli.canceled"
	MsgPanic        Key = "cli.panic"
	MsgPanicLogged  Key = "cli.panic.logged"
	MsgExplainTitle Key = "cli.explain.title"

	MsgHelpUsage       Key = "cli.help.usage"
	MsgHelpAliases     Key = "cli.help.aliases"
	MsgHelpExamples    Key = "cli.help.examples"
	MsgHelpCommands    Key = "cli.help.commands"
	MsgHelpFlags       Key = "cli.help.flags"
	MsgHelpGlobalFlags Key = "cli.help.global_flags"
	MsgHelpMore        Key = "cli.help.more"
	MsgHelpCommandWord Key = "cli.help.command_word"
	MsgHelpFlagsWord   Key = "cli.help.flags_word"
	MsgHelpShort       Key = "cli.help.short"
	MsgCompletionShort Key = "cli.completion.short"
)

var commonMessages = map[Key]message{
	MsgHelpUsage:       {tr: "Kullanım:", en: "Usage:"},
	MsgHelpAliases:     {tr: "Diğer adları:", en: "Aliases:"},
	MsgHelpExamples:    {tr: "Örnekler:", en: "Examples:"},
	MsgHelpCommands:    {tr: "Komutlar:", en: "Commands:"},
	MsgHelpFlags:       {tr: "Seçenekler:", en: "Flags:"},
	MsgHelpGlobalFlags: {tr: "Genel seçenekler:", en: "Global flags:"},
	MsgHelpMore:        {tr: "Bir komutun ayrıntısı için: %s", en: "For more about a command: %s"},
	MsgHelpCommandWord: {tr: "komut", en: "command"},
	MsgHelpFlagsWord:   {tr: "seçenekler", en: "flags"},
	MsgHelpShort:       {tr: "bir komutun yardımını göster", en: "show help for a command"},
	MsgCompletionShort: {tr: "kabuk için otomatik tamamlama betiği üret", en: "generate a shell completion script"},
	MsgRootShort: {
		tr: "Git akışını tek yerden, güvenli ve hızlı yürüt",
		en: "Run your git workflow from one place, safely and fast",
	},
	MsgRootLong: {
		tr: "gitpal; add, commit, push ve diğer git işlemlerini tek yerden yapar.\n" +
			"Geri dönüşü zor işlemleri bir güvenlik katmanından geçirir ve arka planda\n" +
			"hangi git komutunu çalıştırdığını her zaman gösterebilir (--explain).",
		en: "gitpal runs add, commit, push and the rest of your git workflow from one place.\n" +
			"Hard-to-undo operations go through a safety layer, and gitpal can always show\n" +
			"which git commands it runs behind the scenes (--explain).",
	},
	MsgFlagJSON: {
		tr: "çıktıyı JSON olarak ver (script'ler için)",
		en: "print output as JSON (for scripts)",
	},
	MsgFlagNoColor: {
		tr: "renkli çıktıyı kapat",
		en: "disable colored output",
	},
	MsgFlagExplain: {
		tr: "arka planda çalışan git komutlarını göster",
		en: "show the git commands gitpal runs behind the scenes",
	},
	MsgFlagVersion: {
		tr: "gitpal sürümünü göster",
		en: "show the gitpal version",
	},
	MsgFlagHelp: {
		tr: "yardımı göster",
		en: "show help",
	},
	MsgErrorPrefix: {
		tr: "Hata",
		en: "Error",
	},
	MsgHintPrefix: {
		tr: "Çözüm",
		en: "Fix",
	},
	MsgCanceled: {
		tr: "İptal edildi.",
		en: "Canceled.",
	},
	MsgPanic: {
		tr: "gitpal beklenmeyen bir hatayla karşılaştı ve durdu. Reponun durumunu `git status` ile kontrol et.",
		en: "gitpal hit an unexpected error and stopped. Check the state of your repository with `git status`.",
	},
	MsgPanicLogged: {
		tr: "Hata raporu: %s",
		en: "Error report: %s",
	},
	MsgExplainTitle: {
		tr: "Çalışan git komutları",
		en: "Git commands that ran",
	},
}
