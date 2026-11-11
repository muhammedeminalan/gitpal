package i18n

import "strings"

// Texts of the desktop app (spec §9). Keys starting with "gui." are sent to
// the frontend in one go (GUITexts); the rest are errors the Go side returns.
const (
	MsgGUINoRepo    Key = "gui.err.no_repo"
	MsgGUIUnknownOp Key = "gui.err.unknown_op"

	MsgGUIShort        Key = "cli.gui.short"
	MsgGUINotInstalled Key = "cli.gui.not_installed"
)

var guiMessages = map[Key]message{
	MsgGUINoRepo:    {tr: "Önce bir repo aç.", en: "Open a repository first."},
	MsgGUIUnknownOp: {tr: "Bilinmeyen işlem: %s", en: "Unknown operation: %s"},

	MsgGUIShort: {tr: "masaüstü arayüzünü bu repoda aç", en: "open the desktop app on this repository"},
	MsgGUINotInstalled: {
		tr: "gitpal-gui bulunamadı. Masaüstü arayüzü ayrı bir programdır: gitpal ile aynı klasöre ya da PATH'e kur.",
		en: "gitpal-gui was not found. The desktop app is a separate program: install it next to gitpal or on your PATH.",
	},

	"gui.app.title":           {tr: "gitpal", en: "gitpal"},
	"gui.welcome.title":       {tr: "Bir repo seç", en: "Choose a repository"},
	"gui.welcome.recent":      {tr: "Son açılanlar", en: "Recent"},
	"gui.welcome.path":        {tr: "Klasör yolu", en: "Folder path"},
	"gui.welcome.open":        {tr: "Aç", en: "Open"},
	"gui.welcome.init":        {tr: "Yeni repo oluştur", en: "Create a repository"},
	"gui.welcome.clone":       {tr: "Klonla", en: "Clone"},
	"gui.welcome.url":         {tr: "Repo adresi", en: "Repository URL"},
	"gui.welcome.browse":      {tr: "Klasör seç…", en: "Choose folder…"},
	"gui.conflict.ours":       {tr: "Bizimkini al", en: "Take ours"},
	"gui.conflict.theirs":     {tr: "Onlarınkini al", en: "Take theirs"},
	"gui.conflict.resolved":   {tr: "Çözüldü olarak işaretle", en: "Mark resolved"},
	"gui.conflict.title":      {tr: "Çakışan dosyalar", en: "Conflicted files"},
	"gui.toolbar.continue":    {tr: "Devam et", en: "Continue"},
	"gui.toolbar.skip":        {tr: "Bu adımı atla", en: "Skip this step"},
	"gui.welcome.none":        {tr: "Henüz açılmış repo yok.", en: "No repositories opened yet."},
	"gui.nav.changes":         {tr: "Değişiklikler", en: "Changes"},
	"gui.nav.history":         {tr: "Geçmiş", en: "History"},
	"gui.nav.operations":      {tr: "İşlemler ve Undo", en: "Operations and undo"},
	"gui.nav.settings":        {tr: "Ayarlar", en: "Settings"},
	"gui.nav.switch_repo":     {tr: "Repo değiştir", en: "Switch repository"},
	"gui.side.branches":       {tr: "Branch'ler", en: "Branches"},
	"gui.side.remotes":        {tr: "Remote'lar", en: "Remotes"},
	"gui.side.stashes":        {tr: "Stash'ler", en: "Stashes"},
	"gui.side.tags":           {tr: "Tag'ler", en: "Tags"},
	"gui.side.new_branch":     {tr: "Yeni branch", en: "New branch"},
	"gui.side.switch":         {tr: "Geç", en: "Switch"},
	"gui.side.delete":         {tr: "Sil", en: "Delete"},
	"gui.side.merge":          {tr: "Bu branch'e birleştir", en: "Merge into current"},
	"gui.files.staged":        {tr: "Commit'e girecek", en: "Staged"},
	"gui.files.unstaged":      {tr: "Değişiklikler", en: "Changes"},
	"gui.files.stage":         {tr: "Ekle", en: "Stage"},
	"gui.files.unstage":       {tr: "Çıkar", en: "Unstage"},
	"gui.files.stage_all":     {tr: "Hepsini ekle", en: "Stage all"},
	"gui.files.discard":       {tr: "Değişikliği at", en: "Discard"},
	"gui.files.empty":         {tr: "Değişiklik yok.", en: "No changes."},
	"gui.diff.select":         {tr: "Diff'ini görmek için bir dosya seç.", en: "Select a file to see its diff."},
	"gui.diff.binary":         {tr: "Binary dosya", en: "Binary file"},
	"gui.diff.stage_hunk":     {tr: "Bu parçayı ekle", en: "Stage hunk"},
	"gui.diff.unstage_hunk":   {tr: "Bu parçayı çıkar", en: "Unstage hunk"},
	"gui.commit.placeholder":  {tr: "Commit mesajı", en: "Commit message"},
	"gui.commit.ai":           {tr: "AI ile üret", en: "Generate with AI"},
	"gui.commit.commit":       {tr: "Commit", en: "Commit"},
	"gui.commit.push":         {tr: "Commit & Push", en: "Commit & Push"},
	"gui.commit.amend":        {tr: "Son commit'i düzelt", en: "Amend last commit"},
	"gui.toolbar.fetch":       {tr: "Fetch", en: "Fetch"},
	"gui.toolbar.pull":        {tr: "Pull", en: "Pull"},
	"gui.toolbar.push":        {tr: "Push", en: "Push"},
	"gui.toolbar.abort":       {tr: "İşlemi iptal et", en: "Abort operation"},
	"gui.toolbar.refresh":     {tr: "Yenile", en: "Refresh"},
	"gui.toolbar.palette":     {tr: "Komut paleti (Ctrl/Cmd+K)", en: "Command palette (Ctrl/Cmd+K)"},
	"gui.status.protected":    {tr: "korumalı", en: "protected"},
	"gui.status.ahead":        {tr: "ileride", en: "ahead"},
	"gui.status.behind":       {tr: "geride", en: "behind"},
	"gui.status.no_upstream":  {tr: "upstream yok", en: "no upstream"},
	"gui.status.detached":     {tr: "detached HEAD", en: "detached HEAD"},
	"gui.status.unborn":       {tr: "henüz commit yok", en: "no commits yet"},
	"gui.history.search":      {tr: "Mesajda ara", en: "Search messages"},
	"gui.history.all":         {tr: "Tüm branch'ler", en: "All branches"},
	"gui.history.files":       {tr: "Değişen dosyalar", en: "Changed files"},
	"gui.history.revert":      {tr: "Geri al (revert)", en: "Revert"},
	"gui.history.cherry":      {tr: "Cherry-pick", en: "Cherry-pick"},
	"gui.history.reset":       {tr: "Branch'i buraya taşı (reset)", en: "Reset branch here"},
	"gui.ops.undo":            {tr: "Son işlemi geri al", en: "Undo last operation"},
	"gui.ops.undo_none":       {tr: "Geri alınacak işlem yok.", en: "Nothing to undo."},
	"gui.ops.history":         {tr: "İşlem geçmişi", en: "Operation history"},
	"gui.ops.backups":         {tr: "Yedekler", en: "Backups"},
	"gui.ops.commands":        {tr: "Çalışan git komutları", en: "Git commands that ran"},
	"gui.ops.failed":          {tr: "başarısız", en: "failed"},
	"gui.safety.title":        {tr: "Güvenlik kontrolü", en: "Safety check"},
	"gui.safety.risk":         {tr: "Risk", en: "Risk"},
	"gui.safety.effects":      {tr: "Ne olacak", en: "What will happen"},
	"gui.safety.findings":     {tr: "Dikkat", en: "Look out"},
	"gui.safety.backups":      {tr: "Yedek", en: "Backup"},
	"gui.safety.type":         {tr: "Onaylamak için şunu yaz:", en: "Type this to confirm:"},
	"gui.safety.blocked":      {tr: "Bu işlem engellendi.", en: "This operation is blocked."},
	"gui.safety.confirm":      {tr: "Onayla", en: "Confirm"},
	"gui.safety.cancel":       {tr: "Vazgeç", en: "Cancel"},
	"gui.safety.commands":     {tr: "Komutu göster", en: "Show commands"},
	"gui.settings.identity":   {tr: "Git kimliği", en: "Git identity"},
	"gui.settings.name":       {tr: "Ad", en: "Name"},
	"gui.settings.email":      {tr: "E-posta", en: "Email"},
	"gui.settings.global":     {tr: "Tüm repolar için", en: "For every repository"},
	"gui.settings.save":       {tr: "Kaydet", en: "Save"},
	"gui.settings.ai_key":     {tr: "AI anahtarı (keychain'e kaydedilir)", en: "AI key (kept in the keychain)"},
	"gui.settings.key_set":    {tr: "Anahtar kayıtlı", en: "Key stored"},
	"gui.settings.all":        {tr: "Tüm ayarlar", en: "All settings"},
	"gui.settings.saved":      {tr: "Kaydedildi.", en: "Saved."},
	"gui.ai.consent_title":    {tr: "Yapay zekâ onayı", en: "AI consent"},
	"gui.ai.agree":            {tr: "Kabul ediyorum", en: "I agree"},
	"gui.ai.split":            {tr: "Bölme önerisi", en: "Split advice"},
	"gui.ai.withheld":         {tr: "Gönderilmeyen dosyalar", en: "Files not sent"},
	"gui.palette.placeholder": {tr: "Komut ara…", en: "Search commands…"},
	"gui.theme.toggle":        {tr: "Tema", en: "Theme"},
}

// GUITexts returns every "gui." text in lang, for the frontend.
func GUITexts(lang Lang) map[string]string {
	out := map[string]string{}
	for key, msg := range catalog {
		if strings.HasPrefix(string(key), "gui.") {
			out[string(key)] = msg.text(lang)
		}
	}
	return out
}
