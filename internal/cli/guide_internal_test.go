package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/guide"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// Rehberdeki her örnek gerçekten çalıştırılabilir olmalı: komut var, seçenekler
// tanınıyor, argüman sayısı tutuyor. Komut değişip rehber eskirse bu test düşer.
func TestEveryGuideExampleIsARealCommand(t *testing.T) {
	a := &app{lang: i18n.TR}
	root := a.newRootCmd()
	for _, g := range guide.For(i18n.TR) {
		for _, e := range g.Entries {
			args := guide.Args(e.Example)
			cmd, rest, err := root.Find(args)
			require.NoError(t, err, e.Example)
			if len(args) > 0 {
				require.NotSame(t, root, cmd, "%s: komut bulunamadı", e.Example)
			}
			require.NoError(t, cmd.ParseFlags(rest), e.Example)
			if cmd.Args != nil {
				require.NoError(t, cmd.Args(cmd, cmd.Flags().Args()), e.Example)
			}
		}
	}
}
