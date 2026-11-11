package i18n

// Texts of the advanced git commands (spec §4.6–4.12).
const (
	MsgMergeShort      Key = "cli.merge.short"
	MsgMergeNoFF       Key = "cli.merge.no_ff"
	MsgMergeFFOnly     Key = "cli.merge.ff_only"
	MsgMergeSquash     Key = "cli.merge.squash"
	MsgMergeMessage    Key = "cli.merge.message"
	MsgMergeDone       Key = "cli.merge.done"
	MsgMergeFF         Key = "cli.merge.ff"
	MsgMergeUpToDate   Key = "cli.merge.up_to_date"
	MsgMergeSquashed   Key = "cli.merge.squashed"
	MsgConflictsNext   Key = "cli.conflicts.next"
	MsgRebaseShort     Key = "cli.rebase.short"
	MsgRebaseInteract  Key = "cli.rebase.interactive"
	MsgRebaseDone      Key = "cli.rebase.done"
	MsgRebaseNothing   Key = "cli.rebase.nothing"
	MsgCherryPickShort Key = "cli.cherry_pick.short"
	MsgRevertShort     Key = "cli.revert.short"
	MsgRevertMainline  Key = "cli.revert.mainline"
	MsgContinueShort   Key = "cli.continue.short"
	MsgAbortShort      Key = "cli.abort.short"
	MsgSkipShort       Key = "cli.skip.short"
	MsgOperationDone   Key = "cli.operation.done"
	MsgConflictsShort  Key = "cli.conflicts.short"
	MsgConflictsNone   Key = "cli.conflicts.none"
	MsgResolveShort    Key = "cli.resolve.short"
	MsgResolveOurs     Key = "cli.resolve.ours"
	MsgResolveTheirs   Key = "cli.resolve.theirs"
	MsgResolveDone     Key = "cli.resolve.done"

	MsgStashShort      Key = "cli.stash.short"
	MsgStashSaveShort  Key = "cli.stash.save.short"
	MsgStashUntracked  Key = "cli.stash.untracked"
	MsgStashListShort  Key = "cli.stash.list.short"
	MsgStashApplyShort Key = "cli.stash.apply.short"
	MsgStashPopShort   Key = "cli.stash.pop.short"
	MsgStashDropShort  Key = "cli.stash.drop.short"
	MsgStashShowShort  Key = "cli.stash.show.short"
	MsgStashNone       Key = "cli.stash.none"
	MsgStashSaved      Key = "cli.stash.saved"
	MsgStashNothing    Key = "cli.stash.nothing"
	MsgStashApplied    Key = "cli.stash.applied"
	MsgStashDropped    Key = "cli.stash.dropped"

	MsgTagShort       Key = "cli.tag.short"
	MsgTagListShort   Key = "cli.tag.list.short"
	MsgTagNewShort    Key = "cli.tag.new.short"
	MsgTagMessage     Key = "cli.tag.message"
	MsgTagDeleteShort Key = "cli.tag.delete.short"
	MsgTagPushShort   Key = "cli.tag.push.short"
	MsgTagCreated     Key = "cli.tag.created"
	MsgTagDeleted     Key = "cli.tag.deleted"
	MsgTagPushed      Key = "cli.tag.pushed"
	MsgTagNone        Key = "cli.tag.none"

	MsgReflogShort  Key = "cli.reflog.short"
	MsgRecoverShort Key = "cli.recover.short"
	MsgRecoverDone  Key = "cli.recover.done"
	MsgBlameShort   Key = "cli.blame.short"

	MsgRemoteShort       Key = "cli.remote.short"
	MsgRemoteAddShort    Key = "cli.remote.add.short"
	MsgRemoteRemoveShort Key = "cli.remote.remove.short"
	MsgRemoteRenameShort Key = "cli.remote.rename.short"
	MsgRemoteURLShort    Key = "cli.remote.set_url.short"
	MsgRemotePushFlag    Key = "cli.remote.push"
	MsgRemoteNone        Key = "cli.remote.none"
	MsgSubmoduleShort    Key = "cli.submodule.short"
	MsgSubmoduleUpdate   Key = "cli.submodule.update"
	MsgSubmoduleNone     Key = "cli.submodule.none"
	MsgSubmoduleNotInit  Key = "cli.submodule.not_init"
	MsgSubmoduleModified Key = "cli.submodule.modified"

	MsgIdentityShort  Key = "cli.identity.short"
	MsgIdentityGlobal Key = "cli.identity.global"
	MsgIdentityName   Key = "cli.identity.name"
	MsgIdentityEmail  Key = "cli.identity.email"
	MsgIdentityShow   Key = "cli.identity.show"

	MsgPatchFlag       Key = "cli.patch"
	MsgHunkPickTitle   Key = "cli.hunk.pick_title"
	MsgHunksStaged     Key = "cli.hunks.staged"
	MsgTemplateFlag    Key = "cli.ignore.template"
	MsgTemplatesFlag   Key = "cli.ignore.templates"
	MsgTemplateAdded   Key = "cli.ignore.template_added"
	MsgTemplateUnknown Key = "cli.ignore.template_unknown"

	MsgWorkspaceShort       Key = "cli.workspace.short"
	MsgWorkspaceAddShort    Key = "cli.workspace.add.short"
	MsgWorkspaceRemoveShort Key = "cli.workspace.remove.short"
	MsgWorkspaceListShort   Key = "cli.workspace.list.short"
	MsgWorkspaceStatusShort Key = "cli.workspace.status.short"
	MsgWorkspaceFetchShort  Key = "cli.workspace.fetch.short"
	MsgWorkspacePullShort   Key = "cli.workspace.pull.short"
	MsgWorkspaceEmpty       Key = "cli.workspace.empty"
	MsgWorkspaceAdded       Key = "cli.workspace.added"
	MsgWorkspaceRemoved     Key = "cli.workspace.removed"
	MsgWorkspaceComma       Key = "cli.workspace.comma"
	MsgWorkspaceClean       Key = "cli.workspace.clean"
	MsgWorkspaceDirty       Key = "cli.workspace.dirty"

	MsgTUIHelpRebase Key = "tui.help.rebase"
	MsgTUIRebaseMsg  Key = "tui.rebase.message"
)

var advancedMessages = map[Key]message{
	MsgMergeShort:      {tr: "başka bir branch'i bu branch'e birleştir", en: "merge another branch into this one"},
	MsgMergeNoFF:       {tr: "her zaman merge commit'i oluştur", en: "always create a merge commit"},
	MsgMergeFFOnly:     {tr: "yalnızca ileri sar", en: "fast-forward only"},
	MsgMergeSquash:     {tr: "değişiklikleri tek commit olarak stage'e al (commit'lemez)", en: "stage the changes as one commit (does not commit)"},
	MsgMergeMessage:    {tr: "merge commit mesajı", en: "merge commit message"},
	MsgMergeDone:       {tr: "Birleştirildi: %s", en: "Merged: %s"},
	MsgMergeFF:         {tr: "İleri sarıldı: %s", en: "Fast-forwarded: %s"},
	MsgMergeUpToDate:   {tr: "Zaten güncel.", en: "Already up to date."},
	MsgMergeSquashed:   {tr: "Değişiklikler stage'de; commit'lemek için: gitpal commit", en: "The changes are staged; to commit them: gitpal commit"},
	MsgConflictsNext:   {tr: "Çakışmalar: gitpal conflicts • çöz: gitpal resolve <dosya> [--ours|--theirs] • sonra: gitpal continue • vazgeç: gitpal abort", en: "Conflicts: gitpal conflicts • resolve: gitpal resolve <file> [--ours|--theirs] • then: gitpal continue • give up: gitpal abort"},
	MsgRebaseShort:     {tr: "bu branch'in commit'lerini başka bir noktanın üzerine taşı", en: "replay this branch's commits onto another point"},
	MsgRebaseInteract:  {tr: "commit'leri sırala, birleştir, yeniden adlandır, at (interaktif)", en: "reorder, squash, reword, drop commits (interactive)"},
	MsgRebaseDone:      {tr: "Rebase tamamlandı: %s → %s", en: "Rebased: %s → %s"},
	MsgRebaseNothing:   {tr: "Taşınacak commit yok.", en: "There are no commits to replay."},
	MsgCherryPickShort: {tr: "başka commit'lerin değişikliklerini buraya uygula", en: "apply the changes of other commits here"},
	MsgRevertShort:     {tr: "commit'leri geri alan yeni commit'ler ekle", en: "add commits that undo other commits"},
	MsgRevertMainline:  {tr: "merge commit'i hangi ebeveyne göre geri alınsın (1 = birleştirilen branch)", en: "parent to revert a merge against (1 = the branch merged into)"},
	MsgContinueShort:   {tr: "çakışmalar çözüldükten sonra yarım işleme devam et", en: "carry on with the operation once conflicts are resolved"},
	MsgAbortShort:      {tr: "yarım kalan merge/rebase/cherry-pick/revert'ten vazgeç", en: "give up the merge/rebase/cherry-pick/revert under way"},
	MsgSkipShort:       {tr: "takıldığı commit'i atla ve devam et", en: "skip the commit it stopped at and carry on"},
	MsgOperationDone:   {tr: "%s: tamam.", en: "%s: done."},
	MsgConflictsShort:  {tr: "çakışan dosyaları listele", en: "list conflicted files"},
	MsgConflictsNone:   {tr: "Çakışan dosya yok.", en: "No conflicted files."},
	MsgResolveShort:    {tr: "çakışmayı çözülmüş işaretle ya da bir tarafı seç", en: "mark conflicts resolved, or take one side"},
	MsgResolveOurs:     {tr: "bizim tarafı al", en: "take our side"},
	MsgResolveTheirs:   {tr: "onların tarafını al", en: "take their side"},
	MsgResolveDone:     {tr: "Çözüldü: %s", en: "Resolved: %s"},

	MsgStashShort:      {tr: "değişiklikleri geçici olarak kenara koy", en: "put changes aside for a while"},
	MsgStashSaveShort:  {tr: "değişiklikleri stash'e koy", en: "stash the changes"},
	MsgStashUntracked:  {tr: "yeni (untracked) dosyaları da al", en: "include new (untracked) files"},
	MsgStashListShort:  {tr: "stash'leri listele", en: "list stashes"},
	MsgStashApplyShort: {tr: "stash'i geri uygula (stash kalır)", en: "apply a stash (the stash stays)"},
	MsgStashPopShort:   {tr: "stash'i geri uygula ve sil", en: "apply a stash and delete it"},
	MsgStashDropShort:  {tr: "stash'i sil (yedekli)", en: "delete a stash (backed up)"},
	MsgStashShowShort:  {tr: "stash'teki değişiklikleri göster", en: "show what a stash holds"},
	MsgStashNone:       {tr: "Stash boş.", en: "No stashes."},
	MsgStashSaved:      {tr: "Değişiklikler stash'e kondu.", en: "The changes were stashed."},
	MsgStashNothing:    {tr: "Kenara konacak değişiklik yok.", en: "There are no changes to stash."},
	MsgStashApplied:    {tr: "Stash uygulandı: %s", en: "Stash applied: %s"},
	MsgStashDropped:    {tr: "Stash silindi: %s (geri almak için: gitpal undo)", en: "Stash deleted: %s (to take it back: gitpal undo)"},

	MsgTagShort:       {tr: "tag'leri listele ve yönet", en: "list and manage tags"},
	MsgTagListShort:   {tr: "tag'leri listele", en: "list tags"},
	MsgTagNewShort:    {tr: "tag oluştur (-m ile açıklamalı)", en: "create a tag (annotated with -m)"},
	MsgTagMessage:     {tr: "tag mesajı (açıklamalı tag yapar)", en: "tag message (makes an annotated tag)"},
	MsgTagDeleteShort: {tr: "yerel tag'i sil", en: "delete a local tag"},
	MsgTagPushShort:   {tr: "tag'i uzak repoya gönder", en: "send a tag to the remote"},
	MsgTagCreated:     {tr: "Tag oluşturuldu: %s", en: "Tag created: %s"},
	MsgTagDeleted:     {tr: "Tag silindi: %s", en: "Tag deleted: %s"},
	MsgTagPushed:      {tr: "Tag gönderildi: %s → %s", en: "Tag pushed: %s → %s"},
	MsgTagNone:        {tr: "Tag yok.", en: "No tags."},

	MsgReflogShort:  {tr: "HEAD'in gittiği yerleri göster (kaybolan commit'leri bulmak için)", en: "show where HEAD has been (to find lost commits)"},
	MsgRecoverShort: {tr: "kaybolan bir commit'i yeni bir branch olarak geri getir", en: "bring a lost commit back as a new branch"},
	MsgRecoverDone:  {tr: "Geri getirildi: %s → %s", en: "Recovered: %s → %s"},
	MsgBlameShort:   {tr: "dosyanın her satırını son kimin değiştirdiğini göster", en: "show who last changed each line of a file"},

	MsgRemoteShort:       {tr: "remote'ları listele ve yönet", en: "list and manage remotes"},
	MsgRemoteAddShort:    {tr: "remote ekle", en: "add a remote"},
	MsgRemoteRemoveShort: {tr: "remote'u kaldır (uzak repoya dokunmaz)", en: "remove a remote (the remote itself is untouched)"},
	MsgRemoteRenameShort: {tr: "remote'u yeniden adlandır", en: "rename a remote"},
	MsgRemoteURLShort:    {tr: "remote adresini değiştir", en: "change a remote's address"},
	MsgRemotePushFlag:    {tr: "yalnızca push adresini değiştir", en: "change only the push address"},
	MsgRemoteNone:        {tr: "Remote yok.", en: "No remotes."},
	MsgSubmoduleShort:    {tr: "submodule'leri göster", en: "show submodules"},
	MsgSubmoduleUpdate:   {tr: "submodule'leri kaydedilen commit'e getir (gerekirse indir)", en: "check out the recorded commit of every submodule (fetching as needed)"},
	MsgSubmoduleNone:     {tr: "Submodule yok.", en: "No submodules."},
	MsgSubmoduleNotInit:  {tr: "başlatılmamış", en: "not initialized"},
	MsgSubmoduleModified: {tr: "kaydedilenden farklı commit'te", en: "on another commit than recorded"},

	MsgIdentityShort:  {tr: "commit kimliğini (ad ve e-posta) göster ya da ayarla", en: "show or set the commit identity (name and email)"},
	MsgIdentityGlobal: {tr: "tüm repolar için (genel) ayarla", en: "set it for every repository (global)"},
	MsgIdentityName:   {tr: "ad soyad", en: "full name"},
	MsgIdentityEmail:  {tr: "e-posta", en: "email"},
	MsgIdentityShow:   {tr: "Kimlik: %s <%s>", en: "Identity: %s <%s>"},

	MsgPatchFlag:       {tr: "dosyanın yalnızca seçilen parçaları (hunk)", en: "only chosen parts (hunks) of files"},
	MsgHunkPickTitle:   {tr: "Commit'e girecek parçaları seç", en: "Choose the hunks to stage"},
	MsgHunksStaged:     {tr: "%d parça işlendi.", en: "%d hunk(s) done."},
	MsgTemplateFlag:    {tr: "hazır şablon ekle (ör. go,node,flutter)", en: "add a ready-made template (e.g. go,node,flutter)"},
	MsgTemplatesFlag:   {tr: "hazır şablonları listele", en: "list the ready-made templates"},
	MsgTemplateAdded:   {tr: "Şablon eklendi: %s", en: "Template added: %s"},
	MsgTemplateUnknown: {tr: "Böyle bir şablon yok: %s. Liste: gitpal ignore --templates", en: "No such template: %s. See: gitpal ignore --templates"},

	MsgWorkspaceShort:       {tr: "birden çok repoyu birlikte yönet", en: "manage several repositories together"},
	MsgWorkspaceAddShort:    {tr: "repoyu çalışma alanına ekle", en: "add a repository to the workspace"},
	MsgWorkspaceRemoveShort: {tr: "repoyu çalışma alanından çıkar", en: "remove a repository from the workspace"},
	MsgWorkspaceListShort:   {tr: "çalışma alanındaki repoları listele", en: "list the workspace's repositories"},
	MsgWorkspaceStatusShort: {tr: "tüm repoların durumunu göster", en: "show the state of every repository"},
	MsgWorkspaceFetchShort:  {tr: "tüm repolarda fetch", en: "fetch in every repository"},
	MsgWorkspacePullShort:   {tr: "tüm repolarda pull (yalnızca ileri sarma)", en: "pull in every repository (fast-forward only)"},
	MsgWorkspaceEmpty:       {tr: "Çalışma alanı boş. Eklemek için: gitpal workspace add <yol>", en: "The workspace is empty. Add one with: gitpal workspace add <path>"},
	MsgWorkspaceAdded:       {tr: "Eklendi: %s", en: "Added: %s"},
	MsgWorkspaceRemoved:     {tr: "Çıkarıldı: %s", en: "Removed: %s"},
	MsgWorkspaceComma:       {tr: "Yolunda virgül olan repo çalışma alanına eklenemez: %s", en: "A repository whose path has a comma cannot be added: %s"},
	MsgWorkspaceClean:       {tr: "temiz", en: "clean"},
	MsgWorkspaceDirty:       {tr: "%d değişiklik", en: "%d change(s)"},

	MsgTUIHelpRebase: {tr: "↑/↓ gez • K/J taşı • p pick • r reword • s squash • f fixup • d drop • enter başlat • q vazgeç", en: "↑/↓ move • K/J reorder • p pick • r reword • s squash • f fixup • d drop • enter start • q cancel"},
	MsgTUIRebaseMsg:  {tr: "Yeni mesaj (enter: tamam, esc: vazgeç)", en: "New message (enter: done, esc: cancel)"},
}
