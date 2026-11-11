# gitpal

Git'in günlük akışını (`add → commit → push`, branch, pull, merge, rebase…) tek yerden, hızlı ve hatasız yürütmek
için geliştirici aracı. Terminal (CLI + TUI) ve masaüstü (GUI) arayüzleri aynı Go çekirdeğini ve aynı **güvenlik
katmanını** paylaşır: geri dönüşü zor her işlem önce ne yapacağını söyler, gerekirse onay ister, yedek alır ve
`gitpal undo` ile geri alınabilir.

Kısa adı `gp`'dir: `gp save`, `gp undo`.

## Kurulum

Gereksinim: **git 2.30+**.

**macOS / Linux**

```bash
curl -fsSL https://raw.githubusercontent.com/muhammedeminalan/gitpal/main/scripts/install.sh | sh
```

`~/.local/bin` altına `gitpal` ve `gp` kurar. Arşiv, sürümle yayımlanan `checksums.txt` ile doğrulanır; tutmazsa
hiçbir şey kurulmaz. Sürüm ve klasör: `GITPAL_VERSION=1.0.0 GITPAL_PREFIX=/usr/local/bin sh install.sh`.

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/muhammedeminalan/gitpal/main/scripts/install.ps1 | iex
```

`%LOCALAPPDATA%\Programs\gitpal` altına kurar, kullanıcı PATH'ine ekler, `gp` kısa adını oluşturur.

**Debian / Ubuntu / Fedora:** sürüm sayfasındaki `.deb` / `.rpm` paketleri (`gp` bağlantısıyla gelir).

**Kaynaktan**

```bash
make build        # bin/gitpal
make install      # GOPATH/bin altına
```

**Masaüstü arayüzü** ayrı bir programdır (`gitpal-gui`). Sürüm sayfasından indirip `gitpal` ile aynı klasöre koy,
ya da kaynaktan derle (Node 22+ ve cgo gerekir; Linux'ta `libgtk-3-dev` ve `libwebkit2gtk-4.1-dev`):

```bash
make gui          # bin/gitpal-gui
gitpal gui        # bulunduğun repoda pencereyi açar
```

Kurulumdan sonra ortamı kontrol et:

```bash
gitpal doctor
```

## Hızlı başlangıç

```bash
gitpal                      # terminalde interaktif ana ekran (TUI)
gitpal status               # sade durum
gitpal save                 # dosya seç → commit mesajı (AI ya da elle) → push, tek akış
gitpal commit -m "fix: ..." # elle commit
gitpal commit --ai          # AI mesaj önerir, sen seçer/düzenlersin
gitpal undo                 # gitpal'ın yaptığı son işlemi geri al
```

İlk commit kimliği için: `gitpal identity --name "Ad Soyad" --email ad@ornek.com --global`.

AI kullanmak istersen anahtarı keychain'e kaydet (dosyaya yazılmaz):

```bash
gitpal auth ai              # sağlayıcı ayarlardaki ai.provider (anthropic | openai | ollama)
gitpal config set ai.provider openai
```

GitHub için:

```bash
gitpal auth github          # token ya da gh CLI; token keychain'de
gitpal pr create            # branch'i push eder, başlığı commit'lerden (ya da AI ile) önerir
```

Her komut `--help` ile kendi seçeneklerini gösterir. Ortak seçenekler:

| Seçenek | Ne yapar |
|---|---|
| `--dry-run` | Hiçbir şeyi değiştirmeden ne olacağını gösterir |
| `--explain` | Arka planda çalışan git komutlarını gösterir |
| `--json` | Script'ler için JSON çıktı |
| `--yes`, `-y` | "Dikkat" sınıfındaki onayları atlar; **tehlikeli işlemlerde geçmez** |
| `--repo <klasör>` | Başka bir klasördeki repoda çalışır |

Çıkış kodları: `0` başarılı, `1` hata, `2` kullanıcı vazgeçti, `3` güvenlik katmanı reddetti (engellenen işlem ya da
onay gerekiyordu ama terminal yoktu).

## Komutlar

| Alan | Komutlar |
|---|---|
| Günlük | `status`, `diff`, `add`, `unstage`, `discard`, `commit`, `save`, `push`, `pull`, `fetch`, `log` |
| Branch | `branch list / new / switch / delete / clean`, `merge`, `rebase [-i]`, `cherry-pick`, `revert` |
| Yarım işlemler | `conflicts`, `resolve [--ours / --theirs]`, `continue`, `skip`, `abort` |
| Kurtarma | `undo`, `history`, `backups list / restore / prune`, `reflog`, `recover` |
| Diğer | `stash`, `tag`, `blame`, `remote`, `submodule`, `ignore`, `identity`, `init`, `clone`, `workspace` |
| GitHub | `repo`, `pr`, `issue`, `release`, `actions` |
| Araç | `doctor`, `config`, `auth`, `gui`, `completion` |

## Güvenlik modeli

gitpal'ın git'ten farkı, geri alınması zor işlemleri tek bir kapıdan (`internal/core/safety`) geçirmesidir. Arayüz
(CLI, TUI, GUI) bu işlemleri doğrudan çalıştıramaz.

**Risk sınıfları**

| Sınıf | Örnek | Ne olur |
|---|---|---|
| Güvenli | status, commit, push, fetch | Doğrudan çalışır |
| Dikkat | pull, merge, cherry-pick, rebase, birleştirilmiş branch silme, commit düşüren reset | Tek onay (`--yes` geçer) + yedek |
| Tehlikeli | discard, değişiklik silen `reset --hard`, stash drop, birleştirilmemiş branch silme, push edilmiş commit'leri yeniden yazan rebase | Açık onay + otomatik yedek; `--yes` **geçmez** |
| Kritik | force push | Branch adını yazarak onay; korumalı branch'lere hiç izin verilmez |

**Kurallar:** korumalı branch'lere (`main`, `master`, `develop`; ayarlanabilir) force push engellenir; force push her
zaman `--force-with-lease` ile yapılır; korumalı branch'e doğrudan commit'te yeni branch önerilir; commit öncesi sır
(API anahtarı, private key, `.env`), büyük dosya ve `node_modules` / build çıktısı uyarısı verilir; yarım bir
merge / rebase varken başka işlem başlatılmaz; her tehlikeli işlemden önce kuru çalıştırma özeti gösterilir.

**Yedekler:** tehlikeli işlemden önce HEAD `refs/gitpal/backup/...` altına, commit'lenmemiş değişiklikler de çalışma
alanına dokunulmadan ayrı bir ref'e kaydedilir. Bu ref'ler `git branch` / `git tag`'de görünmez, push edilmez, 30 gün
saklanır (`safety.backup_retention_days`). `gitpal backups list / restore / prune`.

**Undo:** her işlem `.git/gitpal/history.jsonl` dosyasına yazılır. `gitpal undo` son işlemi yedeğinden geri alır;
push edilmiş bir şey gibi geri alınamayan durumlarda bunu açıkça söyler ve `revert` önerir.

**Komut güvenliği:** git yalnızca `internal/core/git` içinden, argümanlar ayrı ayrı verilerek çalıştırılır; kullanıcı
girdisi hiçbir zaman bir shell komutuna eklenmez.

## AI ve gizlilik

- AI tamamen isteğe bağlıdır; anahtar yoksa ya da `ai.provider: none` ise her akış elle çalışır.
- İlk kullanımda hangi verinin hangi sağlayıcıya gideceği gösterilir ve onay istenir (`ai.consent_given`).
- Gönderilmeden önce diff temizlenir: bilinen anahtar biçimleri ve yüksek entropili dizeler maskelenir; `.env`,
  private key gibi dosyaların içeriği hiç gönderilmez (yalnızca adı, "içerik gönderilmedi" notuyla). Bu, testle
  doğrulanır: `.env` içeriği ağa giden istek gövdesinde yer almaz.
- `ollama` ile her şey yerel makinede kalır.
- Token, API anahtarı ve diff içeriği hiçbir zaman log dosyasına ya da hata mesajına yazılmaz.

## Klavye kısayolları

**Terminal ana ekranı (`gitpal`)**: `↑/↓` gez · `boşluk` stage · `a` hepsini ekle · `c` commit · `p` push · `l` pull ·
`f` fetch · `r` yenile · `q` çık

**Dosya seçici**: `↑/↓` gez · `boşluk` seç · `a` hepsi · `n` hiçbiri · `enter` onayla · `q` vazgeç

**İnteraktif rebase (`gitpal rebase -i`)**: `K/J` taşı · `p` pick · `r` reword · `s` squash · `f` fixup · `d` drop ·
`enter` başlat · `q` vazgeç

**Masaüstü**: `Ctrl/Cmd+K` komut paleti · `Ctrl/Cmd+Enter` commit · `s` seçili dosyayı stage / unstage

## Ayarlar

`gitpal config` hepsini listeler; `gitpal config set <anahtar> <değer>`, `gitpal config edit`.

- Global: `~/.config/gitpal/config.yaml` (Windows: `%APPDATA%\gitpal\config.yaml`). `GITPAL_CONFIG` ile başka bir dosya gösterilebilir.
- Repo bazlı: repo kökünde `.gitpal.yaml`. Bu dosya klonla birlikte geldiği için **güvenilmez** sayılır:
  commit kurallarını ve pull stratejisini ayarlayabilir, korumayı sıkılaştırabilir; ama korumayı gevşetemez ve
  AI ayarlarına (kodun nereye gönderileceğine) dokunamaz.
- API anahtarları ve token'lar hiçbir zaman dosyaya yazılmaz; işletim sisteminin keychain'inde tutulur.
- Log: `~/.local/state/gitpal/gitpal.log` (Windows: `%LOCALAPPDATA%\gitpal\gitpal.log`).
  5 MB'ı geçince bir önceki `gitpal.log.1` olarak saklanır.

## Bilinen sınırlar

- Masaüstü arayüzünde 3 yönlü çakışma görünümü, interaktif rebase düzenleyici ve GitHub ekranı yok (CLI / TUI'de
  var); dosya izleme yerine pencere öndeyken birkaç saniyede bir yenilenir. Ayrıntı: [DECISIONS.md](DECISIONS.md) A-036.
- GitHub device flow girişi kayıtlı bir OAuth uygulaması gerektirir; şimdilik token ya da `gh` CLI ile giriş yapılır.
- Masaüstü arayüzü macOS'ta `.app` paketi olarak değil, tek dosya olarak dağıtılır.

## Geliştirme

Gereksinimler: Go 1.26+, git 2.30+, golangci-lint v2; arayüz için Node 22+.

| Komut | Ne yapar |
|---|---|
| `make test` | Tüm testler |
| `make lint` | golangci-lint (gofumpt + goimports kontrolü dahil) |
| `make fmt` | Kodu biçimlendirir |
| `make cover` | `internal/core` kapsamını ölçer, %80'in altındaysa başarısız olur |
| `make cross` | macOS / Linux / Windows için derleme kontrolü |
| `make check` | Hepsi birden (commit öncesi) |
| `make gui` | Masaüstü arayüzü |
| `goreleaser release --snapshot --clean` | Sürüm paketlerini yerelde üretir |

Testler gerçek `git` çalıştırır ama yalnızca geçici dizinlerde ve izole bir git ayarıyla; gerçek repolarına ve
`~/.gitconfig` dosyana dokunmaz.

Sürüm: `v` ile başlayan bir tag push'lanınca `.github/workflows/release.yml` taslak bir GitHub sürümü hazırlar
(CLI paketleri + üç platform için `gitpal-gui`).

## Dokümanlar

- [docs/PROJECT_SPEC.md](docs/PROJECT_SPEC.md) — proje spesifikasyonu
- [DECISIONS.md](DECISIONS.md) — mimari kararlar, milestone durumu
