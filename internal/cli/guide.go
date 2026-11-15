package cli

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/guide"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// guideColumn, örnek sütununun genişliği. Daha uzun örneğin açıklaması bir alt
// satıra, aynı hizaya yazılır; dar terminalde satırlar kaymasın diye.
const guideColumn = 40

func (a *app) newGuideCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "guide",
		Aliases: []string{"rehber"},
		Short:   i18n.T(a.lang, i18n.MsgGuideShort),
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			groups := guide.For(a.lang)
			if a.flags.json {
				return a.writeJSON(groups)
			}
			w := a.env.Stdout
			st := a.styleFor(w)
			fmt.Fprintln(w, st.dim(i18n.T(a.lang, i18n.MsgGuideIntro)))
			for _, g := range groups {
				fmt.Fprintf(w, "\n%s\n", st.bold(g.Title))
				for _, e := range g.Entries {
					n := utf8.RuneCountInString(e.Example)
					if n > guideColumn {
						fmt.Fprintf(w, "  %s\n  %s  %s\n", st.green(e.Example), strings.Repeat(" ", guideColumn), e.Text)
						continue
					}
					fmt.Fprintf(w, "  %s%s  %s\n", st.green(e.Example), strings.Repeat(" ", guideColumn-n), e.Text)
				}
			}
			return nil
		},
	}
}
