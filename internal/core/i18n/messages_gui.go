package i18n

import "strings"

// Texts of the desktop app (spec §9). Keys starting with "gui." are sent to
// the frontend in one go (GUITexts); the rest are errors the Go side returns.
const (
	MsgGUINoRepo    Key = "gui.err.no_repo"
	MsgGUIUnknownOp Key = "gui.err.unknown_op"

	MsgTerminalHint     Key = "gui.err.terminal_hint"
	MsgTerminalNoGitpal Key = "gui.err.terminal_no_gitpal"
	MsgTerminalNotFound Key = "gui.err.terminal_not_found"

	MsgUpdateCheckFailed    Key = "gui.err.update_check_failed"
	MsgUpdateNoPackage      Key = "gui.err.update_no_package"
	MsgUpdateNotVerified    Key = "gui.err.update_not_verified"
	MsgUpdateDownloadFailed Key = "gui.err.update_download_failed"
	MsgUpdateHintMac        Key = "gui.err.update_hint_mac"
	MsgUpdateHintWindows    Key = "gui.err.update_hint_windows"
	MsgUpdateHintLinux      Key = "gui.err.update_hint_linux"
	MsgCommandsInstalled    Key = "gui.err.commands_installed"

	MsgGUIShort        Key = "cli.gui.short"
	MsgGUINotInstalled Key = "cli.gui.not_installed"
)

var guiMessages = map[Key]message{
	MsgGUINoRepo:    {tr: "Önce bir repo aç.", en: "Open a repository first."},
	MsgGUIUnknownOp: {tr: "Bilinmeyen işlem: %s", en: "Unknown operation: %s"},

	MsgTerminalHint: {
		tr: "gitpal ana ekranından çıktın. Komut listesi için: gitpal rehber · Ana ekrana dönmek için: gitpal",
		en: "You left gitpal's main screen. For the list of commands: gitpal guide · To go back: gitpal",
	},
	MsgTerminalNoGitpal: {
		tr: "gitpal komut satırı aracı bulunamadı. Uygulamayı sürüm sayfasındaki paketle yeniden kur ya da gitpal'ı PATH'e ekle.",
		en: "The gitpal command-line tool was not found. Reinstall the app from the release page or put gitpal on your PATH.",
	},
	MsgTerminalNotFound: {
		tr: "Açılacak bir terminal programı bulunamadı. Linux'ta TERMINAL değişkeniyle hangisinin kullanılacağını belirtebilirsin.",
		en: "No terminal program was found to open. On Linux you can name one with the TERMINAL variable.",
	},
	MsgUpdateCheckFailed: {
		tr: "Güncelleme denetlenemedi. İnternet bağlantını kontrol edip sonra yeniden dene.",
		en: "Could not check for updates. Check your internet connection and try again later.",
	},
	MsgUpdateNoPackage: {
		tr: "Bu sistem için hazır masaüstü paketi yok. Sürüm sayfasından indirebilirsin: %s",
		en: "There is no desktop package for this system. You can download it from the release page: %s",
	},
	MsgUpdateNotVerified: {
		tr: "İndirilen dosya yayımlanan sağlama toplamıyla doğrulanamadı, silindi. Sürüm sayfasından elle indirebilirsin: %s",
		en: "The download could not be verified against its published checksum and was deleted. You can download it from the release page: %s",
	},
	MsgUpdateDownloadFailed: {
		tr: "Güncelleme indirilemedi. Sürüm sayfasından elle indirebilirsin: %s",
		en: "The update could not be downloaded. You can download it from the release page: %s",
	},
	MsgUpdateHintMac: {
		tr: "İndirildi: %s. Açılan pencerede gitpal'ı Uygulamalar klasörüne sürükle, eskisinin yerine koy; sonra gitpal'ı kapatıp yeniden aç.",
		en: "Downloaded: %s. In the window that opened, drag gitpal to Applications and replace the old one; then quit and reopen gitpal.",
	},
	MsgUpdateHintWindows: {
		tr: "İndirildi: %s. Zip'i aç, içindekileri gitpal'ın bulunduğu klasöre eskilerin üzerine kopyala; sonra gitpal'ı yeniden aç.",
		en: "Downloaded: %s. Open the zip and copy its contents over the old ones in gitpal's folder; then reopen gitpal.",
	},
	MsgUpdateHintLinux: {
		tr: "İndirildi: %s. Arşivi aç, gitpal-gui ve gitpal'ı eskilerinin yerine koy; sonra gitpal'ı yeniden aç.",
		en: "Downloaded: %s. Unpack it, put gitpal-gui and gitpal in place of the old ones; then reopen gitpal.",
	},
	MsgCommandsInstalled: {
		tr: "Terminal komutları kuruldu: gitpal ve gp (%s). Bundan sonra açacağın yeni terminal pencerelerinde çalışır.",
		en: "Terminal commands installed: gitpal and gp (%s). They work in terminal windows you open from now on.",
	},
	MsgGUIShort: {tr: "masaüstü arayüzünü bu repoda aç", en: "open the desktop app on this repository"},
	MsgGUINotInstalled: {
		tr: "gitpal-gui bulunamadı. Masaüstü arayüzü ayrı bir programdır: gitpal ile aynı klasöre ya da PATH'e kur.",
		en: "gitpal-gui was not found. The desktop app is a separate program: install it next to gitpal or on your PATH.",
	},

	"gui.update.available":     {tr: "Yeni sürüm hazır:", en: "A new version is out:"},
	"gui.update.current":       {tr: "sendeki", en: "you have"},
	"gui.update.download":      {tr: "Güncelle", en: "Update"},
	"gui.update.downloading":   {tr: "İndiriliyor…", en: "Downloading…"},
	"gui.update.notes":         {tr: "Sürüm notları", en: "Release notes"},
	"gui.update.later":         {tr: "Sonra", en: "Later"},
	"gui.update.check":         {tr: "Güncellemeleri denetle", en: "Check for updates"},
	"gui.update.uptodate":      {tr: "gitpal güncel.", en: "gitpal is up to date."},
	"gui.update.dev":           {tr: "Geliştirme sürümü kullanıyorsun; son yayımlanan sürüm:", en: "You are running a development build; the latest release is:"},
	"gui.update.version":       {tr: "Sürüm", en: "Version"},
	"gui.toolbar.terminal":     {tr: "Terminal", en: "Terminal"},
	"gui.toolbar.terminal_tip": {tr: "Bu repoda terminal aç; gitpal ana ekranı kendiliğinden başlar", en: "Open a terminal in this repository; gitpal's main screen starts by itself"},
	"gui.nav.help":             {tr: "Yardım", en: "Help"},
	"gui.help.title":           {tr: "Komut rehberi", en: "Command guide"},
	"gui.help.intro":           {tr: "Terminalde kullanabileceğin gitpal komutları ve ne yaptıkları. Denemek için Terminal düğmesine bas, örneği kopyalayıp yapıştır.", en: "The gitpal commands you can use in a terminal and what they do. To try one, press Terminal, then copy and paste the example."},
	"gui.help.search":          {tr: "Ara: commit, branch, geri al…", en: "Search: commit, branch, undo…"},
	"gui.help.example":         {tr: "Örnek komut", en: "Example"},
	"gui.help.what":            {tr: "Ne yapar", en: "What it does"},
	"gui.help.copy":            {tr: "Kopyala", en: "Copy"},
	"gui.help.copied":          {tr: "Kopyalandı", en: "Copied"},
	"gui.help.none":            {tr: "Eşleşen komut yok.", en: "No matching command."},
	"gui.branch.menu":          {tr: "Branch değiştir", en: "Switch branch"},
	"gui.branch.search":        {tr: "Branch ara…", en: "Find a branch…"},
	"gui.branch.local":         {tr: "Bu bilgisayardaki branch'ler", en: "Local branches"},
	"gui.branch.remote":        {tr: "Uzak branch'ler", en: "Remote branches"},
	"gui.branch.current":       {tr: "şu an burada", en: "you are here"},
	"gui.branch.switch_tip":    {tr: "Bu branch'e geçmek için tıkla", en: "Click to switch to this branch"},
	"gui.branch.none":          {tr: "Eşleşen branch yok.", en: "No matching branch."},
	"gui.branch.new":           {tr: "Yeni branch", en: "New branch"},
	"gui.branch.new_title":     {tr: "Yeni branch aç", en: "Create a new branch"},
	"gui.branch.new_hint":      {tr: "Branch açılır ve hemen ona geçilir. Kaydedilmemiş değişikliklerin seninle gelir.", en: "The branch is created and you switch to it. Your unsaved changes come along."},
	"gui.branch.name":          {tr: "Branch adı", en: "Branch name"},
	"gui.branch.placeholder":   {tr: "örn. feature/giris", en: "e.g. feature/login"},
	"gui.branch.create":        {tr: "Oluştur ve geç", en: "Create and switch"},
	"gui.branch.created":       {tr: "Branch oluşturuldu: ", en: "Branch created: "},
	"gui.palette.switch_to":    {tr: "Branch'e geç: ", en: "Switch to: "},
	"gui.app.title":            {tr: "gitpal", en: "gitpal"},
	"gui.welcome.title":        {tr: "Bir repo seç", en: "Choose a repository"},
	"gui.welcome.recent":       {tr: "Son açılanlar", en: "Recent"},
	"gui.welcome.path":         {tr: "Klasör yolu", en: "Folder path"},
	"gui.welcome.open":         {tr: "Aç", en: "Open"},
	"gui.welcome.init":         {tr: "Yeni repo oluştur", en: "Create a repository"},
	"gui.welcome.clone":        {tr: "Klonla", en: "Clone"},
	"gui.welcome.url":          {tr: "Repo adresi", en: "Repository URL"},
	"gui.welcome.browse":       {tr: "Klasör seç…", en: "Choose folder…"},
	"gui.conflict.ours":        {tr: "Bizimkini al", en: "Take ours"},
	"gui.conflict.theirs":      {tr: "Onlarınkini al", en: "Take theirs"},
	"gui.conflict.resolved":    {tr: "Çözüldü olarak işaretle", en: "Mark resolved"},
	"gui.conflict.title":       {tr: "Çakışan dosyalar", en: "Conflicted files"},
	"gui.toolbar.continue":     {tr: "Devam et", en: "Continue"},
	"gui.toolbar.skip":         {tr: "Bu adımı atla", en: "Skip this step"},
	"gui.welcome.none":         {tr: "Henüz açılmış repo yok.", en: "No repositories opened yet."},
	"gui.nav.changes":          {tr: "Değişiklikler", en: "Changes"},
	"gui.nav.history":          {tr: "Geçmiş", en: "History"},
	"gui.nav.operations":       {tr: "İşlemler ve Undo", en: "Operations and undo"},
	"gui.nav.settings":         {tr: "Ayarlar", en: "Settings"},
	"gui.nav.switch_repo":      {tr: "Repo değiştir", en: "Switch repository"},
	"gui.side.branches":        {tr: "Branch'ler", en: "Branches"},
	"gui.side.remotes":         {tr: "Remote'lar", en: "Remotes"},
	"gui.side.stashes":         {tr: "Stash'ler", en: "Stashes"},
	"gui.side.tags":            {tr: "Tag'ler", en: "Tags"},
	"gui.side.new_branch":      {tr: "Yeni branch", en: "New branch"},
	"gui.side.switch":          {tr: "Geç", en: "Switch"},
	"gui.side.delete":          {tr: "Sil", en: "Delete"},
	"gui.side.merge":           {tr: "Bu branch'e birleştir", en: "Merge into current"},
	"gui.files.staged":         {tr: "Commit'e girecek", en: "Staged"},
	"gui.files.unstaged":       {tr: "Değişiklikler", en: "Changes"},
	"gui.files.stage":          {tr: "Ekle", en: "Stage"},
	"gui.files.unstage":        {tr: "Çıkar", en: "Unstage"},
	"gui.files.stage_all":      {tr: "Hepsini ekle", en: "Stage all"},
	"gui.files.discard":        {tr: "Değişikliği at", en: "Discard"},
	"gui.files.empty":          {tr: "Değişiklik yok.", en: "No changes."},
	"gui.diff.select":          {tr: "Diff'ini görmek için bir dosya seç.", en: "Select a file to see its diff."},
	"gui.diff.binary":          {tr: "Binary dosya", en: "Binary file"},
	"gui.diff.stage_hunk":      {tr: "Bu parçayı ekle", en: "Stage hunk"},
	"gui.diff.unstage_hunk":    {tr: "Bu parçayı çıkar", en: "Unstage hunk"},
	"gui.commit.placeholder":   {tr: "Commit mesajı", en: "Commit message"},
	"gui.commit.ai":            {tr: "AI ile üret", en: "Generate with AI"},
	"gui.commit.commit":        {tr: "Commit", en: "Commit"},
	"gui.commit.push":          {tr: "Commit & Push", en: "Commit & Push"},
	"gui.commit.amend":         {tr: "Son commit'i düzelt", en: "Amend last commit"},
	"gui.toolbar.fetch":        {tr: "Fetch", en: "Fetch"},
	"gui.toolbar.pull":         {tr: "Pull", en: "Pull"},
	"gui.toolbar.push":         {tr: "Push", en: "Push"},
	"gui.toolbar.abort":        {tr: "İşlemi iptal et", en: "Abort operation"},
	"gui.toolbar.refresh":      {tr: "Yenile", en: "Refresh"},
	"gui.toolbar.palette":      {tr: "Komut paleti (Ctrl/Cmd+K)", en: "Command palette (Ctrl/Cmd+K)"},
	"gui.status.protected":     {tr: "korumalı", en: "protected"},
	"gui.status.ahead":         {tr: "ileride", en: "ahead"},
	"gui.status.behind":        {tr: "geride", en: "behind"},
	"gui.status.no_upstream":   {tr: "upstream yok", en: "no upstream"},
	"gui.status.detached":      {tr: "detached HEAD", en: "detached HEAD"},
	"gui.status.unborn":        {tr: "henüz commit yok", en: "no commits yet"},
	"gui.history.search":       {tr: "Mesajda ara", en: "Search messages"},
	"gui.history.all":          {tr: "Tüm branch'ler", en: "All branches"},
	"gui.history.files":        {tr: "Değişen dosyalar", en: "Changed files"},
	"gui.history.revert":       {tr: "Geri al (revert)", en: "Revert"},
	"gui.history.cherry":       {tr: "Cherry-pick", en: "Cherry-pick"},
	"gui.history.reset":        {tr: "Branch'i buraya taşı (reset)", en: "Reset branch here"},
	"gui.ops.undo":             {tr: "Son işlemi geri al", en: "Undo last operation"},
	"gui.ops.undo_none":        {tr: "Geri alınacak işlem yok.", en: "Nothing to undo."},
	"gui.ops.history":          {tr: "İşlem geçmişi", en: "Operation history"},
	"gui.ops.backups":          {tr: "Yedekler", en: "Backups"},
	"gui.ops.commands":         {tr: "Çalışan git komutları", en: "Git commands that ran"},
	"gui.ops.failed":           {tr: "başarısız", en: "failed"},
	"gui.safety.title":         {tr: "Güvenlik kontrolü", en: "Safety check"},
	"gui.safety.risk":          {tr: "Risk", en: "Risk"},
	"gui.safety.effects":       {tr: "Ne olacak", en: "What will happen"},
	"gui.safety.findings":      {tr: "Dikkat", en: "Look out"},
	"gui.safety.backups":       {tr: "Yedek", en: "Backup"},
	"gui.safety.type":          {tr: "Onaylamak için şunu yaz:", en: "Type this to confirm:"},
	"gui.safety.blocked":       {tr: "Bu işlem engellendi.", en: "This operation is blocked."},
	"gui.safety.confirm":       {tr: "Onayla", en: "Confirm"},
	"gui.safety.cancel":        {tr: "Vazgeç", en: "Cancel"},
	"gui.safety.commands":      {tr: "Komutu göster", en: "Show commands"},
	"gui.settings.identity":    {tr: "Git kimliği", en: "Git identity"},
	"gui.settings.name":        {tr: "Ad", en: "Name"},
	"gui.settings.email":       {tr: "E-posta", en: "Email"},
	"gui.settings.global":      {tr: "Tüm repolar için", en: "For every repository"},
	"gui.settings.save":        {tr: "Kaydet", en: "Save"},
	"gui.settings.ai_key":      {tr: "AI anahtarı (keychain'e kaydedilir)", en: "AI key (kept in the keychain)"},
	"gui.settings.key_set":     {tr: "Anahtar kayıtlı", en: "Key stored"},
	"gui.settings.all":         {tr: "Tüm ayarlar", en: "All settings"},
	"gui.settings.saved":       {tr: "Kaydedildi.", en: "Saved."},
	"gui.ai.consent_title":     {tr: "Yapay zekâ onayı", en: "AI consent"},
	"gui.ai.agree":             {tr: "Kabul ediyorum", en: "I agree"},
	"gui.ai.split":             {tr: "Bölme önerisi", en: "Split advice"},
	"gui.ai.withheld":          {tr: "Gönderilmeyen dosyalar", en: "Files not sent"},
	"gui.palette.placeholder":  {tr: "Komut ara…", en: "Search commands…"},
	"gui.theme.toggle":         {tr: "Tema", en: "Theme"},
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
