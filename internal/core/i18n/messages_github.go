package i18n

// Texts of the GitHub features (spec §5). These are message keys, not
// credentials.
//
//nolint:gosec // G101: key names mention "token"
const (
	MsgGHNoToken        Key = "github.no_token"
	MsgGHAuth           Key = "github.auth"
	MsgGHRateLimit      Key = "github.rate_limit"
	MsgGHRateLimitReset Key = "github.rate_limit_reset"
	MsgGHNotFound       Key = "github.not_found"
	MsgGHInvalid        Key = "github.invalid"
	MsgGHNetwork        Key = "github.network"
	MsgGHServer         Key = "github.server"
	MsgGHNotGitHub      Key = "github.not_github"
	MsgGHNotMergeable   Key = "github.not_mergeable"

	MsgAuthGitHubShort    Key = "cli.auth.github.short"
	MsgAuthGitHubToken    Key = "cli.auth.github.token"
	MsgAuthGitHubGH       Key = "cli.auth.github.gh"
	MsgAuthGitHubNoClient Key = "cli.auth.github.no_client"
	MsgAuthGitHubCode     Key = "cli.auth.github.code"
	MsgAuthGitHubWaiting  Key = "cli.auth.github.waiting"
	MsgAuthGitHubDone     Key = "cli.auth.github.done"
	MsgAuthGitHubNoGH     Key = "cli.auth.github.no_gh"

	MsgRepoShort         Key = "cli.repo.short"
	MsgRepoCreateShort   Key = "cli.repo.create.short"
	MsgRepoPrivate       Key = "cli.repo.private"
	MsgRepoDescription   Key = "cli.repo.description"
	MsgRepoREADME        Key = "cli.repo.readme"
	MsgRepoGitignore     Key = "cli.repo.gitignore"
	MsgRepoLicense       Key = "cli.repo.license"
	MsgRepoOrg           Key = "cli.repo.org"
	MsgRepoPush          Key = "cli.repo.push"
	MsgRepoCreated       Key = "cli.repo.created"
	MsgRepoConnected     Key = "cli.repo.connected"
	MsgRepoHasOrigin     Key = "cli.repo.has_origin"
	MsgRepoListShort     Key = "cli.repo.list.short"
	MsgRepoCloneShort    Key = "cli.repo.clone.short"
	MsgRepoBadName       Key = "cli.repo.bad_name"
	MsgRepoPrivateTag    Key = "cli.repo.private_tag"
	MsgRepoNotLinked     Key = "cli.repo.not_linked"
	MsgRepoProtectedWarn Key = "cli.repo.protected_warn"

	MsgPRShort         Key = "cli.pr.short"
	MsgPRCreateShort   Key = "cli.pr.create.short"
	MsgPRTitle         Key = "cli.pr.title"
	MsgPRBody          Key = "cli.pr.body"
	MsgPRBase          Key = "cli.pr.base"
	MsgPRDraft         Key = "cli.pr.draft"
	MsgPRAI            Key = "cli.pr.ai"
	MsgPRCreated       Key = "cli.pr.created"
	MsgPRListShort     Key = "cli.pr.list.short"
	MsgPRState         Key = "cli.pr.state"
	MsgPRNone          Key = "cli.pr.none"
	MsgPRViewShort     Key = "cli.pr.view.short"
	MsgPRMergeShort    Key = "cli.pr.merge.short"
	MsgPRMerged        Key = "cli.pr.merged"
	MsgPRCloseShort    Key = "cli.pr.close.short"
	MsgPRClosed        Key = "cli.pr.closed"
	MsgPRCheckoutShort Key = "cli.pr.checkout.short"
	MsgPRCommentShort  Key = "cli.pr.comment.short"
	MsgPRCommented     Key = "cli.pr.commented"
	MsgPRTitlePrompt   Key = "cli.pr.title_prompt"
	MsgPRMergeConfirm  Key = "cli.pr.merge_confirm"
	MsgPRBadNumber     Key = "cli.pr.bad_number"

	MsgIssueShort       Key = "cli.issue.short"
	MsgIssueListShort   Key = "cli.issue.list.short"
	MsgIssueCreateShort Key = "cli.issue.create.short"
	MsgIssueCloseShort  Key = "cli.issue.close.short"
	MsgIssueLabelShort  Key = "cli.issue.label.short"
	MsgIssueLabels      Key = "cli.issue.labels"
	MsgIssueCreated     Key = "cli.issue.created"
	MsgIssueClosed      Key = "cli.issue.closed"
	MsgIssueLabeled     Key = "cli.issue.labeled"
	MsgIssueNone        Key = "cli.issue.none"
	MsgIssueFixesHint   Key = "cli.issue.fixes_hint"

	MsgReleaseShort       Key = "cli.release.short"
	MsgReleaseCreateShort Key = "cli.release.create.short"
	MsgReleaseNotes       Key = "cli.release.notes"
	MsgReleaseGenerate    Key = "cli.release.generate"
	MsgReleasePrerelease  Key = "cli.release.prerelease"
	MsgReleaseCreated     Key = "cli.release.created"
	MsgActionsShort       Key = "cli.actions.short"
	MsgActionsNone        Key = "cli.actions.none"
)

var githubMessages = map[Key]message{
	MsgGHNoToken: {
		tr: "GitHub'a giriş yapılmamış. `gitpal auth github` ile giriş yap.",
		en: "You are not signed in to GitHub. Sign in with `gitpal auth github`.",
	},
	MsgGHAuth: {
		tr: "GitHub token'ı geçersiz ya da bu işlem için yetkisi yok. `gitpal auth github` ile yeniden giriş yap.",
		en: "The GitHub token is invalid or not allowed to do this. Sign in again with `gitpal auth github`.",
	},
	MsgGHRateLimit: {
		tr: "GitHub istek sınırına ulaşıldı.",
		en: "The GitHub rate limit was reached.",
	},
	MsgGHRateLimitReset: {tr: "%s'de yeniden dene.", en: "Try again at %s."},
	MsgGHNotFound: {
		tr: "GitHub'da bulunamadı (ya da token'ın görme yetkisi yok).",
		en: "Not found on GitHub (or the token cannot see it).",
	},
	MsgGHInvalid: {tr: "GitHub isteği reddetti:", en: "GitHub rejected the request:"},
	MsgGHNetwork: {
		tr: "GitHub'a ulaşılamadı. İnternet bağlantını kontrol et.",
		en: "Could not reach GitHub. Check your internet connection.",
	},
	MsgGHServer: {tr: "GitHub şu an cevap veremiyor; biraz sonra yeniden dene.", en: "GitHub cannot answer right now; try again shortly."},
	MsgGHNotGitHub: {
		tr: "Bu reponun remote'u GitHub'da değil (ya da remote yok). `--repo sahip/ad` ile belirt.",
		en: "This repository's remote is not on GitHub (or there is none). Name it with `--repo owner/name`.",
	},
	MsgGHNotMergeable: {
		tr: "Pull request şu an birleştirilemiyor (çakışma, bekleyen kontrol ya da branch koruması).",
		en: "The pull request cannot be merged right now (conflicts, pending checks or branch protection).",
	},

	MsgAuthGitHubShort: {tr: "GitHub'a giriş yap (token keychain'e kaydedilir)", en: "sign in to GitHub (the token is kept in the keychain)"},
	MsgAuthGitHubToken: {tr: "kişisel erişim token'ını gir", en: "enter a personal access token"},
	MsgAuthGitHubGH:    {tr: "gh CLI'daki girişi kullan", en: "use the gh CLI's sign-in"},
	MsgAuthGitHubNoClient: {
		tr: "Bu sürümde tarayıcıyla giriş yok. `--gh` (gh CLI) ya da `--token` (github.com/settings/tokens) kullan.",
		en: "This build has no browser sign-in. Use `--gh` (gh CLI) or `--token` (github.com/settings/tokens).",
	},
	MsgAuthGitHubCode: {
		tr: "%s adresini aç ve şu kodu gir: %s",
		en: "Open %s and enter this code: %s",
	},
	MsgAuthGitHubWaiting: {tr: "Onay bekleniyor…", en: "Waiting for approval…"},
	MsgAuthGitHubDone:    {tr: "GitHub'a %s olarak giriş yapıldı.", en: "Signed in to GitHub as %s."},
	MsgAuthGitHubNoGH:    {tr: "gh CLI bulunamadı ya da giriş yapılmamış (`gh auth login`).", en: "The gh CLI is missing or not signed in (`gh auth login`)."},

	MsgRepoShort:       {tr: "GitHub repolarını oluştur, listele, kopyala", en: "create, list and clone GitHub repositories"},
	MsgRepoCreateShort: {tr: "GitHub'da repo oluştur ve bu klasörü bağla", en: "create a GitHub repository and connect this folder"},
	MsgRepoPrivate:     {tr: "gizli (private) repo", en: "private repository"},
	MsgRepoDescription: {tr: "açıklama", en: "description"},
	MsgRepoREADME:      {tr: "GitHub README ile başlatsın", en: "let GitHub start it with a README"},
	MsgRepoGitignore:   {tr: "GitHub'ın .gitignore şablonu (ör. Go)", en: "GitHub's .gitignore template (e.g. Go)"},
	MsgRepoLicense:     {tr: "lisans (ör. mit)", en: "license (e.g. mit)"},
	MsgRepoOrg:         {tr: "organizasyon altında oluştur", en: "create it in an organization"},
	MsgRepoPush:        {tr: "oluşturduktan sonra ilk push'u yap", en: "push right after creating it"},
	MsgRepoCreated:     {tr: "Repo oluşturuldu: %s", en: "Repository created: %s"},
	MsgRepoConnected:   {tr: "Bu klasör %s remote'u olarak bağlandı.", en: "This folder was connected as remote %s."},
	MsgRepoHasOrigin:   {tr: "Bu repoda zaten `origin` var; bağlantı değiştirilmedi.", en: "This repository already has an `origin`; it was left as is."},
	MsgRepoListShort:   {tr: "GitHub repolarını listele", en: "list your GitHub repositories"},
	MsgRepoCloneShort:  {tr: "GitHub reposunu kopyala (sahip/ad)", en: "clone a GitHub repository (owner/name)"},
	MsgRepoBadName:     {tr: "Repo adı sahip/ad biçiminde olmalı: %q", en: "The repository must be given as owner/name: %q"},
	MsgRepoPrivateTag:  {tr: "gizli", en: "private"},
	MsgRepoNotLinked:   {tr: "Bu repo GitHub'a bağlı değil.", en: "This repository is not on GitHub."},
	MsgRepoProtectedWarn: {
		tr: "Uyarı: %s GitHub'da korumalı; doğrudan push reddedilebilir. Ayrı bir branch'ten pull request açman gerekebilir.",
		en: "Warning: %s is protected on GitHub; a direct push may be rejected. You may need a pull request from another branch.",
	},

	MsgPRShort:         {tr: "pull request'leri yönet", en: "manage pull requests"},
	MsgPRCreateShort:   {tr: "bu branch'ten pull request aç", en: "open a pull request from this branch"},
	MsgPRTitle:         {tr: "başlık", en: "title"},
	MsgPRBody:          {tr: "açıklama", en: "description"},
	MsgPRBase:          {tr: "hedef branch (varsayılan: reponun ana branch'i)", en: "target branch (default: the repository's default branch)"},
	MsgPRDraft:         {tr: "taslak (draft) olarak aç", en: "open as a draft"},
	MsgPRAI:            {tr: "başlık ve açıklamayı yapay zekâ yazsın", en: "let the AI write the title and description"},
	MsgPRCreated:       {tr: "Pull request açıldı: #%d %s", en: "Pull request opened: #%d %s"},
	MsgPRListShort:     {tr: "pull request'leri listele", en: "list pull requests"},
	MsgPRState:         {tr: "durum: open, closed, all", en: "state: open, closed, all"},
	MsgPRNone:          {tr: "Pull request yok.", en: "No pull requests."},
	MsgPRViewShort:     {tr: "pull request'i göster", en: "show a pull request"},
	MsgPRMergeShort:    {tr: "pull request'i GitHub'da birleştir", en: "merge a pull request on GitHub"},
	MsgPRMerged:        {tr: "#%d birleştirildi (%s).", en: "#%d merged (%s)."},
	MsgPRCloseShort:    {tr: "pull request'i birleştirmeden kapat", en: "close a pull request without merging"},
	MsgPRClosed:        {tr: "#%d kapatıldı.", en: "#%d closed."},
	MsgPRCheckoutShort: {tr: "pull request'in branch'ine geç", en: "check out a pull request's branch"},
	MsgPRCommentShort:  {tr: "pull request'e ya da issue'ya yorum yaz", en: "comment on a pull request or issue"},
	MsgPRCommented:     {tr: "Yorum eklendi.", en: "Comment added."},
	MsgPRTitlePrompt:   {tr: "Başlık: ", en: "Title: "},
	MsgPRMergeConfirm: {
		tr: "#%d (%s → %s) %s ile GitHub'da birleştirilecek. Devam edilsin mi? [e/H] ",
		en: "#%d (%s → %s) will be merged on GitHub with %s. Go ahead? [y/N] ",
	},
	MsgPRBadNumber: {tr: "Geçersiz numara: %q", en: "Invalid number: %q"},

	MsgIssueShort:       {tr: "issue'ları yönet", en: "manage issues"},
	MsgIssueListShort:   {tr: "issue'ları listele", en: "list issues"},
	MsgIssueCreateShort: {tr: "issue aç", en: "open an issue"},
	MsgIssueCloseShort:  {tr: "issue'yu kapat", en: "close an issue"},
	MsgIssueLabelShort:  {tr: "issue'ya etiket ekle", en: "add labels to an issue"},
	MsgIssueLabels:      {tr: "etiketler (virgülle)", en: "labels (comma-separated)"},
	MsgIssueCreated:     {tr: "Issue açıldı: #%d %s", en: "Issue opened: #%d %s"},
	MsgIssueClosed:      {tr: "#%d kapatıldı.", en: "#%d closed."},
	MsgIssueLabeled:     {tr: "#%d etiketlendi.", en: "#%d labeled."},
	MsgIssueNone:        {tr: "Issue yok.", en: "No issues."},
	MsgIssueFixesHint: {
		tr: "İpucu: branch adı #%d numaralı issue'ya işaret ediyor; mesaja `Fixes #%d` eklersen merge'de issue kapanır.",
		en: "Tip: the branch name points at issue #%d; adding `Fixes #%d` to the message closes it on merge.",
	},

	MsgReleaseShort:       {tr: "GitHub release'leri", en: "GitHub releases"},
	MsgReleaseCreateShort: {tr: "bir tag'den release yayımla", en: "publish a release from a tag"},
	MsgReleaseNotes:       {tr: "release notları", en: "release notes"},
	MsgReleaseGenerate:    {tr: "notları GitHub birleştirilen PR'lardan yazsın", en: "let GitHub write the notes from merged pull requests"},
	MsgReleasePrerelease:  {tr: "ön sürüm (pre-release)", en: "pre-release"},
	MsgReleaseCreated:     {tr: "Release yayımlandı: %s", en: "Release published: %s"},
	MsgActionsShort:       {tr: "GitHub Actions'ın son çalışmalarını göster", en: "show the latest GitHub Actions runs"},
	MsgActionsNone:        {tr: "Bu branch için Actions çalışması yok.", en: "No Actions runs for this branch."},
}
