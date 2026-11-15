// Package guide yeni başlayanlar için gitpal komutlarını örnekleriyle listeler:
// terminaldeki `gitpal guide` ve masaüstündeki "Yardım" sekmesi aynı listeyi gösterir.
//
// Örnekler gerçek komutlardır; cli paketindeki test her birinin gerçekten var olan bir
// komuta ve seçeneklere çözüldüğünü denetler, böylece liste komutlardan kopmaz.
package guide

import (
	"strings"

	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// Entry bir örnek komut ve ne yaptığıdır.
type Entry struct {
	Example string `json:"example"`
	Text    string `json:"text"`
}

// Group başlığı olan bir örnek grubudur.
type Group struct {
	Title   string  `json:"title"`
	Entries []Entry `json:"entries"`
}

type item struct {
	example string
	text    i18n.Key
}

type group struct {
	Title i18n.Key
	Items []item
}

var groups = []group{
	{Title: "guide.group.start", Items: []item{
		{"gitpal doctor", "guide.doctor"},
		{"gitpal identity --name \"Ad Soyad\" --email ad@ornek.com --global", "guide.identity"},
		{"gitpal init", "guide.init"},
		{"gitpal clone https://github.com/kullanici/proje.git", "guide.clone"},
		{"gitpal", "guide.dashboard"},
	}},
	{Title: "guide.group.daily", Items: []item{
		{"gitpal status", "guide.status"},
		{"gitpal diff", "guide.diff"},
		{"gitpal add", "guide.add"},
		{"gitpal add lib/main.dart", "guide.add_file"},
		{"gitpal commit -m \"fix: giriş hatası düzeltildi\"", "guide.commit"},
		{"gitpal commit --ai", "guide.commit_ai"},
		{"gitpal save", "guide.save"},
		{"gitpal push", "guide.push"},
		{"gitpal pull", "guide.pull"},
		{"gitpal log --graph", "guide.log"},
	}},
	{Title: "guide.group.branch", Items: []item{
		{"gitpal branch list", "guide.branch_list"},
		{"gitpal branch new feature/giris", "guide.branch_new"},
		{"gitpal branch switch main", "guide.branch_switch"},
		{"gitpal merge feature/giris", "guide.merge"},
		{"gitpal branch delete feature/giris", "guide.branch_delete"},
	}},
	{Title: "guide.group.undo", Items: []item{
		{"gitpal undo", "guide.undo"},
		{"gitpal discard lib/main.dart", "guide.discard"},
		{"gitpal unstage lib/main.dart", "guide.unstage"},
		{"gitpal stash save", "guide.stash_save"},
		{"gitpal stash pop", "guide.stash_pop"},
		{"gitpal backups list", "guide.backups"},
	}},
	{Title: "guide.group.conflict", Items: []item{
		{"gitpal conflicts", "guide.conflicts"},
		{"gitpal resolve lib/main.dart --theirs", "guide.resolve"},
		{"gitpal continue", "guide.continue"},
		{"gitpal abort", "guide.abort"},
	}},
	{Title: "guide.group.github", Items: []item{
		{"gitpal auth github", "guide.auth_github"},
		{"gitpal pr create", "guide.pr_create"},
		{"gitpal pr list", "guide.pr_list"},
		{"gitpal issue list", "guide.issue_list"},
	}},
	{Title: "guide.group.help", Items: []item{
		{"gitpal guide", "guide.guide"},
		{"gitpal commit --help", "guide.help"},
		{"gitpal discard lib/main.dart --dry-run", "guide.dry_run"},
		{"gitpal push --explain", "guide.explain"},
	}},
}

// For rehberi verilen dilde döndürür.
func For(lang i18n.Lang) []Group {
	out := make([]Group, 0, len(groups))
	for _, g := range groups {
		entries := make([]Entry, 0, len(g.Items))
		for _, it := range g.Items {
			entries = append(entries, Entry{Example: it.example, Text: i18n.T(lang, it.text)})
		}
		out = append(out, Group{Title: i18n.T(lang, g.Title), Entries: entries})
	}
	return out
}

// Args bir örneği "gitpal"den sonraki argümanlarına ayırır; çift tırnak içindeki
// boşluklar bölünmez. Testlerin örnekleri gerçek komutlara karşı denetlemesi içindir.
func Args(example string) []string {
	var (
		args    = []string{}
		cur     strings.Builder
		quoted  bool
		started bool
	)
	for _, r := range example {
		switch {
		case r == '"':
			quoted, started = !quoted, true
		case r == ' ' && !quoted:
			if started {
				args = append(args, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteRune(r)
			started = true
		}
	}
	if started {
		args = append(args, cur.String())
	}
	if len(args) > 0 && args[0] == "gitpal" {
		args = args[1:]
	}
	return args
}
