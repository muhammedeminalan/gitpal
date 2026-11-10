package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	tests := []struct {
		name                  string
		version, commit, date string
		want                  string
	}{
		{name: "development build", version: "dev", want: "dev"},
		{name: "release build", version: "v0.1.0", commit: "abc1234", date: "2026-11-11T10:00:00Z", want: "v0.1.0 (abc1234, 2026-11-11T10:00:00Z)"},
		{name: "commit only", version: "v0.1.0", commit: "abc1234", want: "v0.1.0 (abc1234)"},
		{name: "placeholder commit is not shown", version: "dev", commit: "none", want: "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, format(tt.version, tt.commit, tt.date))
		})
	}
}
