package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
)

func c(hash string, parents ...string) git.Commit { return git.Commit{Hash: hash, Parents: parents} }

func TestGraphLinear(t *testing.T) {
	assert.Equal(t, []string{"*", "*", "*"}, Rows([]git.Commit{c("c", "b"), c("b", "a"), c("a")}))
}

func TestGraphMerge(t *testing.T) {
	// m merges s (side) into x; both come from a.
	rows := Rows([]git.Commit{
		c("m", "x", "s"),
		c("s", "a"),
		c("x", "a"),
		c("a"),
	})
	assert.Equal(t, []string{
		"*",
		"| *",
		"* |",
		"* /",
	}, rows)
}

func TestGraphTwoBranchTips(t *testing.T) {
	// With --all, two tips that share a base.
	rows := Rows([]git.Commit{c("f", "a"), c("m", "a"), c("a")})
	assert.Equal(t, []string{"*", "| *", "* /"}, rows)
}

func TestGraphEmpty(t *testing.T) {
	assert.Empty(t, Rows(nil))
}
