# gitpal

Git'in günlük akışını (`add → commit → push`, branch, pull, merge, rebase…) tek yerden, hızlı ve hatasız
yürütmek için kişisel geliştirici aracı. Terminal (CLI/TUI) ve masaüstü (GUI) arayüzleri aynı Go çekirdeğini paylaşır.

> **Durum:** geliştirme aşamasında. Tamamlanan milestone'lar ve sıradaki işler için
> [DECISIONS.md](DECISIONS.md) dosyasının sonundaki "Milestone durumu" bölümüne bak.
> Tam README (kurulum, hızlı başlangıç, güvenlik modeli) M8'de yazılacak.

## Şu an çalışan

```bash
make build
```

```bash
./bin/gitpal doctor
```

`doctor` git sürümünü, commit kimliğini, ayar dosyasını, GitHub token'ını ve AI anahtarını kontrol eder.
`--json` script'ler için çıktı verir, `--explain` arka planda çalışan git komutlarını gösterir.

## Geliştirme

Gereksinimler: Go 1.26+, git 2.30+, golangci-lint v2.

| Komut | Ne yapar |
|---|---|
| `make test` | Tüm testler |
| `make lint` | golangci-lint (gofumpt + goimports kontrolü dahil) |
| `make fmt` | Kodu biçimlendirir |
| `make cover` | `internal/core` kapsamını ölçer, %80'in altındaysa başarısız olur |
| `make cross` | macOS / Linux / Windows için derleme kontrolü |
| `make check` | Hepsi birden (commit öncesi) |

Testler gerçek `git` çalıştırır ama yalnızca geçici dizinlerde ve izole bir git ayarıyla; gerçek repolarına ve
`~/.gitconfig` dosyana dokunmaz.

## Ayarlar

- Global: `~/.config/gitpal/config.yaml` (Windows: `%APPDATA%\gitpal\config.yaml`). `GITPAL_CONFIG` ile başka bir dosya gösterilebilir.
- Repo bazlı: repo kökünde `.gitpal.yaml`. Bu dosya klonla birlikte geldiği için **güvenilmez** sayılır:
  commit kurallarını ve pull stratejisini ayarlayabilir, korumayı sıkılaştırabilir; ama korumayı gevşetemez ve
  AI ayarlarına (kodun nereye gönderileceğine) dokunamaz.
- API anahtarları ve token'lar hiçbir zaman dosyaya yazılmaz; işletim sisteminin keychain'inde tutulur.

## Dokümanlar

- [docs/PROJECT_SPEC.md](docs/PROJECT_SPEC.md) — proje spesifikasyonu (tek kaynak doküman)
- [DECISIONS.md](DECISIONS.md) — mimari kararlar, ortam, milestone durumu
