package i18n

// catalog is every user-facing message gitpal can print. Messages live in
// per-domain files (messages_*.go) and are merged here; a key defined twice
// panics at start-up instead of silently shadowing another message.
var catalog = mergeCatalogs(
	commonMessages,
	configMessages,
	doctorMessages,
	gitMessages,
	safetyMessages,
)
