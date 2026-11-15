package i18n

// Komut rehberi (gitpal guide / masaüstü "Yardım"): her örneğin açıklaması.
// Örnek komutların kendisi internal/core/guide içinde, dilden bağımsız.
var guideMessages = map[Key]message{
	"guide.group.start":    {tr: "Başlarken", en: "Getting started"},
	"guide.doctor":         {tr: "Bilgisayarını kontrol eder: git kurulu mu, kimliğin ayarlı mı", en: "Checks your setup: is git installed, is your identity set"},
	"guide.identity":       {tr: "Commit'lerde görünecek adını ve e-postanı ayarlar", en: "Sets the name and e-mail your commits are signed with"},
	"guide.init":           {tr: "Bu klasörü yeni bir git reposu yapar", en: "Turns this folder into a new git repository"},
	"guide.clone":          {tr: "Bir repoyu internetten bilgisayarına kopyalar", en: "Copies a repository from the internet to your computer"},
	"guide.dashboard":      {tr: "Tuşlarla çalışılan ana ekranı açar (q ile çıkılır)", en: "Opens the keyboard-driven main screen (q quits)"},
	"guide.group.daily":    {tr: "Günlük iş", en: "Everyday work"},
	"guide.status":         {tr: "Hangi dosyaların değiştiğini gösterir", en: "Shows which files changed"},
	"guide.diff":           {tr: "Değişiklikleri satır satır gösterir", en: "Shows the changes line by line"},
	"guide.add":            {tr: "Commit'e girecek dosyaları listeden seçtirir", en: "Lets you pick the files for the next commit from a list"},
	"guide.add_file":       {tr: "Belirli bir dosyayı commit'e ekler", en: "Adds one file to the next commit"},
	"guide.commit":         {tr: "Eklenen dosyaları bir mesajla kaydeder", en: "Records the added files with a message"},
	"guide.commit_ai":      {tr: "Commit mesajını yapay zekâ önerir, sen seçersin", en: "AI suggests the commit message, you choose"},
	"guide.save":           {tr: "Dosya seç → commit → push, hepsi tek adımda", en: "Pick files → commit → push, all in one go"},
	"guide.push":           {tr: "Commit'lerini uzak repoya (ör. GitHub) gönderir", en: "Sends your commits to the remote (e.g. GitHub)"},
	"guide.pull":           {tr: "Uzak repodaki yenilikleri bilgisayarına alır", en: "Brings the remote's new commits to your computer"},
	"guide.log":            {tr: "Commit geçmişini dallarıyla birlikte gösterir", en: "Shows the commit history with its branches"},
	"guide.group.branch":   {tr: "Branch'ler", en: "Branches"},
	"guide.branch_list":    {tr: "Tüm branch'leri listeler", en: "Lists all branches"},
	"guide.branch_new":     {tr: "Yeni bir branch açar ve ona geçer", en: "Creates a new branch and switches to it"},
	"guide.branch_switch":  {tr: "Başka bir branch'e geçer", en: "Switches to another branch"},
	"guide.merge":          {tr: "Başka bir branch'i bulunduğun branch'e birleştirir", en: "Merges another branch into the current one"},
	"guide.branch_delete":  {tr: "Branch'i siler (önce yedek alınır)", en: "Deletes a branch (a backup is taken first)"},
	"guide.group.undo":     {tr: "Geri alma ve kurtarma", en: "Undo and recovery"},
	"guide.undo":           {tr: "gitpal'ın yaptığı son işlemi geri alır", en: "Undoes the last thing gitpal did"},
	"guide.discard":        {tr: "Dosyadaki kaydedilmemiş değişiklikleri atar (yedekli)", en: "Throws away a file's unsaved changes (backed up)"},
	"guide.unstage":        {tr: "Dosyayı commit listesinden çıkarır, değişiklik durur", en: "Takes a file out of the next commit; the change stays"},
	"guide.stash_save":     {tr: "Değişiklikleri geçici olarak kenara koyar", en: "Puts your changes aside for a while"},
	"guide.stash_pop":      {tr: "Kenara koyduklarını geri getirir", en: "Brings back what you put aside"},
	"guide.backups":        {tr: "gitpal'ın aldığı yedekleri listeler", en: "Lists the backups gitpal took"},
	"guide.group.conflict": {tr: "Çakışma olunca", en: "When there is a conflict"},
	"guide.conflicts":      {tr: "Çakışan dosyaları listeler", en: "Lists the conflicted files"},
	"guide.resolve":        {tr: "Çakışan dosyada karşı tarafın halini alır", en: "Takes the other side's version of a conflicted file"},
	"guide.continue":       {tr: "Çakışmalar çözülünce işleme devam eder", en: "Carries on once the conflicts are resolved"},
	"guide.abort":          {tr: "Yarım kalan birleştirmeden vazgeçer, öncesine döner", en: "Gives up the unfinished merge and goes back"},
	"guide.group.github":   {tr: "GitHub işlemleri", en: "GitHub"},
	"guide.auth_github":    {tr: "GitHub hesabına bağlanır", en: "Connects your GitHub account"},
	"guide.pr_create":      {tr: "Bu branch'ten pull request açar", en: "Opens a pull request from this branch"},
	"guide.pr_list":        {tr: "Açık pull request'leri listeler", en: "Lists the open pull requests"},
	"guide.issue_list":     {tr: "Açık issue'ları listeler", en: "Lists the open issues"},
	"guide.group.help":     {tr: "Yardım", en: "Help"},
	"guide.guide":          {tr: "Bu listeyi gösterir (kısa adı: gitpal rehber)", en: "Shows this list"},
	"guide.help":           {tr: "Bir komutun bütün seçeneklerini gösterir", en: "Shows every option of a command"},
	"guide.dry_run":        {tr: "Hiçbir şeyi değiştirmeden ne olacağını gösterir", en: "Shows what would happen without changing anything"},
	"guide.explain":        {tr: "Arka planda çalışan git komutlarını da gösterir", en: "Also shows the git commands run behind the scenes"},
	MsgGuideShort:          {tr: "komutları örnekleriyle listele (yeni başlayanlar için)", en: "list the commands with examples (for beginners)"},
	MsgGuideIntro: {
		tr: "Her satırda bir örnek ve ne yaptığı. Bir komutun bütün seçenekleri için: gitpal <komut> --help",
		en: "One example per line and what it does. For every option of a command: gitpal <command> --help",
	},
}

// Rehberin açıklama satırı ve komutun kısa tanımı.
const (
	MsgGuideIntro Key = "guide.intro"
	MsgGuideShort Key = "cli.guide.short"
)
