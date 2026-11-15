package guide_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/guide"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

func TestGuideIsGroupedAndLocalized(t *testing.T) {
	tr := guide.For(i18n.TR)
	en := guide.For(i18n.EN)
	require.NotEmpty(t, tr)
	require.Len(t, en, len(tr))

	seen := map[string]bool{}
	for i, g := range tr {
		assert.NotEmpty(t, g.Title)
		assert.NotEqual(t, g.Title, en[i].Title, "başlık iki dilde aynı: çevrilmemiş")
		require.NotEmpty(t, g.Entries, g.Title)
		require.Len(t, en[i].Entries, len(g.Entries))
		for j, e := range g.Entries {
			assert.True(t, strings.HasPrefix(e.Example, "gitpal"), e.Example)
			assert.False(t, seen[e.Example], "aynı örnek iki kez: %s", e.Example)
			seen[e.Example] = true
			assert.NotEmpty(t, e.Text, e.Example)
			// Örnek komut dilden bağımsızdır; yalnızca açıklama çevrilir.
			assert.Equal(t, e.Example, en[i].Entries[j].Example)
			assert.NotEqual(t, e.Text, en[i].Entries[j].Text, e.Example)
		}
	}
	// Yeni başlayan için en önemli akışlar rehberde olmalı.
	for _, must := range []string{"gitpal save", "gitpal undo", "gitpal status", "gitpal guide"} {
		assert.True(t, seen[must], must)
	}
}

func TestArgsSplitsLikeAShell(t *testing.T) {
	assert.Equal(t, []string{"commit", "-m", "fix: giriş hatası"}, guide.Args(`gitpal commit -m "fix: giriş hatası"`))
	assert.Equal(t, []string{}, guide.Args("gitpal"))
	assert.Equal(t, []string{"identity", "--name", "Ad Soyad"}, guide.Args(`gitpal identity --name "Ad Soyad"`))
}
