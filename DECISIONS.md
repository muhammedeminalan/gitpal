# DECISIONS — gitpal

Mimari kararlar, varsayımlar ve ortam kayıtları. Kaynak doküman: [docs/PROJECT_SPEC.md](docs/PROJECT_SPEC.md).
Her karar tarih ve gerekçe taşır. Yeni karar ilgili başlığın sonuna eklenir.

## Ortam

Geliştirme makinesi: macOS 27.2, arm64 (Apple M3). Kurulum: 2026-11-10.

| Araç | Sürüm | Durum |
|---|---|---|
| Go | 1.27.4 | **Yeni kuruldu** — `brew install go` |
| golangci-lint | 2.15.0 | **Yeni kuruldu** — `brew install golangci-lint` |
| gofumpt | 0.12.0 | **Yeni kuruldu** — `go install mvdan.cc/gofumpt@latest` → `~/go/bin` |
| Wails CLI | 2.17.1 | **Yeni kuruldu** — `go install github.com/wailsapp/wails/v2/cmd/wails@latest` → `~/go/bin` |
| git | 2.54.0 (Apple Git-157) | Zaten kuruluydu |
| Node.js / npm | 22.16.0 / 10.9.2 | Zaten kuruluydu |
| Homebrew, gh, make, Xcode CLT | — | Zaten kuruluydu |
| goreleaser | — | Kurulmadı; spec gereği M8'de kurulacak |

- `wails doctor` sonucu: "Your system is ready for Wails development!" Eksik olanlar yalnızca isteğe bağlı `upx` ve `nsis`.
- `~/go/bin` PATH'te **değil** ve shell profiline dokunulmadı (spec 2.1: kalıcı PATH değişikliği onay ister).
  gofumpt/wails'i doğrudan çağırmak için: `export PATH="$HOME/go/bin:$PATH"`.
  Makefile bunlara ihtiyaç duymaz; biçimlendirme `golangci-lint fmt` üzerinden yapılır.
- `sudo` gerektiren hiçbir komut çalıştırılmadı.

## Süreç kararları

### S-001 — Dil Go; Python/Flutter'a dönülmeyecek (2026-11-10)
Kullanıcı başlangıçta Python düşünüyordu, Go'yu tercih etti (spec Bölüm 2 notu). Gerekçe: tek statik binary,
kolay cross-compile. Bu karar yeniden açılmayacak.

### S-002 — Tasarım turu atlandı, spec onaylı tasarım sayıldı (2026-11-10)
`PROJECT_SPEC.md` tasarımın kendisi ve "belirsizlikte soru sorma, en güvenli varsayılanı seç ve belgele" diyor
(Bölüm 17). Bu yüzden ayrı bir soru-cevap/tasarım turu ve ayrı bir plan dokümanı üretilmedi. Milestone'lar
spec Bölüm 13'te tanımlı; kararlar bu dosyada.

### S-003 — Spec repoya kopyalandı (2026-11-10)
`~/Downloads/PROJECT_SPEC.md` → `docs/PROJECT_SPEC.md`. Gerekçe: tek kaynak doküman sürüm kontrolünde olmalı;
sonraki oturumlar aynı metni okumalı. `CLAUDE.md` bu dosyaya ve DECISIONS.md'ye işaret eder.

### S-004 — Commit'ler doğrudan `main` üzerinde (2026-11-11)
Repo yeni ve yerel; uzak repo yok, push yapılmıyor. Her milestone sonunda `main` üzerine commit atılır
(spec Bölüm 13: "çalışan, test edilmiş, commit'lenmiş durum"). Uzak repo eklendiğinde branch + PR akışına geçilebilir.

### S-005 — Test önce (TDD) (2026-11-11)
Her paket için önce test yazılır, başarısız olduğu görülür, sonra kod yazılır. Yapılandırma dosyaları
(Makefile, CI, lint ayarı) ve insan için yazılmış dokümanlar test edilmez.

## Mimari kararlar

### A-001 — Modül yolu `github.com/eminalan/gitpal` (2026-11-10)
**Varsayım.** Global git config'teki GitHub noreply adresinden kullanıcı adı `eminalan` çıkarıldı.
Yanlışsa: `go mod edit -module <yeni>` + import yollarında toplu değiştirme + Makefile `MODULE` + `.golangci.yml`.

### A-002 — golangci-lint v2, Homebrew ile (2026-11-10)
Spec'teki `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` yolu eski v1 modülüdür
(son sürümü Go 1.27 ile uyumsuz) ve upstream `go install` ile kurulumu önermiyor. macOS satırındaki
`brew install golangci-lint` kullanıldı. Ayar dosyası v2 biçiminde (`version: "2"`). CI aynı sürüme sabitlendi.

### A-003 — Spec'teki yapıya eklenen paketler (2026-11-11)
Spec Bölüm 3'teki ağaç korundu. Şunlar eklendi:

| Paket | Neden |
|---|---|
| `internal/core/i18n` | TR/EN mesaj kataloğu. `language: tr` ilk günden config'te; `doctor` çıktısı ve hata açıklamaları (Bölüm 11) iki dilde. Sonradan eklemek tüm metinleri yeniden yazdırırdı. |
| `internal/core/secrets` | Keychain sarmalayıcı (`zalando/go-keyring`). `ai/`, `github/` ve `doctor` ortak kullanır. |
| `internal/core/doctor` | `doctor` kontrollerinin iş mantığı (kural: iş mantığı yalnızca `internal/core`). |
| `internal/core/logging` | Log dosyası kurulumu (Bölüm 11). |
| `internal/core/git/gittest` | Testler için izole geçici repo yardımcıları. |
| `internal/version` | ldflags ile gömülen sürüm bilgisi. |

Henüz boş olan paketler (`safety`, `ai`, `github`, `history`, `tui`, `gui`) yalnızca `doc.go` içerir;
`frontend/` ve `testdata/` `.gitkeep` ile tutulur. `.goreleaser.yaml` M8'de eklenecek (goreleaser o zaman kurulacak,
doğrulanamayan ayar dosyası eklenmedi).

### A-004 — Go alt sınırı fiilen 1.26 (2026-11-11)
Spec "Go 1.23+" diyor. `golang.org/x/sys` ve `golang.org/x/term` güncel sürümleri `go 1.26` istiyor; bu yüzden
`go.mod` `go 1.26.0`. Eski sürümlere sabitlemek güvenlik güncellemelerinden vazgeçmek olurdu ve M3'te gelecek
bubbletea/lipgloss zaten güncel `x/*` ister. Eski Go'su olanlarda `GOTOOLCHAIN=auto` doğru sürümü kendisi indirir.

### A-005 — Repo ayar dosyası güvenilmez girdidir (2026-11-11)
`.gitpal.yaml` klonla birlikte gelir; yani repoyu kim yazdıysa o yazmıştır. Kurallar (`internal/core/config/load.go`):

- Yalnızca şu anahtarları ayarlayabilir: `commit.*`, `pull.strategy`, `safety.*`.
- `safety.*` yalnızca **sıkılaştırabilir**: korumalı branch'ler eklenir, hiç çıkarılmaz; `large_file_mb` düşebilir,
  `backup_retention_days` artabilir, tersi olmaz; `warn_on_secrets` kapatılamaz.
- `ai.*` (özellikle `consent_given` ve `provider`), `language`, `github.*`, `ui.*` **yalnızca global dosyadan** okunur.
  Aksi halde klonlanan bir repo AI onayını kendisi verebilir ya da diff'i başka sağlayıcıya yönlendirebilirdi.
- Bozuk ya da geçersiz repo dosyası tümüyle yok sayılır ve uyarı verilir (gitpal'ı durduramaz, korumayı gevşetemez).
- Global dosyada geçersiz değer **hatadır** (sahibi kullanıcıdır, düzeltebilir). Bilinmeyen anahtar her iki dosyada
  uyarıdır: yanlış yazılmış `protected_branchs` sessizce yok sayılırsa kullanıcı korunduğunu sanır.

M4 notu: AI onayı sağlayıcıya bağlanmalı (onay verilen sağlayıcı değişirse yeniden sorulmalı).

### A-006 — Ayar katmanları ve dosya yerleri (2026-11-11)
Katmanlar: varsayılanlar → global dosya → repo dosyası. Tek tek ayarlar için ortam değişkeni desteği yok (spec istemiyor).
`GITPAL_CONFIG` global dosyanın yerini değiştirir (testler ve taşınabilir kurulum için).
Spec'e uygun olarak macOS'ta da `~/.config/gitpal/config.yaml` kullanılır (`~/Library/Application Support` değil);
`XDG_CONFIG_HOME` / `XDG_STATE_HOME` mutlak yol ise geçerlidir. Windows: `%APPDATA%\gitpal\config.yaml`,
log `%LOCALAPPDATA%\gitpal\gitpal.log`. Viper yalnızca dosya okuma/çözme için kullanılıyor; katman birleştirme
kuralları (A-005) elle yazıldı çünkü viper'ın düz "üsttekini al" birleştirmesi bu kuralları ifade edemiyor.

### A-007 — Tek `git` çalıştırma noktası: `git.Runner` (2026-11-11)
- `os/exec` ile süreç başlatan tek yer `internal/core/git/runner.go`. Başka pakette `exec.Command*` kullanımı
  `forbidigo` lint kuralıyla derleme aşamasında reddedilir (ihlal ekleyip kuralın yakaladığı doğrulandı).
- Argümanlar her zaman ayrı argv öğeleri olarak verilir; shell yok. Düşmanca girdilerle (`$(…)`, `;`, tırnak) test edildi.
- Her komuta `-c color.ui=never -c core.quotepath=false` eklenir (kullanıcının git ayarı çıktıyı bozmasın).
- Zaman aşımı: yerel komutlar 2 dk, ağ komutları 10 dk; bağlam iptalinde süreç öldürülür, boruyu açık tutan yardımcı
  süreçler için 5 sn beklenir (`WaitDelay`).
- stdin verilmezse süreç hiçbir şey okuyamaz (terminalin stdin'i miras alınmaz).

### A-008 — Runner ortam değişkenleri (2026-11-11)
- `LC_MESSAGES=C` + `LANGUAGE=en`: git mesajları İngilizce olsun diye (hata tablosu metne bakıyor). `LC_ALL`'a
  **bilerek dokunulmuyor**: `LC_ALL=C` karakter kodlamasını da değiştirir ve Türkçe karakter basan pre-commit
  hook'larını bozabilir. Güvenlik kararları hiçbir zaman stderr metnine dayanmaz; metin yalnızca açıklama içindir.
- `GIT_TERMINAL_PROMPT=0`: git, gitpal'ın göstermediği bir terminalde parola beklemek yerine hata versin.
  Çağıran taraf komut bazında yeniden açabilir.
- `GIT_EDITOR=true`: beklenmedik bir editör isteği kimsenin göremediği bir editör açmasın.
- Repo-yönlendiren değişkenler (`GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`, … — `git rev-parse --local-env-vars`
  listesi, ayar taşıyanlar hariç) miras alınan ortamdan **silinir**. gitpal bir git hook'undan ya da
  `rebase --exec` içinden çalıştırılırsa bu değişkenler onu başka repoya yönlendirirdi. Testle doğrulandı:
  temizleme kapatılınca komut gerçekten dıştaki repoda çalışıyor.
- `GIT_CONFIG_COUNT` / `GIT_CONFIG_PARAMETERS` korunur (CI'ların ve kullanıcıların bilinçli ayar enjeksiyonu).

### A-009 — Kimlik bilgisi redaksiyonu (2026-11-11)
Hata nesnesine, gözlemciye (`--explain`) ve loga giden her argüman/stderr'de URL içindeki kullanıcı bilgisi
(`https://kullanici:parola@…`, `https://<token>@…`) ve `Authorization` başlıkları `[REDACTED]` yapılır.
`ssh://git@host` gibi parolasız SSH kullanıcı adı sır sayılmaz.

### A-010 — i18n: yan yana TR/EN katalog (2026-11-11)
Her mesaj tek bir Go değerinde iki dilde durur (`message{tr, en}`); test, eksik çeviriyi ve dil-arası biçim
belirteci uyumsuzluğunu yakalar. Çekirdek tipli hata döndürür (`ErrorKind`, `ValidationError`…); kullanıcı metni
`Localize(lang)` ile üretilir. Cobra'nın kendi sabit metinleri ("Usage:", "Available Commands:") şimdilik İngilizce (M8).

### A-011 — `doctor` şiddet seviyeleri (2026-11-11)
Yalnızca commit akışını gerçekten engelleyen şeyler **hata**: git yok/eski, kimlik eksik, global ayar dosyası bozuk.
GitHub token'ı ve AI anahtarı eksikse **uyarı** (spec: bunlar olmadan temel akış çalışmalı). `gh-cli` modunda şimdilik
yalnızca `gh` ikilisinin varlığına bakılıyor; token doğrulaması M6'da, Ollama erişilebilirlik kontrolü M4'te.
`doctor` sırları yalnızca "var mı" diye okur; değer hiçbir çıktıya ulaşmaz (testli).

### A-012 — Keychain adları (2026-11-11)
Servis `gitpal`; hesap adları `github-token` ve `ai-<sağlayıcı>-api-key`. Bu adlar depolama sözleşmesidir
(değişirse kayıtlı anahtarlar kaybolur), testle sabitlendi. Ortam değişkeninden anahtar okuma (ör. `ANTHROPIC_API_KEY`)
şimdilik yok; keychain'siz ortamlar için M4/M6'da değerlendirilecek.

### A-013 — Loglama (2026-11-11)
`log/slog` metin biçimi, dosya `0600`, dizin `0700`. Seviye `GITPAL_LOG_LEVEL` ile (varsayılan info). Loga yalnızca
üstveri gider: komut adı (argümansız), git argümanları (redakte), çıkış kodu, süre, hata türü. Komut çıktısı asla
loglanmaz. Log dosyası açılamazsa komut sessizce logsuz devam eder. Panic yakalanır; yığın izi loga, kullanıcıya
kısa mesaj + log yolu gider. **Bilinen eksik:** log rotasyonu yok (M8).

### A-014 — CLI kabuğu (2026-11-11)
- Dış dünya (`stdin/out/err`, ortam, cwd, runner, keychain, terminal tespiti) `cli.Env` ile enjekte edilir; testler
  gerçek ev dizinine, keychain'e ve git ayarına dokunmadan uçtan uca çalışır.
- Çıkış kodları spec'teki gibi: 0 başarılı, 1 hata, 2 kullanıcı iptali, 3 güvenlik katmanı engeli.
- `--explain` çalışan her git komutunu `$ git …` olarak **stderr**'e yazar; stdout `--json` için temiz kalır.
- Renk yalnızca etkileşimli terminalde; `--no-color`, `--json`, `NO_COLOR`, `TERM=dumb` kapatır.
- Argümansız `gitpal` şimdilik yardım gösterir (TUI M3'te). `--repo`, `--dry-run`, `--yes` bayrakları, onları
  kullanan komutlarla birlikte M3'te eklenecek (işlevsiz bayrak eklenmedi).

### A-015 — Test altyapısı (2026-11-11)
- `gittest`: her test geçici dizinde; `HOME`, `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM`, `GIT_CONFIG_NOSYSTEM` izole;
  geliştiricinin kabuğundaki tüm `GIT_*` değişkenleri filtrelenir; kimlik sabit. İzolasyonun kendisi testli.
- Runner'ın süreç davranışı (argv, ortam, stdin, zaman aşımı, iptal) için sahte `git` olarak test ikilisinin kendisi
  kullanılır (`GITPAL_FAKE_GIT`); `os/exec` mock'lanmaz, her platformda aynı çalışır.

## Bilinen eksikler ve ertelenenler

| Konu | Ne zaman |
|---|---|
| Log rotasyonu | M8 |
| Cobra sabit metinlerinin çevirisi | M8 |
| Windows ve Linux'ta test koşusu | CI ilk push'ta çalışacak; yerelde yalnızca cross-compile + vet doğrulandı |
| `.goreleaser.yaml`, `gp` kısa adı | M8 |

## Milestone durumu

### M0 — İskelet ✅ (2026-11-11)
**Biten:** Go modülü ve klasör yapısı, Makefile, GitHub Actions CI (3 işletim sistemi + lint + kapsam), golangci-lint v2
ayarı, ayar yükleme (`internal/core/config`), `git.Runner`, sürüm/kimlik okuma, keychain sarmalayıcı, loglama, i18n,
`gitpal doctor` (`--json`, `--explain`, `--no-color`).
**Doğrulama:** `go test -race ./...` geçti; lint 0 sorun; `internal/core` kapsamı %93,1; 5 hedef platformda derleme + vet temiz;
`./bin/gitpal doctor` bu makinede çalıştırıldı (3 tamam, 2 uyarı: GitHub token'ı ve AI anahtarı yok).
**Kalan:** yok. **Bilinen sorun:** CI henüz hiç koşmadı (uzak repo yok).
