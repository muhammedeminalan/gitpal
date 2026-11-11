# gitpal — geliştirme komutları. `make` ya da `make help` hedefleri listeler.

GO        ?= go
GOLANGCI  ?= golangci-lint
MODULE    := github.com/muhammedeminalan/gitpal
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE      ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.Date=$(DATE)

# internal/core için asgari test kapsamı (spec Bölüm 12).
COVER_MIN ?= 80
# `make cross` ile derlemesi doğrulanan hedefler (spec kabul kriteri 7).
PLATFORMS ?= darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64

.DEFAULT_GOAL := help

.PHONY: help
help: ## Hedefleri listele
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## bin/gitpal üret
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/gitpal ./cmd/gitpal

# Masaüstü arayüzü (spec §9): Wails + cgo. Linux'ta webkit2gtk-4.1 ve gtk3
# geliştirme paketleri gerekir; macOS ve Windows'ta sistemin web görünümü kullanılır.
GUI_TAGS ?= gui,production$(if $(filter Linux,$(shell uname -s 2>/dev/null)),$(comma)webkit2_41)
comma := ,
# macOS: Wails'in kullandığı UTType için (wails CLI'sı da bunu ekler).
GUI_LDFLAGS ?= $(if $(filter Darwin,$(shell uname -s 2>/dev/null)),-framework UniformTypeIdentifiers)

.PHONY: frontend
frontend: ## Arayüzün web kısmını derle (frontend/dist)
	cd frontend && npm ci && npm run check && npm run build

.PHONY: gui
gui: frontend ## bin/gitpal-gui üret (masaüstü arayüzü)
	CGO_ENABLED=1 CGO_LDFLAGS='$(GUI_LDFLAGS)' $(GO) build -trimpath -tags '$(GUI_TAGS)' -ldflags '$(LDFLAGS)' -o bin/gitpal-gui ./cmd/gitpal-gui

.PHONY: install
install: ## gitpal'ı GOPATH/bin altına kur
	$(GO) install -trimpath -ldflags '$(LDFLAGS)' ./cmd/gitpal

.PHONY: test
test: ## Tüm testler
	$(GO) test -count=1 ./...

.PHONY: test-race
test-race: ## Tüm testler, race dedektörü açık
	$(GO) test -race -count=1 ./...

.PHONY: cover
cover: ## internal/core kapsamını ölç, eşiğin altındaysa başarısız ol
	$(GO) test -count=1 -covermode=atomic -coverpkg=./internal/core/... -coverprofile=coverage.out ./internal/core/...
	@$(GO) tool cover -func=coverage.out | awk -v min=$(COVER_MIN) \
		'/^total:/ { gsub("%", "", $$3); printf "internal/core kapsam: %s%% (eşik: %s%%)\n", $$3, min; if ($$3 + 0 < min + 0) exit 1 }'

.PHONY: vet
vet: ## go vet
	$(GO) vet ./...

.PHONY: lint
lint: ## golangci-lint (gofumpt + goimports biçim kontrolü dahil)
	$(GOLANGCI) run ./...

.PHONY: fmt
fmt: ## Kodu biçimlendir (gofumpt + goimports)
	$(GOLANGCI) fmt ./...

.PHONY: tidy
tidy: ## go.mod / go.sum temizle
	$(GO) mod tidy

.PHONY: cross
cross: ## Tüm hedef platformlar için derleme + vet kontrolü
	@for target in $(PLATFORMS); do \
		os=$${target%/*}; arch=$${target#*/}; \
		echo "==> $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build ./... || exit 1; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) vet ./... || exit 1; \
	done

.PHONY: check
check: lint test-race cover cross ## Commit öncesi tam kontrol

.PHONY: clean
clean: ## Derleme çıktılarını sil
	rm -rf bin dist coverage.out frontend/dist
