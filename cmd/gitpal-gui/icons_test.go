package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Uygulama ikonları derleme sırasında üretilmiyor, repoda duruyor (DECISIONS A-040).
// Bu test, biri silerse ya da bozuk üretirse paketleme sessizce ikonsuz
// kalmasın diye var: macOS dock, Windows .exe ve Linux pencere ikonu buradan gelir.

const root = "../.."

func TestMacIconHasEverySizeTheDockUses(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(root, "build/darwin/gitpal.icns"))
	require.NoError(t, err)
	require.Greater(t, len(data), 8)
	require.Equal(t, "icns", string(data[:4]), "dosya imzası")
	require.Equal(t, uint32(len(data)), binary.BigEndian.Uint32(data[4:8]), "başlıktaki toplam uzunluk")

	// macOS her türü belirli bir piksel boyutunda bekler; yanlış boyut ikonu bulanıklaştırır.
	want := map[string]int{
		"icp4": 16, "icp5": 32, "ic11": 32, "ic12": 64,
		"ic07": 128, "ic13": 256, "ic08": 256, "ic14": 512, "ic09": 512, "ic10": 1024,
	}
	got := map[string]int{}
	for off := 8; off < len(data); {
		require.LessOrEqual(t, off+8, len(data))
		kind := string(data[off : off+4])
		size := int(binary.BigEndian.Uint32(data[off+4 : off+8]))
		require.Greater(t, size, 8, kind)
		require.LessOrEqual(t, off+size, len(data), kind)
		img, err := png.Decode(bytes.NewReader(data[off+8 : off+size]))
		require.NoError(t, err, kind)
		b := img.Bounds()
		require.Equal(t, b.Dx(), b.Dy(), kind)
		requireDrawn(t, img, kind)
		got[kind] = b.Dx()
		off += size
	}
	assert.Equal(t, want, got)
}

func TestWindowsIconIsLinkedIntoBothArchitectures(t *testing.T) {
	// go build, GOOS/GOARCH'a uyan .syso dosyasını kendiliğinden bağlar.
	for _, arch := range []string{"amd64", "arm64"} {
		info, err := os.Stat("rsrc_windows_" + arch + ".syso")
		require.NoError(t, err, arch)
		assert.Greater(t, info.Size(), int64(1000), arch)
	}
}

func TestLogoFilesTheAppAndReadmeUse(t *testing.T) {
	for _, p := range []string{"docs/logo/gitpal-logo.svg", "frontend/public/gitpal-logo.svg"} {
		data, err := os.ReadFile(filepath.Join(root, p))
		require.NoError(t, err, p)
		assert.Contains(t, string(data), "<svg", p)
	}
	// Linux'ta pencere ikonu olarak gömülür.
	f, err := os.Open(filepath.Join(root, "frontend/public/gitpal-256.png"))
	require.NoError(t, err)
	defer f.Close()
	img, err := png.Decode(f)
	require.NoError(t, err)
	assert.Equal(t, 256, img.Bounds().Dx())
	assert.Equal(t, 256, img.Bounds().Dy())
	requireDrawn(t, img, "gitpal-256.png")
}

// requireDrawn: ikonun ortası opak olmalı. Dışa aktarma sırasında logo
// yüklenemezse boyutu doğru ama içi saydam bir PNG çıkıyor (bir kez oldu).
func requireDrawn(t *testing.T, img image.Image, name string) {
	t.Helper()
	b := img.Bounds()
	_, _, _, a := img.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2).RGBA()
	require.Equal(t, uint32(0xffff), a, "%s: ortası saydam, logo çizilmemiş", name)
}
