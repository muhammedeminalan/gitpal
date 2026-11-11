package i18n

// What the safety layer tells the user (spec §7): why an operation is risky
// or refused, and why something cannot be undone.
const (
	MsgSafetyOperationInProgress   Key = "safety.operation_in_progress"
	MsgSafetyProtectedForcePush    Key = "safety.protected_force_push"
	MsgSafetyBehindRemote          Key = "safety.behind_remote"
	MsgSafetyRewritesPushed        Key = "safety.rewrites_pushed"
	MsgSafetyProtectedBranchCommit Key = "safety.protected_branch_commit"
	MsgSafetyDetachedHeadCommit    Key = "safety.detached_head_commit"
	MsgSafetySecret                Key = "safety.secret"
	MsgSafetySecretFile            Key = "safety.secret_file"
	MsgSafetyLargeFile             Key = "safety.large_file"
	MsgSafetyJunkFile              Key = "safety.junk_file"
	MsgSafetyUncommittedChanges    Key = "safety.uncommitted_changes"
	MsgSafetyUnmergedBranch        Key = "safety.unmerged_branch"
	MsgSafetyProtectedBranchDelete Key = "safety.protected_branch_delete"
	MsgSafetyForcePush             Key = "safety.force_push"

	MsgSafetyBlocked              Key = "safety.blocked"
	MsgSafetyDeclined             Key = "safety.declined"
	MsgSafetyConfirmationRequired Key = "safety.confirmation_required"
	MsgSafetyRepositoryChanged    Key = "safety.repository_changed"

	MsgSafetyUndoNothing       Key = "safety.undo.nothing"
	MsgSafetyUndoPushed        Key = "safety.undo.pushed"
	MsgSafetyUndoMoved         Key = "safety.undo.moved"
	MsgSafetyUndoDirty         Key = "safety.undo.dirty"
	MsgSafetyUndoNoBackup      Key = "safety.undo.no_backup"
	MsgSafetyUndoFirstCommit   Key = "safety.undo.first_commit"
	MsgSafetyUndoBranchExists  Key = "safety.undo.branch_exists"
	MsgSafetyUndoNotSupported  Key = "safety.undo.not_supported"
	MsgSafetyBackupNotFound    Key = "safety.backup.not_found"
	MsgSafetyNothingToAbort    Key = "safety.nothing_to_abort"
	MsgSafetyBackupUnknownName Key = "safety.backup.unknown_name"
)

var safetyMessages = map[Key]message{
	MsgSafetyOperationInProgress: {
		tr: "Yarım kalmış bir %s var. Önce onu bitir ya da iptal et.",
		en: "A %s is still in progress. Finish or abort it first.",
	},
	MsgSafetyProtectedForcePush: {
		tr: "`%s` korumalı bir branch; zorla push engellendi. Değişikliğini ayrı bir branch'ten pull request ile gönder.",
		en: "`%s` is a protected branch, so the forced push was blocked. Send your change from another branch with a pull request.",
	},
	MsgSafetyBehindRemote: {
		tr: "Uzak branch'te sende olmayan %d commit var. Önce pull yapmalısın.",
		en: "The remote branch has %d commit(s) you do not have. Pull first.",
	},
	MsgSafetyRewritesPushed: {
		tr: "Bu işlem daha önce push edilmiş %d commit'i değiştiriyor. Başkaları bu commit'leri çekmiş olabilir; " +
			"sonrasında zorla push gerekir ve onların geçmişi bozulur.",
		en: "This rewrites %d commit(s) that were already pushed. Others may have pulled them; " +
			"it will need a forced push afterwards and breaks their history.",
	},
	MsgSafetyProtectedBranchCommit: {
		tr: "Doğrudan korumalı `%s` branch'ine commit atıyorsun. Bunun yerine yeni bir branch açmak ister misin?",
		en: "You are committing directly to the protected branch `%s`. Would you rather create a new branch?",
	},
	MsgSafetyDetachedHeadCommit: {
		tr: "Şu an bir branch üzerinde değilsin (detached HEAD). Bu commit hiçbir branch'e ait olmayacak ve " +
			"başka bir yere geçince kaybolabilir. Önce bir branch aç.",
		en: "You are not on a branch (detached HEAD). This commit will belong to no branch and can be lost " +
			"when you switch away. Create a branch first.",
	},
	MsgSafetySecret: {
		tr: "%s, satır %d: gizli bilgi gibi görünüyor (%s). Commit'lersen geçmişte kalır.",
		en: "%s, line %d: looks like a secret (%s). Once committed it stays in the history.",
	},
	MsgSafetySecretFile: {
		tr: "%s genellikle gizli bilgi içeren bir dosya. Commit'lemek yerine `.gitignore`'a ekle.",
		en: "%s is a kind of file that usually holds secrets. Add it to `.gitignore` instead of committing it.",
	},
	MsgSafetyLargeFile: {
		tr: "%s büyük bir dosya (%s). Git büyük dosyaları geçmişte sonsuza kadar taşır.",
		en: "%s is a large file (%s). Git carries large files in the history for ever.",
	},
	MsgSafetyJunkFile: {
		tr: "%s bağımlılık ya da derleme çıktısı gibi görünüyor (%d dosya). Bunlar genellikle commit'lenmez.",
		en: "%s looks like dependencies or build output (%d file(s)). These are usually not committed.",
	},
	MsgSafetyUncommittedChanges: {
		tr: "Commit'lenmemiş değişikliklerin var (%d dosya). İşlemden önce yedekleri alınacak.",
		en: "You have uncommitted changes (%d file(s)). They will be backed up before the operation.",
	},
	MsgSafetyUnmergedBranch: {
		tr: "Bu branch'te başka hiçbir yerde olmayan %d commit var; silersen branch listesinden kaybolurlar.",
		en: "This branch has %d commit(s) that exist nowhere else; deleting it removes them from every branch.",
	},
	MsgSafetyProtectedBranchDelete: {
		tr: "`%s` korumalı bir branch. Silmek istediğinden emin misin?",
		en: "`%s` is a protected branch. Are you sure you want to delete it?",
	},
	MsgSafetyForcePush: {
		tr: "Zorla push, uzak branch'teki %d commit'in yerine seninkileri koyacak.",
		en: "The forced push will replace %d commit(s) on the remote branch with yours.",
	},
	MsgSafetyBlocked: {
		tr: "Güvenlik katmanı bu işlemi engelledi.",
		en: "The safety layer blocked this operation.",
	},
	MsgSafetyDeclined: {
		tr: "İşlem onaylanmadı; hiçbir şey değişmedi.",
		en: "The operation was not confirmed; nothing changed.",
	},
	MsgSafetyConfirmationRequired: {
		tr: "Bu işlem açık onay gerektiriyor ve onay verebilecek biri yok (`--yes` tehlikeli işlemlerde geçerli değildir).",
		en: "This operation needs explicit confirmation and nobody is there to give it (`--yes` does not cover dangerous operations).",
	},
	MsgSafetyRepositoryChanged: {
		tr: "Onay beklenirken repo değişti. Güncel durumu görüp yeniden dene.",
		en: "The repository changed while waiting for confirmation. Look at the current state and try again.",
	},
	MsgSafetyUndoNothing: {
		tr: "Geri alınacak bir işlem yok.",
		en: "There is nothing to undo.",
	},
	MsgSafetyUndoPushed: {
		tr: "Bu işlem push edildi; yerelde geri almak uzak repoyu değiştirmez. " +
			"Etkisini geri almak için yeni bir commit ile `revert` kullan.",
		en: "This was pushed; undoing it locally does not change the remote. Use `revert` to undo its effect with a new commit.",
	},
	MsgSafetyUndoMoved: {
		tr: "Bu işlemden sonra repo değişti (başka commit atıldı ya da branch değişti); otomatik geri alma güvenli değil.",
		en: "The repository changed after this operation (new commits or another branch); undoing it automatically is not safe.",
	},
	MsgSafetyUndoDirty: {
		tr: "Commit'lenmemiş değişikliklerin var; geri alma onları ezebilir. Önce commit'le ya da stash'le.",
		en: "You have uncommitted changes that undoing could overwrite. Commit or stash them first.",
	},
	MsgSafetyUndoNoBackup: {
		tr: "Bu işlemin yedeği bulunamadı (süresi dolup silinmiş olabilir).",
		en: "The backup for this operation was not found (it may have expired).",
	},
	MsgSafetyUndoFirstCommit: {
		tr: "Reponun ilk commit'i geri alınamaz.",
		en: "The first commit of a repository cannot be undone.",
	},
	MsgSafetyUndoBranchExists: {
		tr: "`%s` adında bir branch yeniden oluşturulmuş; silinen branch aynı adla geri getirilemez.",
		en: "A branch named `%s` exists again; the deleted one cannot be brought back under the same name.",
	},
	MsgSafetyUndoNotSupported: {
		tr: "`%s` işlemi otomatik olarak geri alınamaz.",
		en: "A `%s` cannot be undone automatically.",
	},
	MsgSafetyNothingToAbort: {
		tr: "İptal edilecek yarım bir işlem (merge, rebase, cherry-pick, revert) yok.",
		en: "There is no merge, rebase, cherry-pick or revert under way to abort.",
	},
	MsgSafetyBackupNotFound: {
		tr: "Böyle bir yedek yok: %q.",
		en: "No such backup: %q.",
	},
	MsgSafetyBackupUnknownName: {
		tr: "Bu yedeğin hangi branch'e ait olduğu bilinmiyor; `git branch <ad> %s` ile elle geri getirebilirsin.",
		en: "It is not known which branch this backup belonged to; bring it back by hand with `git branch <name> %s`.",
	},
}
