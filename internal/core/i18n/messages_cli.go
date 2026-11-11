package i18n

// Texts of the everyday commands (spec §8).
const (
	MsgFlagRepo   Key = "cli.flag.repo"
	MsgFlagDryRun Key = "cli.flag.dry_run"
	MsgFlagYes    Key = "cli.flag.yes"

	MsgNotARepo        Key = "cli.not_a_repo"
	MsgNeedsTerminal   Key = "cli.needs_terminal"
	MsgNothingSelected Key = "cli.nothing_selected"
	MsgDone            Key = "cli.done"

	// Safety output.
	MsgRiskLabel          Key = "cli.risk.label"
	MsgRiskSafe           Key = "cli.risk.safe"
	MsgRiskCaution        Key = "cli.risk.caution"
	MsgRiskDangerous      Key = "cli.risk.dangerous"
	MsgRiskCritical       Key = "cli.risk.critical"
	MsgEffectsTitle       Key = "cli.effects.title"
	MsgFindingsTitle      Key = "cli.findings.title"
	MsgEffectFiles        Key = "cli.effect.files"
	MsgEffectCommits      Key = "cli.effect.commits"
	MsgEffectBranch       Key = "cli.effect.branch"
	MsgEffectRemote       Key = "cli.effect.remote"
	MsgEffectPushed       Key = "cli.effect.pushed"
	MsgEffectRewritten    Key = "cli.effect.rewritten"
	MsgEffectReplayed     Key = "cli.effect.replayed"
	MsgEffectStashDropped Key = "cli.effect.stash_dropped"
	MsgEffectAborted      Key = "cli.effect.aborted"
	MsgBackupCommit       Key = "cli.backup.commit"
	MsgBackupWorktree     Key = "cli.backup.worktree" //nolint:gosec // G101: a message key, not a credential
	MsgConfirmQuestion    Key = "cli.confirm.question"
	MsgConfirmUseYes      Key = "cli.confirm.use_yes"
	MsgConfirmExplicit    Key = "cli.confirm.explicit"
	MsgConfirmTypeBranch  Key = "cli.confirm.type_branch"
	MsgDryRunNothingDone  Key = "cli.dry_run.nothing_done"
	MsgBackupCreated      Key = "cli.backup.created"
	MsgBackupHowToRestore Key = "cli.backup.how_to_restore"
	MsgYesAnswers         Key = "cli.yes_answers"

	// status
	MsgStatusShort      Key = "cli.status.short"
	MsgStatusOnBranch   Key = "cli.status.on_branch"
	MsgStatusDetached   Key = "cli.status.detached"
	MsgStatusUnborn     Key = "cli.status.unborn"
	MsgStatusUpstream   Key = "cli.status.upstream"
	MsgStatusNoUpstream Key = "cli.status.no_upstream"
	MsgStatusGone       Key = "cli.status.gone"
	MsgStatusClean      Key = "cli.status.clean"
	MsgStatusStaged     Key = "cli.status.staged"
	MsgStatusUnstaged   Key = "cli.status.unstaged"
	MsgStatusUntracked  Key = "cli.status.untracked"
	MsgStatusConflicted Key = "cli.status.conflicted"
	MsgStatusOperation  Key = "cli.status.operation"

	// add / unstage / discard / ignore
	MsgAddShort      Key = "cli.add.short"
	MsgAddAllFlag    Key = "cli.add.all"
	MsgAddPickTitle  Key = "cli.add.pick_title"
	MsgAddDone       Key = "cli.add.done"
	MsgUnstageShort  Key = "cli.unstage.short"
	MsgUnstageDone   Key = "cli.unstage.done"
	MsgDiscardShort  Key = "cli.discard.short"
	MsgDiscardDone   Key = "cli.discard.done"
	MsgIgnoreShort   Key = "cli.ignore.short"
	MsgIgnoreDone    Key = "cli.ignore.done"
	MsgDiffShort     Key = "cli.diff.short"
	MsgDiffStaged    Key = "cli.diff.staged"
	MsgDiffBinary    Key = "cli.diff.binary"
	MsgDiffNoChanges Key = "cli.diff.no_changes"

	// commit
	MsgCommitShort      Key = "cli.commit.short"
	MsgCommitMessage    Key = "cli.commit.message"
	MsgCommitAmend      Key = "cli.commit.amend"
	MsgCommitAllowEmpty Key = "cli.commit.allow_empty"
	MsgCommitPrompt     Key = "cli.commit.prompt"
	MsgCommitDone       Key = "cli.commit.done"
	MsgCommitNoMessage  Key = "cli.commit.no_message"
	MsgCommitAI         Key = "cli.commit.ai"

	// save
	MsgSaveShort      Key = "cli.save.short"
	MsgSaveNoPush     Key = "cli.save.no_push"
	MsgSaveNothing    Key = "cli.save.nothing"
	MsgSaveStep       Key = "cli.save.step"
	MsgSaveStepStage  Key = "cli.save.step.stage"
	MsgSaveStepCommit Key = "cli.save.step.commit"
	MsgSaveStepPush   Key = "cli.save.step.push"
	MsgSaveSummary    Key = "cli.save.summary"

	// push / pull / fetch
	MsgPushShort       Key = "cli.push.short"
	MsgPushForce       Key = "cli.push.force"
	MsgPushSetUpstream Key = "cli.push.set_upstream"
	MsgPushDone        Key = "cli.push.done"
	MsgPushUpToDate    Key = "cli.push.up_to_date"
	MsgPullShort       Key = "cli.pull.short"
	MsgPullRebase      Key = "cli.pull.rebase"
	MsgPullMerge       Key = "cli.pull.merge"
	MsgPullFFOnly      Key = "cli.pull.ff_only"
	MsgPullDone        Key = "cli.pull.done"
	MsgPullUpToDate    Key = "cli.pull.up_to_date"
	MsgFetchShort      Key = "cli.fetch.short"
	MsgFetchAll        Key = "cli.fetch.all"
	MsgFetchPrune      Key = "cli.fetch.prune"
	MsgFetchDone       Key = "cli.fetch.done"

	// branch
	MsgBranchShort        Key = "cli.branch.short"
	MsgBranchListShort    Key = "cli.branch.list.short"
	MsgBranchRemotes      Key = "cli.branch.remotes"
	MsgBranchNewShort     Key = "cli.branch.new.short"
	MsgBranchNoSwitch     Key = "cli.branch.no_switch"
	MsgBranchSwitchShort  Key = "cli.branch.switch.short"
	MsgBranchDeleteShort  Key = "cli.branch.delete.short"
	MsgBranchForce        Key = "cli.branch.force"
	MsgBranchRenameShort  Key = "cli.branch.rename.short"
	MsgBranchCleanShort   Key = "cli.branch.clean.short"
	MsgBranchCleanNone    Key = "cli.branch.clean.none"
	MsgBranchCreated      Key = "cli.branch.created"
	MsgBranchSwitched     Key = "cli.branch.switched"
	MsgBranchDeleted      Key = "cli.branch.deleted"
	MsgBranchRenamed      Key = "cli.branch.renamed"
	MsgBranchSuggest      Key = "cli.branch.suggest"
	MsgBranchAheadBehind  Key = "cli.branch.ahead_behind"
	MsgBranchUpstreamGone Key = "cli.branch.upstream_gone"

	// log
	MsgLogShort  Key = "cli.log.short"
	MsgLogMax    Key = "cli.log.max"
	MsgLogAuthor Key = "cli.log.author"
	MsgLogGrep   Key = "cli.log.grep"
	MsgLogGraph  Key = "cli.log.graph"
	MsgLogAll    Key = "cli.log.all"
	MsgLogEmpty  Key = "cli.log.empty"

	// undo / backups / history
	MsgUndoShort          Key = "cli.undo.short"
	MsgUndoWouldUndo      Key = "cli.undo.would_undo"
	MsgUndoDone           Key = "cli.undo.done"
	MsgBackupsShort       Key = "cli.backups.short"
	MsgBackupsListShort   Key = "cli.backups.list.short"
	MsgBackupsRestore     Key = "cli.backups.restore.short"
	MsgBackupsPruneShort  Key = "cli.backups.prune.short"
	MsgBackupsNone        Key = "cli.backups.none"
	MsgBackupsPruned      Key = "cli.backups.pruned"
	MsgBackupsRestored    Key = "cli.backups.restored"
	MsgBackupsWorktreeTag Key = "cli.backups.worktree_tag"
	MsgHistoryShort       Key = "cli.history.short"
	MsgHistoryEmpty       Key = "cli.history.empty"
	MsgHistoryFailed      Key = "cli.history.failed"
	MsgHistoryUndone      Key = "cli.history.undone"

	// config
	MsgConfigShort     Key = "cli.config.short"
	MsgConfigGetShort  Key = "cli.config.get.short"
	MsgConfigSetShort  Key = "cli.config.set.short"
	MsgConfigListShort Key = "cli.config.list.short"
	MsgConfigPathShort Key = "cli.config.path.short"
	MsgConfigEditShort Key = "cli.config.edit.short"
	MsgConfigUnknown   Key = "cli.config.unknown"
	MsgConfigSaved     Key = "cli.config.saved"
	MsgConfigNoEditor  Key = "cli.config.no_editor"

	// init / clone
	MsgInitShort   Key = "cli.init.short"
	MsgInitBranch  Key = "cli.init.branch"
	MsgInitDone    Key = "cli.init.done"
	MsgCloneShort  Key = "cli.clone.short"
	MsgCloneBranch Key = "cli.clone.branch"
	MsgCloneDepth  Key = "cli.clone.depth"
	MsgCloneDone   Key = "cli.clone.done"

	// TUI
	MsgTUIHelpPicker    Key = "tui.help.picker"
	MsgTUIHelpDashboard Key = "tui.help.dashboard"
	MsgTUIStaged        Key = "tui.staged"
	MsgTUIChanges       Key = "tui.changes"
	MsgTUINoChanges     Key = "tui.no_changes"
	MsgTUICommitPrompt  Key = "tui.commit_prompt"
	MsgTUIConfirmHelp   Key = "tui.confirm_help"
	MsgTUIWorking       Key = "tui.working"
	MsgTUITypeBranch    Key = "tui.type_branch"
)

var cliMessages = map[Key]message{
	MsgFlagRepo:   {tr: "bu klasördeki repoda çalış", en: "work in the repository at this path"},
	MsgFlagDryRun: {tr: "hiçbir şeyi değiştirmeden ne olacağını göster", en: "show what would happen without changing anything"},
	MsgFlagYes: {
		tr: "\"Dikkat\" sınıfındaki onayları atla (tehlikeli işlemlerde geçmez)",
		en: "skip confirmations of the \"caution\" class (never for dangerous operations)",
	},

	MsgNotARepo:        {tr: "Burası bir git reposu değil: %s", en: "This is not a git repository: %s"},
	MsgNeedsTerminal:   {tr: "Bu komut bir terminalde etkileşim gerektiriyor; %s kullan.", en: "This needs an interactive terminal; use %s."},
	MsgNothingSelected: {tr: "Hiçbir şey seçilmedi.", en: "Nothing was selected."},
	MsgDone:            {tr: "Tamam.", en: "Done."},

	MsgRiskLabel:          {tr: "Risk", en: "Risk"},
	MsgRiskSafe:           {tr: "güvenli", en: "safe"},
	MsgRiskCaution:        {tr: "dikkat", en: "caution"},
	MsgRiskDangerous:      {tr: "tehlikeli", en: "dangerous"},
	MsgRiskCritical:       {tr: "çok tehlikeli", en: "critical"},
	MsgEffectsTitle:       {tr: "Ne olacak:", en: "What will happen:"},
	MsgFindingsTitle:      {tr: "Dikkat edilmesi gerekenler:", en: "Things to look at:"},
	MsgEffectFiles:        {tr: "%d dosyadaki commit'lenmemiş değişiklik atılacak: %s", en: "uncommitted changes in %d file(s) are thrown away: %s"},
	MsgEffectCommits:      {tr: "%d commit branch'ten çıkacak", en: "%d commit(s) leave the branch"},
	MsgEffectBranch:       {tr: "branch silinecek: %s", en: "branch deleted: %s"},
	MsgEffectRemote:       {tr: "uzak branch'teki %d commit seninkilerle değiştirilecek", en: "%d commit(s) on the remote branch are replaced by yours"},
	MsgEffectPushed:       {tr: "%d commit gönderilecek", en: "%d commit(s) are sent"},
	MsgEffectRewritten:    {tr: "son commit yenisiyle değiştirilecek (%s)", en: "the last commit is replaced by a new one (%s)"},
	MsgEffectReplayed:     {tr: "%d commit yeniden yazılacak (yeni hash'ler alacak)", en: "%d commit(s) are rewritten (they get new hashes)"},
	MsgEffectStashDropped: {tr: "stash silinecek: %s", en: "stash entry deleted: %s"},
	MsgEffectAborted:      {tr: "yarım kalan %s iptal edilecek; çakışma çözümlerin kaybolur", en: "the %s under way is given up, with your conflict resolutions"},
	MsgBackupCommit:       {tr: "Yedek: %s commit'i saklanacak", en: "Backup: commit %s is kept"},
	MsgBackupWorktree:     {tr: "Yedek: commit'lenmemiş değişikliklerin saklanacak", en: "Backup: your uncommitted changes are kept"},
	MsgConfirmUseYes: {
		tr: "Bu işlem onay istiyor ama soracak bir terminal yok. Onaylıyorsan komutu `--yes` ile çalıştır.",
		en: "This operation asks for confirmation but there is no terminal to ask in. If you agree, run the command with `--yes`.",
	},
	MsgConfirmQuestion:    {tr: "Devam edilsin mi? [e/H] ", en: "Go ahead? [y/N] "},
	MsgConfirmExplicit:    {tr: "Bu işlem tehlikeli. Devam etmek için \"evet\" yaz: ", en: "This is dangerous. Type \"yes\" to go ahead: "},
	MsgConfirmTypeBranch:  {tr: "Onaylamak için branch adını yaz (%s): ", en: "Type the branch name to confirm (%s): "},
	MsgDryRunNothingDone:  {tr: "Kuru çalıştırma: hiçbir şey değişmedi.", en: "Dry run: nothing was changed."},
	MsgBackupCreated:      {tr: "Yedek alındı: %s", en: "Backup taken: %s"},
	MsgBackupHowToRestore: {tr: "Geri almak için: gitpal undo", en: "To take it back: gitpal undo"},
	MsgYesAnswers:         {tr: "e,evet,y,yes", en: "y,yes"},

	MsgStatusShort:      {tr: "çalışma alanının durumunu göster", en: "show the state of the working tree"},
	MsgStatusOnBranch:   {tr: "Branch: %s", en: "Branch: %s"},
	MsgStatusDetached:   {tr: "Branch yok (detached HEAD, %s)", en: "No branch (detached HEAD, %s)"},
	MsgStatusUnborn:     {tr: "Branch: %s (henüz commit yok)", en: "Branch: %s (no commits yet)"},
	MsgStatusUpstream:   {tr: "Upstream: %s — %d ileride, %d geride", en: "Upstream: %s — %d ahead, %d behind"},
	MsgStatusNoUpstream: {tr: "Upstream yok (ilk push'ta ayarlanır)", en: "No upstream (set on the first push)"},
	MsgStatusGone:       {tr: "Upstream %s uzak repoda silinmiş", en: "Upstream %s was deleted on the remote"},
	MsgStatusClean:      {tr: "Commit'lenecek bir şey yok, çalışma alanı temiz.", en: "Nothing to commit, the working tree is clean."},
	MsgStatusStaged:     {tr: "Commit'e girecek (staged):", en: "Staged for commit:"},
	MsgStatusUnstaged:   {tr: "Değişmiş, henüz eklenmemiş:", en: "Changed, not staged:"},
	MsgStatusUntracked:  {tr: "Yeni dosyalar (untracked):", en: "New files (untracked):"},
	MsgStatusConflicted: {tr: "Çakışan dosyalar:", en: "Conflicted files:"},
	MsgStatusOperation:  {tr: "Yarım kalan işlem: %s", en: "Operation in progress: %s"},

	MsgAddShort:      {tr: "dosyaları commit'e ekle (stage)", en: "stage files for the next commit"},
	MsgAddAllFlag:    {tr: "tüm değişiklikleri ekle", en: "stage every change"},
	MsgAddPickTitle:  {tr: "Commit'e girecek dosyaları seç", en: "Choose the files to commit"},
	MsgAddDone:       {tr: "%d dosya eklendi.", en: "%d file(s) staged."},
	MsgUnstageShort:  {tr: "dosyaları stage'den çıkar (değişiklikler kalır)", en: "unstage files (the changes are kept)"},
	MsgUnstageDone:   {tr: "Stage'den çıkarıldı.", en: "Unstaged."},
	MsgDiscardShort:  {tr: "dosyalardaki commit'lenmemiş değişiklikleri at (yedekli)", en: "throw away uncommitted changes to files (backed up)"},
	MsgDiscardDone:   {tr: "Değişiklikler atıldı. Geri almak için: gitpal undo", en: "Changes thrown away. To take it back: gitpal undo"},
	MsgIgnoreShort:   {tr: "dosyaları .gitignore'a ekle", en: "add files to .gitignore"},
	MsgIgnoreDone:    {tr: ".gitignore güncellendi.", en: ".gitignore updated."},
	MsgDiffShort:     {tr: "değişiklikleri göster", en: "show changes"},
	MsgDiffStaged:    {tr: "commit'e girecek değişiklikleri göster", en: "show the staged changes"},
	MsgDiffBinary:    {tr: "binary dosya değişti", en: "binary file changed"},
	MsgDiffNoChanges: {tr: "Değişiklik yok.", en: "No changes."},

	MsgCommitShort:      {tr: "stage'deki değişiklikleri commit'le", en: "commit the staged changes"},
	MsgCommitMessage:    {tr: "commit mesajı (birden çok kez verilirse paragraf olur)", en: "commit message (repeat for more paragraphs)"},
	MsgCommitAmend:      {tr: "son commit'i düzelt", en: "amend the last commit"},
	MsgCommitAllowEmpty: {tr: "değişiklik olmadan commit'e izin ver", en: "allow a commit without changes"},
	MsgCommitPrompt:     {tr: "Commit mesajı: ", en: "Commit message: "},
	MsgCommitDone:       {tr: "Commit atıldı: %s %s", en: "Committed: %s %s"},
	MsgCommitNoMessage:  {tr: "Commit mesajı gerekli: -m \"mesaj\"", en: "A commit message is needed: -m \"message\""},
	MsgCommitAI:         {tr: "mesajı yapay zekâya önerttir", en: "have the AI suggest the message"},

	MsgSaveShort:      {tr: "değişiklikleri seç, commit'le ve push et (tek akış)", en: "pick changes, commit and push in one go"},
	MsgSaveNoPush:     {tr: "push etme, yalnızca commit'le", en: "commit only, do not push"},
	MsgSaveNothing:    {tr: "Kaydedilecek bir değişiklik yok.", en: "There is nothing to save."},
	MsgSaveStep:       {tr: "[%d/3] %s", en: "[%d/3] %s"},
	MsgSaveStepStage:  {tr: "Dosyalar seçiliyor", en: "Picking files"},
	MsgSaveStepCommit: {tr: "Commit", en: "Commit"},
	MsgSaveStepPush:   {tr: "Push", en: "Push"},
	MsgSaveSummary:    {tr: "Kaydedildi: %d dosya, commit %s, %s", en: "Saved: %d file(s), commit %s, %s"},

	MsgPushShort:       {tr: "commit'leri uzak repoya gönder", en: "send commits to the remote"},
	MsgPushForce:       {tr: "zorla push (her zaman --force-with-lease olarak)", en: "force push (always as --force-with-lease)"},
	MsgPushSetUpstream: {tr: "upstream'i ayarla", en: "set the upstream"},
	MsgPushDone:        {tr: "Gönderildi: %s → %s/%s", en: "Pushed: %s → %s/%s"},
	MsgPushUpToDate:    {tr: "Uzak repo zaten güncel.", en: "The remote is already up to date."},
	MsgPullShort:       {tr: "uzak repodaki değişiklikleri al", en: "bring in changes from the remote"},
	MsgPullRebase:      {tr: "rebase ile birleştir", en: "integrate with a rebase"},
	MsgPullMerge:       {tr: "merge ile birleştir", en: "integrate with a merge"},
	MsgPullFFOnly:      {tr: "yalnızca ileri sar (fast-forward)", en: "fast-forward only"},
	MsgPullDone:        {tr: "Güncellendi: %s → %s", en: "Updated: %s → %s"},
	MsgPullUpToDate:    {tr: "Zaten güncel.", en: "Already up to date."},
	MsgFetchShort:      {tr: "uzak repodaki yenilikleri indir (yerel branch'e dokunmaz)", en: "download news from the remote (local branches stay)"},
	MsgFetchAll:        {tr: "tüm remote'lardan", en: "from every remote"},
	MsgFetchPrune:      {tr: "uzakta silinmiş branch'leri temizle", en: "remove branches deleted on the remote"},
	MsgFetchDone:       {tr: "Fetch tamamlandı.", en: "Fetched."},

	MsgBranchShort:        {tr: "branch'leri listele ve yönet", en: "list and manage branches"},
	MsgBranchListShort:    {tr: "branch'leri listele", en: "list branches"},
	MsgBranchRemotes:      {tr: "uzak branch'leri de göster", en: "also show remote branches"},
	MsgBranchNewShort:     {tr: "yeni branch aç ve ona geç", en: "create a branch and switch to it"},
	MsgBranchNoSwitch:     {tr: "oluştur ama geçme", en: "create without switching"},
	MsgBranchSwitchShort:  {tr: "başka bir branch'e geç", en: "switch to another branch"},
	MsgBranchDeleteShort:  {tr: "branch sil (yedekli)", en: "delete a branch (backed up)"},
	MsgBranchForce:        {tr: "birleşmemiş commit'leri olsa da sil", en: "delete even with unmerged commits"},
	MsgBranchRenameShort:  {tr: "branch'i yeniden adlandır", en: "rename a branch"},
	MsgBranchCleanShort:   {tr: "birleştirilmiş branch'leri toplu sil", en: "delete merged branches"},
	MsgBranchCleanNone:    {tr: "Silinecek birleştirilmiş branch yok.", en: "No merged branches to delete."},
	MsgBranchCreated:      {tr: "Branch oluşturuldu: %s", en: "Branch created: %s"},
	MsgBranchSwitched:     {tr: "Şu an %s branch'indesin.", en: "Now on %s."},
	MsgBranchDeleted:      {tr: "Branch silindi: %s (geri almak için: gitpal undo)", en: "Branch deleted: %s (to take it back: gitpal undo)"},
	MsgBranchRenamed:      {tr: "Yeniden adlandırıldı: %s → %s", en: "Renamed: %s → %s"},
	MsgBranchSuggest:      {tr: "Geçerli bir ad değil. Öneri: %s", en: "Not a valid name. Suggestion: %s"},
	MsgBranchAheadBehind:  {tr: "%d ileride, %d geride", en: "%d ahead, %d behind"},
	MsgBranchUpstreamGone: {tr: "upstream silinmiş", en: "upstream gone"},

	MsgLogShort:  {tr: "commit geçmişini göster", en: "show the commit history"},
	MsgLogMax:    {tr: "en fazla bu kadar commit", en: "at most this many commits"},
	MsgLogAuthor: {tr: "yazara göre süz", en: "filter by author"},
	MsgLogGrep:   {tr: "mesajda geçen metne göre süz", en: "filter by text in the message"},
	MsgLogGraph:  {tr: "branch grafiğini çiz", en: "draw the branch graph"},
	MsgLogAll:    {tr: "tüm branch'ler", en: "every branch"},
	MsgLogEmpty:  {tr: "Henüz commit yok.", en: "No commits yet."},

	MsgUndoShort:          {tr: "gitpal'ın yaptığı son işlemi geri al", en: "undo the last operation gitpal performed"},
	MsgUndoWouldUndo:      {tr: "Geri alınacak: %s (%s)", en: "Will undo: %s (%s)"},
	MsgUndoDone:           {tr: "Geri alındı: %s", en: "Undone: %s"},
	MsgBackupsShort:       {tr: "gitpal'ın aldığı yedekleri yönet", en: "manage the backups gitpal took"},
	MsgBackupsListShort:   {tr: "yedekleri listele", en: "list backups"},
	MsgBackupsRestore:     {tr: "bir yedeği geri yükle", en: "restore a backup"},
	MsgBackupsPruneShort:  {tr: "saklama süresi dolan yedekleri sil", en: "delete backups past their retention"},
	MsgBackupsNone:        {tr: "Yedek yok.", en: "No backups."},
	MsgBackupsPruned:      {tr: "%d yedek silindi.", en: "%d backup(s) deleted."},
	MsgBackupsRestored:    {tr: "Yedek geri yüklendi: %s", en: "Backup restored: %s"},
	MsgBackupsWorktreeTag: {tr: "commit'lenmemiş değişiklikler", en: "uncommitted changes"},
	MsgHistoryShort:       {tr: "gitpal'ın bu repoda yaptığı işlemleri göster", en: "show what gitpal did in this repository"},
	MsgHistoryEmpty:       {tr: "Kayıtlı işlem yok.", en: "No operations recorded."},
	MsgHistoryFailed:      {tr: "başarısız: %s", en: "failed: %s"},
	MsgHistoryUndone:      {tr: "geri alındı", en: "undone"},

	MsgConfigShort:     {tr: "ayarları göster ve değiştir", en: "show and change settings"},
	MsgConfigGetShort:  {tr: "bir ayarın değerini göster", en: "show a setting"},
	MsgConfigSetShort:  {tr: "bir ayarı değiştir (genel ayar dosyasında)", en: "change a setting (in the global file)"},
	MsgConfigListShort: {tr: "tüm ayarları göster", en: "show every setting"},
	MsgConfigPathShort: {tr: "ayar dosyalarının yerini göster", en: "show where the settings files are"},
	MsgConfigEditShort: {tr: "ayar dosyasını düzenleyicide aç", en: "open the settings file in an editor"},
	MsgConfigUnknown:   {tr: "Böyle bir ayar yok: %s", en: "No such setting: %s"},
	MsgConfigSaved:     {tr: "Kaydedildi: %s = %s", en: "Saved: %s = %s"},
	MsgConfigNoEditor:  {tr: "Düzenleyici bulunamadı; EDITOR değişkenini ayarla ya da dosyayı elle aç: %s", en: "No editor found; set EDITOR or open the file yourself: %s"},

	MsgInitShort:   {tr: "yeni bir repo oluştur", en: "create a repository"},
	MsgInitBranch:  {tr: "ilk branch'in adı", en: "name of the first branch"},
	MsgInitDone:    {tr: "Repo oluşturuldu: %s", en: "Repository created: %s"},
	MsgCloneShort:  {tr: "bir repoyu kopyala", en: "copy a repository"},
	MsgCloneBranch: {tr: "bu branch'i aç", en: "check out this branch"},
	MsgCloneDepth:  {tr: "yalnızca son N commit (sığ kopya)", en: "only the last N commits (shallow)"},
	MsgCloneDone:   {tr: "Kopyalandı: %s", en: "Cloned: %s"},

	MsgTUIHelpPicker:    {tr: "↑/↓ gez • boşluk seç • a hepsi • n hiçbiri • enter onayla • q vazgeç", en: "↑/↓ move • space toggle • a all • n none • enter confirm • q cancel"},
	MsgTUIHelpDashboard: {tr: "↑/↓ gez • boşluk stage • a hepsini ekle • c commit • p push • l pull • f fetch • r yenile • q çık", en: "↑/↓ move • space stage • a stage all • c commit • p push • l pull • f fetch • r refresh • q quit"},
	MsgTUIStaged:        {tr: "Stage'de", en: "Staged"},
	MsgTUIChanges:       {tr: "Değişiklikler", en: "Changes"},
	MsgTUINoChanges:     {tr: "Değişiklik yok.", en: "No changes."},
	MsgTUICommitPrompt:  {tr: "Commit mesajı (enter: commit, esc: vazgeç)", en: "Commit message (enter: commit, esc: cancel)"},
	MsgTUIConfirmHelp:   {tr: "y onayla • n vazgeç", en: "y confirm • n cancel"},
	MsgTUIWorking:       {tr: "Çalışıyor…", en: "Working…"},
	MsgTUITypeBranch:    {tr: "Onaylamak için branch adını yaz: %s", en: "Type the branch name to confirm: %s"},
}
