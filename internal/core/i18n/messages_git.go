package i18n

// Explanations of git failures (spec §11) and of arguments refused before git
// ran. Each explanation says what happened and what to do next.
const (
	MsgGitFailed Key = "git.failed"
	MsgGitOutput Key = "git.output"

	MsgGitKindGitNotFound         Key = "git.kind.git_not_found"
	MsgGitKindDirNotFound         Key = "git.kind.dir_not_found"
	MsgGitKindTimeout             Key = "git.kind.timeout"
	MsgGitKindCanceled            Key = "git.kind.canceled"
	MsgGitKindNotARepo            Key = "git.kind.not_a_repo"
	MsgGitKindNothingToCommit     Key = "git.kind.nothing_to_commit"
	MsgGitKindIdentityMissing     Key = "git.kind.identity_missing"
	MsgGitKindHookFailed          Key = "git.kind.hook_failed"
	MsgGitKindAuthFailed          Key = "git.kind.auth_failed"
	MsgGitKindRemoteUnreachable   Key = "git.kind.remote_unreachable"
	MsgGitKindNoRemote            Key = "git.kind.no_remote"
	MsgGitKindRejected            Key = "git.kind.push_rejected"
	MsgGitKindStaleLease          Key = "git.kind.stale_lease"
	MsgGitKindRemoteDeclined      Key = "git.kind.remote_declined"
	MsgGitKindNoUpstream          Key = "git.kind.no_upstream"
	MsgGitKindDiverged            Key = "git.kind.diverged"
	MsgGitKindDirtyWorktree       Key = "git.kind.dirty_worktree"
	MsgGitKindConflict            Key = "git.kind.conflict"
	MsgGitKindOperationInProgress Key = "git.kind.operation_in_progress"
	MsgGitKindDetachedHead        Key = "git.kind.detached_head"
	MsgGitKindUnbornBranch        Key = "git.kind.unborn_branch"
	MsgGitKindBranchExists        Key = "git.kind.branch_exists"
	MsgGitKindBranchNotMerged     Key = "git.kind.branch_not_merged"
	MsgGitKindInvalidName         Key = "git.kind.invalid_name"
	MsgGitKindPathspecNoMatch     Key = "git.kind.pathspec_no_match"
	MsgGitKindPathIgnored         Key = "git.kind.path_ignored"
	MsgGitKindRefNotFound         Key = "git.kind.ref_not_found"
	MsgGitKindIndexLocked         Key = "git.kind.index_locked"

	MsgGitArgNoPaths       Key = "git.arg.no_paths"
	MsgGitArgInvalidPath   Key = "git.arg.invalid_path"
	MsgGitArgEmptyMessage  Key = "git.arg.empty_message"
	MsgGitArgInvalidBranch Key = "git.arg.invalid_branch"
	MsgGitArgInvalidRemote Key = "git.arg.invalid_remote"
	MsgGitArgInvalidRev    Key = "git.arg.invalid_rev"
)

var gitMessages = map[Key]message{
	MsgGitFailed: {
		tr: "`%s` başarısız oldu (çıkış kodu %d).",
		en: "`%s` failed (exit status %d).",
	},
	MsgGitOutput: {
		tr: "git'in çıktısı:",
		en: "git said:",
	},
	MsgGitKindGitNotFound: {
		tr: "git bulunamadı. git'i kur ve PATH'te olduğundan emin ol.",
		en: "git was not found. Install git and make sure it is on your PATH.",
	},
	MsgGitKindDirNotFound: {
		tr: "Klasör bulunamadı. Repo yolunu kontrol et.",
		en: "The folder does not exist. Check the repository path.",
	},
	MsgGitKindTimeout: {
		tr: "git komutu çok uzun sürdü ve durduruldu. Ağ bağlantını kontrol edip yeniden dene.",
		en: "The git command took too long and was stopped. Check your network connection and try again.",
	},
	MsgGitKindCanceled: {
		tr: "İşlem iptal edildi.",
		en: "The operation was canceled.",
	},
	MsgGitKindNotARepo: {
		tr: "Bu klasör bir git reposu değil. Doğru klasörü seç ya da `git init` ile repo oluştur.",
		en: "This folder is not a git repository. Pick the right folder or create one with `git init`.",
	},
	MsgGitKindNothingToCommit: {
		tr: "Commit'lenecek değişiklik yok. Önce dosya seç (stage).",
		en: "There is nothing to commit. Stage some changes first.",
	},
	MsgGitKindIdentityMissing: {
		tr: "git kimliğin ayarlı değil. `git config --global user.name \"Ad Soyad\"` ve " +
			"`git config --global user.email \"sen@ornek.com\"` komutlarını çalıştır.",
		en: "Your git identity is not set. Run `git config --global user.name \"Your Name\"` and " +
			"`git config --global user.email \"you@example.com\"`.",
	},
	MsgGitKindHookFailed: {
		tr: "Bir git hook'u işlemi reddetti. Hook'un çıktısındaki sorunu düzeltip yeniden dene.",
		en: "A git hook rejected the operation. Fix what its output reports and try again.",
	},
	MsgGitKindAuthFailed: {
		tr: "Uzak repo kimliğini kabul etmedi. Token'ını ya da SSH anahtarını kontrol et (`gitpal doctor`).",
		en: "The remote did not accept your credentials. Check your token or SSH key (`gitpal doctor`).",
	},
	MsgGitKindRemoteUnreachable: {
		tr: "Uzak repoya ulaşılamadı. İnternet bağlantını, remote adresini ve erişim iznini kontrol et.",
		en: "The remote could not be reached. Check your connection, the remote URL and your access rights.",
	},
	MsgGitKindNoRemote: {
		tr: "Bu repoda böyle bir remote yok. `git remote -v` ile tanımlı remote'ları gör.",
		en: "This repository has no such remote. See the configured ones with `git remote -v`.",
	},
	MsgGitKindRejected: {
		tr: "Uzak repoda senin olmayan yeni commit'ler var. Önce pull yapmalısın.",
		en: "The remote has new commits you do not have yet. Pull first.",
	},
	MsgGitKindStaleLease: {
		tr: "Uzak branch sen son fetch ettiğinden beri değişmiş; başkasının işini ezmemek için zorla push durduruldu. " +
			"Önce fetch yap ve değişiklikleri incele.",
		en: "The remote branch changed since you last fetched, so the force push was stopped to avoid overwriting " +
			"someone else's work. Fetch and review the changes first.",
	},
	MsgGitKindRemoteDeclined: {
		tr: "Sunucu push'u reddetti (branch koruması ya da sunucu kuralı). Bir branch açıp pull request ile ilerle.",
		en: "The server declined the push (branch protection or a server rule). Push a branch and open a pull request instead.",
	},
	MsgGitKindNoUpstream: {
		tr: "Bu branch'in bağlı olduğu bir uzak branch (upstream) yok. İlk push'ta upstream ayarlanmalı.",
		en: "This branch has no upstream branch. Set one with the first push.",
	},
	MsgGitKindDiverged: {
		tr: "Yerel ve uzak branch ayrı yönlere gitmiş; ikisinde de diğerinde olmayan commit'ler var. " +
			"Merge ya da rebase ile birleştirmen gerekiyor.",
		en: "Your branch and the remote have diverged: each has commits the other lacks. " +
			"Combine them with a merge or a rebase.",
	},
	MsgGitKindDirtyWorktree: {
		tr: "Kaydedilmemiş değişikliklerin bu işlemle kaybolacaktı, bu yüzden git durdu. Önce commit'le ya da stash'le.",
		en: "Your uncommitted changes would be overwritten, so git stopped. Commit or stash them first.",
	},
	MsgGitKindConflict: {
		tr: "Çakışma var. Çakışan dosyaları çözüp işaretle, sonra devam et ya da işlemden vazgeç.",
		en: "There are conflicts. Resolve and mark the conflicting files, then continue or abort.",
	},
	MsgGitKindOperationInProgress: {
		tr: "Yarım kalmış bir merge, rebase ya da cherry-pick var. Önce onu bitir ya da iptal et.",
		en: "A merge, rebase or cherry-pick is still in progress. Finish or abort it first.",
	},
	MsgGitKindDetachedHead: {
		tr: "Şu an bir branch üzerinde değilsin (detached HEAD). Önce bir branch'e geç ya da yeni branch aç.",
		en: "You are not on a branch (detached HEAD). Switch to a branch or create one first.",
	},
	MsgGitKindUnbornBranch: {
		tr: "Bu branch'te henüz hiç commit yok. Önce ilk commit'i at.",
		en: "This branch has no commits yet. Make the first commit.",
	},
	MsgGitKindBranchExists: {
		tr: "Bu adla bir branch zaten var. Başka bir ad seç ya da var olana geç.",
		en: "A branch with that name already exists. Pick another name or switch to it.",
	},
	MsgGitKindBranchNotMerged: {
		tr: "Bu branch'teki commit'ler başka hiçbir yere birleştirilmemiş; silersen kaybolurlar.",
		en: "This branch has commits that are not merged anywhere; deleting it would lose them.",
	},
	MsgGitKindInvalidName: {
		tr: "git bu adı kabul etmiyor. Boşluk, `..`, `~`, `^`, `:` ve `\\` kullanma.",
		en: "git does not accept this name. Avoid spaces, `..`, `~`, `^`, `:` and `\\`.",
	},
	MsgGitKindPathspecNoMatch: {
		tr: "Verilen yol git'in bildiği hiçbir dosyayla eşleşmiyor. Dosya adını kontrol et.",
		en: "The given path matches no file git knows. Check the file name.",
	},
	MsgGitKindPathIgnored: {
		tr: "Bu dosya `.gitignore` kuralları yüzünden yok sayılıyor, bu yüzden eklenmedi. " +
			"Eklemek istiyorsan önce `.gitignore` içindeki kuralı kaldır.",
		en: "This file is excluded by a `.gitignore` rule, so it was not added. " +
			"Remove the rule from `.gitignore` first if you want to add it.",
	},
	MsgGitKindRefNotFound: {
		tr: "Böyle bir branch, tag ya da commit bulunamadı. Adı kontrol et; uzak branch'ler için önce fetch yap.",
		en: "No such branch, tag or commit. Check the name; for remote branches, fetch first.",
	},
	MsgGitKindIndexLocked: {
		tr: "Başka bir git işlemi bu repoda çalışıyor. Bitmesini bekle; çalışan yoksa `.git/index.lock` dosyasını sil.",
		en: "Another git process is running in this repository. Wait for it; if none is running, delete `.git/index.lock`.",
	},
	MsgGitArgNoPaths: {
		tr: "Hiç dosya seçilmedi.",
		en: "No files were selected.",
	},
	MsgGitArgInvalidPath: {
		tr: "Geçersiz dosya yolu: %q. Yol repo kökünden göreli olmalı ve repo dışına çıkmamalı.",
		en: "Invalid path: %q. Paths must be relative to the repository root and stay inside it.",
	},
	MsgGitArgEmptyMessage: {
		tr: "Commit mesajı boş olamaz.",
		en: "The commit message cannot be empty.",
	},
	MsgGitArgInvalidBranch: {
		tr: "Geçersiz branch adı: %q.",
		en: "Invalid branch name: %q.",
	},
	MsgGitArgInvalidRemote: {
		tr: "Geçersiz remote adı: %q.",
		en: "Invalid remote name: %q.",
	},
	MsgGitArgInvalidRev: {
		tr: "Geçersiz revizyon: %q.",
		en: "Invalid revision: %q.",
	},
}
