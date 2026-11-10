package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in      string
		want    Version
		wantErr bool
	}{
		{in: "git version 2.54.0 (Apple Git-157)\n", want: Version{Major: 2, Minor: 54, Patch: 0}},
		{in: "git version 2.47.1.windows.2", want: Version{Major: 2, Minor: 47, Patch: 1}},
		{in: "git version 2.30.0", want: Version{Major: 2, Minor: 30, Patch: 0}},
		{in: "git version 2.43.0-rc1", want: Version{Major: 2, Minor: 43, Patch: 0}},
		{in: "git version 2.38.GIT", want: Version{Major: 2, Minor: 38, Patch: 0}},
		{in: "git version 2.9", want: Version{Major: 2, Minor: 9, Patch: 0}},
		{in: "git version 1.8.3.1", want: Version{Major: 1, Minor: 8, Patch: 3}},
		{in: "git version x.y", wantErr: true},
		{in: "hub version 2.14.2", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseVersion(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		name string
		v    Version
		min  Version
		want bool
	}{
		{name: "equal", v: Version{2, 30, 0}, min: Version{2, 30, 0}, want: true},
		{name: "newer patch", v: Version{2, 30, 1}, min: Version{2, 30, 0}, want: true},
		{name: "newer minor", v: Version{2, 54, 0}, min: Version{2, 30, 0}, want: true},
		{name: "newer major beats smaller minor", v: Version{3, 0, 0}, min: Version{2, 30, 0}, want: true},
		{name: "older minor", v: Version{2, 29, 9}, min: Version{2, 30, 0}, want: false},
		{name: "older major beats bigger minor", v: Version{1, 99, 0}, min: Version{2, 30, 0}, want: false},
		{name: "older patch", v: Version{2, 30, 0}, min: Version{2, 30, 1}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.v.AtLeast(tt.min))
		})
	}
}

func TestVersionString(t *testing.T) {
	assert.Equal(t, "2.54.0", Version{Major: 2, Minor: 54, Patch: 0}.String())
}
