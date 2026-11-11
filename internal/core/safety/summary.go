package safety

import (
	"strings"

	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// Summary is an assessment put into words, for a confirmation screen or a
// dry run. Every interface shows the same text.
type Summary struct {
	Risk     string
	Effects  []string
	Findings []string
	Backups  []string
}

// maxListedItems bounds how many names an effect lists before "…".
const maxListedItems = 5

// Summary describes the assessment in lang.
func (a Assessment) Summary(lang i18n.Lang) Summary {
	s := Summary{Risk: i18n.T(lang, riskMessages[a.Risk])}
	for _, e := range a.Effects {
		s.Effects = append(s.Effects, e.describe(lang))
	}
	for _, f := range a.Findings {
		s.Findings = append(s.Findings, f.Localize(lang))
	}
	if a.BackupCommit != "" {
		s.Backups = append(s.Backups, i18n.T(lang, i18n.MsgBackupCommit, short(a.BackupCommit)))
	}
	if a.BackupWorktree {
		s.Backups = append(s.Backups, i18n.T(lang, i18n.MsgBackupWorktree))
	}
	return s
}

var riskMessages = map[Risk]i18n.Key{
	RiskSafe:      i18n.MsgRiskSafe,
	RiskCaution:   i18n.MsgRiskCaution,
	RiskDangerous: i18n.MsgRiskDangerous,
	RiskCritical:  i18n.MsgRiskCritical,
}

func (e Effect) describe(lang i18n.Lang) string {
	switch e.Kind {
	case EffectFilesDiscarded:
		return i18n.T(lang, i18n.MsgEffectFiles, e.Count, listItems(e.Items))
	case EffectCommitsDropped:
		return i18n.T(lang, i18n.MsgEffectCommits, e.Count)
	case EffectBranchDeleted:
		return i18n.T(lang, i18n.MsgEffectBranch, listItems(e.Items))
	case EffectRemoteCommitsReplaced:
		return i18n.T(lang, i18n.MsgEffectRemote, e.Count)
	case EffectCommitsPushed:
		return i18n.T(lang, i18n.MsgEffectPushed, e.Count)
	case EffectCommitRewritten:
		return i18n.T(lang, i18n.MsgEffectRewritten, listItems(e.Items))
	}
	return string(e.Kind)
}

func listItems(items []string) string {
	if len(items) > maxListedItems {
		return strings.Join(items[:maxListedItems], ", ") + ", …"
	}
	return strings.Join(items, ", ")
}
