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

### S-006 — M1 ayrı branch'te: `m1-git-core` (2026-11-11)
S-004'ten bilinçli sapma. M1 birkaç ara commit gerektiriyor (ortak temel, sonra modüller); `main` yalnızca bitmiş
milestone'ları taşısın diye iş `m1-git-core` branch'inde yürüyor. `make check` geçince `main`'e fast-forward edilir.

### S-007 — Çalışma temposu ve karar yetkisi (2026-11-11)
Kullanıcının isteği: soru sorma, kararı kendin ver, tek başına bitir. Buna göre:
- Her oturumda bir milestone; her biri `make check` + commit + bu dosyadaki özetle kapanır.
- Teknik belirsizlikte en güvenli varsayılan seçilir ve buraya yazılır (spec Bölüm 17).
- **Yalnızca kullanıcının kararı olanlar:** uzak repo açmak ve push, modül yolu (A-001 varsayım), ücretli API anahtarları,
  shell profili / PATH, `sudo`.
- Milestone sonunda kullanıcı özeti okur ve "devam" der ya da yön verir.

### S-008 — Modüllerin paralel yazımı denendi, kullanıcı durdurdu (2026-11-11)
M1'in 6 modülü birbirinden bağımsız olduğu için ayrı git worktree'lerinde paralel ajanlarla yazdırılmak üzere bir iş akışı
başlatıldı (her modül: önce test, sonra kod, ardından bağımsız bir inceleme). Kullanıcı ~5 dakika sonra tüm işlemleri
durdurdu. O anda ajanlar hâlâ spec'i ve mevcut kodu okuyordu: **hiç kod yazılmamıştı, kaybolan iş yok**; geçici worktree'ler
ve branch'leri silindi. Modül tarifleri `docs/M1_PLAN.md` içinde saklandı; sıradaki kişi sırayla ya da yine paralel yazabilir.

### S-009 — Repo GitHub'da, public (2026-11-11)
Kullanıcı repoyu kendi komutuyla açtı ve push etti: <https://github.com/muhammedeminalan/gitpal> (public).
`main` (M0) ve `m1-git-core` (M1 temeli + notlar) yüklü; varsayılan branch `main`. S-004'teki "uzak repo yok" artık geçerli değil:
- CI, `main`'e her push'ta ve her pull request'te koşar (`.github/workflows/ci.yml`); `m1-git-core`'a düz push CI tetiklemez.
- M1 bitince `main`'e almanın iki yolu var: pull request (CI üç işletim sisteminde koşar — önerilen) ya da yerelde
  `git merge --ff-only` + `git push`.
- Push etmek hâlâ kullanıcının kararıdır (S-007); Claude Code sormadan push etmez.

### S-010 — M1'in kalanı bulut oturumunda, `claude/serene-mccarthy-w86sgq` branch'inde (2026-11-11)
`m1-git-core` ve `fix/windows-cli-tests` bu branch'e alındı; 6 modül burada sırayla yazıldı (S-008'deki paralel deneme
tekrarlanmadı), her modül ayrı commit. `main`'e dokunulmadı. CI yalnızca `main`'e push'ta ve pull request'te koştuğu için
bu branch'te CI koşmadı.

### S-011 — M1 `main`'de; CI üç işletim sisteminde yeşil (2026-11-11)
Kullanıcının isteğiyle M1 `main`'e fast-forward edildi (`e6e724b..419cb40`). Tetiklenen CI koşusu
([run 42986662180](https://github.com/muhammedeminalan/gitpal/actions/runs/42986662180)) tamamen yeşil: lint, kapsam,
Linux, **macOS ve Windows** testleri. M1 için "Windows'ta denenmedi" kaydı böylece kapandı; hook testlerinin Windows'ta
Git for Windows'un `sh`'ıyla çalıştığı varsayımı da doğrulandı. M2 aynı branch'te sürdü (`main` ile aynı noktadan).

## Mimari kararlar

### A-001 — Modül yolu `github.com/muhammedeminalan/gitpal` (2026-11-10, düzeltme 2026-11-11)
İlk varsayım `github.com/eminalan/gitpal` idi (global git config'teki noreply adresinden çıkarılmıştı) ve yanlış çıktı:
repo `github.com/muhammedeminalan/gitpal` adresinde açıldı (S-009). 2026-11-11'de düzeltildi: `go.mod`, tüm import yolları,
Makefile `MODULE` ve `.golangci.yml` `local-prefixes` yeni yolu kullanıyor. Yanlış yol hiçbir sürümde yayımlanmadı.

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

### A-016 — Hata türleri: stderr tablosu yalnızca açıklama içindir (2026-11-11)
`internal/core/git/errors.go` içinde 22 yeni `ErrorKind` (commit'lenecek şey yok, kimlik eksik, hook reddi, kimlik doğrulama,
remote'a ulaşılamadı, remote yok, push reddi, eski lease, sunucu reddi, upstream yok, ayrışma, kirli çalışma alanı, çakışma,
yarım işlem, detached HEAD, commit'siz branch, branch var / birleşmemiş / geçersiz ad, yol eşleşmedi, ref yok, index kilidi).
git'in (İngilizceye sabitlenmiş, A-008) stderr metni sıralı bir parça tablosuyla türe çevrilir; daha özel olan önce gelir
(ör. "stale info" de "[rejected]" içerir; publickey hatası da "could not read from remote repository" der). Tablo gerçek git
çıktılarıyla testli (46 örnek).
**Kural:** tür yalnızca kullanıcıya hangi açıklamanın gösterileceğini seçer. Bir durumu *bilmesi* gereken kod (çakışma var mı,
stage'de bir şey var mı, hook mu reddetti) metne değil reponun durumuna bakar. `KindHookFailed`'in tabloda karşılığı yoktur:
git hook reddinde kendi mesajını basmaz, bu türü hook'un varlığını kontrol eden çağıran atar.

### A-017 — Hata açıklamaları: `git.Error.Localize` ve `ArgError` (2026-11-11)
Spec Bölüm 11. Her tür için `i18n/messages_git.go` içinde TR + EN açıklama var; açıklama ne olduğunu ve ne yapılacağını söyler
("Uzak repoda senin olmayan yeni commit'ler var. Önce pull yapmalısın."). `Localize` açıklamanın altına git'in kendi çıktısını
(redakte, 2000 karakterle sınırlı) ekler: çoğu zaman asıl cevap oradadır (hook'un lint hataları, çakışan dosyalar).
Bilinmeyen türde komut satırı ve çıkış kodu gösterilir. Test, her türün iki dilde de açıklaması olduğunu zorlar.
`ArgError` git hiç çalıştırılmadan reddedilen girdidir (dosya seçilmedi, geçersiz yol / branch / remote / revizyon, boş mesaj);
git hatası sayılmaz (`KindOf` → `KindUnknown`).

### A-018 — Çekirdek komutların ortak kuralları (2026-11-11)
`internal/core/git/args.go` başındaki yorumda da yazılı:
- `dir` her zaman çalışma ağacının köküdür (`TopLevel`); yollar köke göreli ve `/` ayraçlıdır (`status --porcelain` gibi).
- Kullanıcıdan gelen yollar git'e **stdin'den, NUL ayraçlı** gider (`--pathspec-from-file=- --pathspec-file-nul`): Windows'un
  32 KiB komut satırı sınırı ve "yol seçenek sanılır" riski ortadan kalkar. Komut bunu desteklemiyorsa `--` sonrasına konur.
  Her iki durumda `GIT_LITERAL_PATHSPECS=1`: `*.go` ya da `:(exclude)x` genişletilmez, dosya adı olarak okunur.
- Yol kontrolü (`checkPaths`): boş, mutlak, NUL içeren ya da repodan çıkan (`../`) yol git'e hiç ulaşmaz.
- Kullanıcıdan gelen ad ve revizyonlar `checkName`'den geçer (boş, `-` ile başlayan, boşluk/kontrol karakteri içeren reddedilir)
  ve `--end-of-options` sonrasına konur (git 2.24+; alt sınır 2.30). `--force` gibi bir değer asla seçenek olarak okunamaz.
- Makine biçimleri ayrıştırılır (`-z`, `--porcelain`, açık `--format`); insan için yazılmış çıktı ayrıştırılmaz.
- Çıktı kullanıcının git ayarına bağlı olmamalı: her komut, çıktıyı değiştiren ayarı açık bayrakla etkisizleştirir ve bunu
  "düşmanca ayar" testiyle kanıtlar.
- Ağ komutları `networkCmd` ile 10 dk zaman aşımı alır.
- `Runner.Head`: HEAD commit'i; henüz commit yoksa `ok=false` (yeni `init` edilmiş repo her komutta ayrı ele alınır).

### A-019 — M1 modül sınırları ve API tasarımı (2026-11-11)
M1 altı bağımsız modüle bölündü: status, diff, stage + commit, log, branch, remote. Her birinin dışa açık API'si, etkisizleştirmesi
gereken git ayarları ve yazılması gereken testler `docs/M1_PLAN.md` içinde. Öne çıkan tasarım kararları:
- `Push` API'sinde düz `--force` isteyebilmenin **yolu yok** (spec 7.2 kural 2); yalnızca `ForceWithLease` (+ `--force-if-includes`).
  Tek branch açık refspec ile push edilir; `push.default`, `push.followTags` başka bir şey push ettiremez.
- `Pull` stratejisi (`merge` / `rebase` / `ff-only`) açık bayraklarla kullanıcının `pull.rebase` / `pull.ff` ayarını ezer;
  `--no-autostash` ile kullanıcının haberi olmadan stash alınmaz (kirli çalışma alanı M2 güvenlik katmanının işi).
- `Commit` mesajı stdin'den alır (`--file=- --cleanup=whitespace`): editör açılmaz, `#` ile başlayan satırlar kaybolmaz.
  Hook'lar her zaman çalışır (`--no-verify` yok). Stage boşsa commit denenmeden `KindNothingToCommit` döner.
- `Restore` / `DeleteBranch(force)` yalnızca isteneni yapar; onay ve yedek M2'de bu fonksiyonların etrafına sarılır.
- `Remotes()` URL'leri redakte döndürür: gömülü token arayüze ya da loga hiç ulaşmaz.
- `diff` yolları ayrıştırırken `rename from/to`, `---`/`+++` satırlarını tercih eder; yalnızca `diff --git` başlığı kalmışsa
  git'in kendi `apply`'ı gibi ortadan böler. Boş ağaç kimliği sabit yazılmaz (SHA-256 repolar).
- Modüller paralel yazılırsa: her modül yalnızca kendi dosyalarına dokunur, API dışındaki her paket düzeyi ad modül önekiyle başlar.

### A-020 — M1'de bilerek yapılmayanlar (2026-11-11)
- `init` ve `clone` (spec 4.1) M3'e bırakıldı: spec'in M1 listesinde yok; `clone` ilerleme göstergesi için Runner'a akışlı
  (streaming) çıktı desteği gerekiyor ve bunu kullanan komut M3'te geliyor.
- `log --graph` çizimi M3 (TUI). M1'de `Commit.Parents` döner; grafik bundan çizilir.
- Çakışma çözümü, stash, tag, reflog, hunk stage M5.
- **Bilinen sınır:** SSH, parola/host anahtarı sorusunu doğrudan `/dev/tty`'ye sorabilir (`GIT_TERMINAL_PROMPT=0` bunu kapsamaz).
  CLI'da bu istenen davranış; GUI'de (M7) ele alınmalı.

### A-021 — status (2026-11-11)
- `GIT_OPTIONAL_LOCKS=0`: arka planda yenilenen status, kullanıcının çalıştırdığı komutun elinden index kilidini almaz.
- Tanınmayan `# …` başlıkları atlanır (ör. `status.showStash` → `# stash N`); tanınmayan kayıt türü hatadır.
- `branch.upstream` var ama `branch.ab` yoksa upstream silinmiştir (`UpstreamGone`); bunun doğru çalışması için `--ahead-behind`
  her zaman verilir (`status.aheadBehind=false` aynı görüntüyü üretirdi).
- Yarım kalan işlem tek `rev-parse --git-path …` çağrısıyla bulunur. Sıra: rebase / am, merge, cherry-pick, revert, bisect —
  çakışmada duran rebase arkasında cherry-pick durumu da bırakabildiği için rebase önce gelir.

### A-022 — stage ve commit (2026-11-11)
- `Unstage`: commit varsa `restore --staged`, yoksa `rm --cached --force -r` (`--force` şart: stage'den sonra yeniden düzenlenen
  dosyada git düz `rm --cached`'i reddeder; `--cached` ile çalışma alanına dokunulmaz).
- `UnstageAll`: `reset --quiet -- .`. Yolsuz `git reset` devam eden merge'ü de iptal ederdi.
- `.gitignore`'a ekleme: desen köke sabitlenir (`/yol`), `\ * ? [` kaçırılır, sondaki boşluklar tek tek kaçırılır; satır sonu
  içeren ad reddedilir (yazılamaz). Dosyanın CRLF/LF tercihi korunur.
- Stage'de bir şey yokken commit denenmez (`KindNothingToCommit`) — **istisna:** merge sonuçlandırılırken (çakışmayı "bizimki"
  lehine çözmek stage'i boş bırakır ama commit yine gerekir).
- `HasStagedChanges` önce reponun varlığına bakar: repo dışında `git diff --cached --quiet` dosya karşılaştırma aracına döner
  ve çıkış kodu 1 "değişiklik var" diye okunurdu. Bu hata testle yakalandı.
- Hook reddi: commit başarısız + tür bilinmiyor + çalıştırılabilir hook var → `KindHookFailed`. Aynı yardımcı (`hookInstalled`)
  push'ta `pre-push` için de kullanılıyor.
- `commit.template` için ek önlem gerekmedi: mesaj dosyadan (`--file`) gelince git şablonu zaten yok sayıyor (testle doğrulandı).
- Yeni tür: `KindPathIgnored` (yok sayılan dosyayı stage etme).

### A-023 — diff (2026-11-11)
- Hunk'ın nerede bittiğine başlıktaki satır sayıları karar verir, satırın görünüşü değil: dosya içeriği `--- a/x` ya da
  `diff --git …` gibi görünse de doğru ayrıştırılır.
- `DiffStat` `--unified` vermez: `--unified`, `--patch`'i de açıyor ve numstat çıktısının arkasına yama ekleniyordu.
- Commit diff'i `git diff <ebeveyn> <commit>` ile alınır; ebeveyn `rev-parse <commit>^1`, kök commit'te boş ağaç
  (`hash-object -t tree --stdin`, SHA-256 repolarda da doğru). Merge commit'te ilk ebeveyn.
- Tür değişimi (dosya → sembolik bağ) git tarafından silme + ekleme çifti olarak basılır; `ChangeTypeChanged` yalnızca
  `old mode` / `new mode` satırlarında tür bitleri farklıysa çıkar.
- Bilerek kullanıcıya bırakılan ayarlar: `diff.orderFile` (yalnızca dosya sırası), `diff.algorithm`, `diff.ignoreSubmodules`.

### A-024 — log (2026-11-11)
- Biçim: 12 alan, `%x00` ile ayrık, `-z` ile commit sonu da NUL. Alan sayısı 12'nin katı değilse hata (sessiz kayma yok).
- Yazar / mesaj araması **düz metin** (`--fixed-strings`): arama kutusuna `fix(ui)` ya da `[wip]` yazan kişi regex kastetmez.
- Yazar adları commit'te yazıldığı gibi (`%an`, `--no-use-mailmap`); mailmap ayrıca `--author` eşleşmesini de değiştiriyordu.
- `Since` / `Until` committer tarihine göre süzer (git'in davranışı).
- `Follow` yalnızca tek yol verildiğinde geçerli, değilse yok sayılır; `log.follow` ayarı `--no-follow` ile etkisizleşir.
- `ShortHash` uzunluğu kullanıcının `core.abbrev` ayarını izler (görsel tercih, bilerek).

### A-025 — branch (2026-11-11)
- Kısa ad tam ref'ten kesilir (`%(refname:short)` belirsizlikte değişiyor). Remote-tracking branch'in remote'u: adının
  başladığı **en uzun** tanımlı remote adı (remote adında `/` olabilir); remote artık tanımlı değilse ilk yol parçası.
- Takip (tracking) kuralı sabit, `branch.autoSetupMerge`'den bağımsız: remote-tracking branch'ten açılan branch onu izler,
  başka hiçbir başlangıç noktası izlenmez.
- Yalnızca remote'ta olan ada geçiş (`git`'in "guess" davranışı) açıkça yapılır: tam bir remote'ta varsa izleyen yerel branch
  açılır, birden fazlasında varsa `KindAmbiguousRef`. `checkout.guess` / `checkout.defaultRemote` etkisiz.
- `ValidateBranchName` git'in kurallarına ek olarak `HEAD`, `@`, `@{-1}` türü kısaltmaları ve `refs/` ile başlayan adları reddeder.
- Olmayan branch her işlemde aynı şekilde bildirilir (`KindRefNotFound`): önce `show-ref --verify` ile bakılır, çünkü git'in
  mesajı alt komuta göre değişiyor.
- `git switch` ve `git pull`'da `--end-of-options` kullanılmadı (eski sürümlerde bu komutların onu doğru işlediğinden emin
  olunamadı); değerler `checkName`'den geçtiği için `-` ile başlayamaz, `--create` değeri ya da tam ref olarak verilir.
- Yeni türler: `KindBranchCheckedOut`, `KindAmbiguousRef`.

### A-026 — remote: fetch / pull / push (2026-11-11)
- Remote her zaman tanımlı bir **ad**dır, URL değil; tanımlı değilse git çalıştırılmadan `KindNoRemote`.
- `Remotes()` URL'leri `remote get-url` ile alır (`insteadOf` uygulanmış, gerçekten kullanılacak adres) ve redakte eder.
- `Fetch`: `Prune=false` iken budama kullanıcının `fetch.prune` ayarına bırakılır (bilerek ezilmedi).
- `Pull`: detached HEAD'de her zaman reddedilir (kaynak açık verilse de); `Remote` ve `Branch` ya birlikte ya hiç;
  bilinmeyen strateji `ArgInvalidStrategy`. `branch.<ad>.mergeOptions` içindeki `--squash --no-commit` de etkisizleştirilir.
  Çakışma, çıktıdan değil reponun durumundan anlaşılır (çakışan dosya + merge/rebase sürüyor).
- `Push` hedefi: remote verilmediyse `branch.<ad>.pushRemote` → `remote.pushDefault` → upstream'in remote'u → `origin`.
  Uzak branch adı: branch o remote'ta bir upstream izliyorsa onun adı, yoksa yerel ad.
- `--force-with-lease` her zaman `--force-if-includes` ile: uzak branch başkası tarafından ilerletilmişse, **fetch edilmiş olsa
  bile** (ama yerel branch'e alınmamışsa) zorla push reddedilir (`KindStaleLease`). Arka plan fetch'i tek başına izin sayılmaz.
  `ExpectedRemote` yalnızca `ForceWithLease` ile anlamlıdır.
- `PushResult.Before`: git'in bildirdiği eski commit; yerelde varsa tam hash, yoksa git'in kısalttığı haliyle.
- `pre-push` hook reddi `KindHookFailed`.
- Yeni argüman kodu: `ArgInvalidStrategy`.

### A-027 — git paketi: gitpal'ın kendi ref alanı, anlık görüntü, reset (2026-11-11)
- `CreateRef` / `DeleteRef` / `Refs` **yalnızca `refs/gitpal/` altında** çalışır; başka her ref `ArgInvalidRev` ile reddedilir.
  Ref'i doğrudan yazan fonksiyonların bir branch'i ya da tag'i oynatabilmesinin yolu yok. `CreateRef` var olanın üzerine yazmaz.
- `WorktreeSnapshot`: `git stash create` — commit'lenmemiş değişiklikleri (stage dahil) index'e, çalışma alanına ve kullanıcının
  stash listesine dokunmadan bir commit'e yazar. Untracked dosyalar dahil değil: gitpal'ın yaptığı hiçbir işlem onları silmiyor.
  `ApplySnapshot` = `stash apply --index`; uymuyorsa hiçbir şeyi değiştirmeden başarısız olur.
- `Reset(soft|mixed|hard)`: yalnızca isteneni yapar; onay ve yedek güvenlik katmanında.
- `GitPath`: git dizini içindeki bir dosyanın mutlak yolu (linked worktree'de o worktree'nin kendi dizini).

### A-028 — history (2026-11-11)
- `.git/gitpal/history.jsonl`, satır başına bir JSON. Linked worktree'lerin her birinin kendi geçmişi var (HEAD de worktree'ye özel).
- Yazılanlar: zaman, işlem, branch, önceki/sonraki HEAD, yedek ref'leri, hedef, dosya yolları. **Yazılmayanlar:** dosya içeriği,
  commit mesajı, kimlik bilgisi. Başarısız işlemde yalnızca hata **türü** yazılır, hata metni değil.
- Bozuk satır atlanır (yarım kalmış yazma tüm geçmişi götürmez). Her kayıt tek `write` ile eklenir.
- `LastUndoable`: en yeni, başarılı, geri alınmamış ve kendisi `undo` olmayan kayıt.

### A-029 — Güvenlik katmanı: model (2026-11-11)
Çağıran yapmak istediğini bir `Operation` olarak tarif eder. `Guard.Assess` = kuru çalıştırma (risk, bulgular, etkiler, hangi
onay gerekir, hangi yedek alınacak). `Guard.Do` = değerlendir → engelliyse dur → onay al → yedekle → çalıştır → geçmişe yaz.

| Risk | Onay | `--yes` |
|---|---|---|
| Güvenli | yok | — |
| Dikkat | tek evet | yeter |
| Tehlikeli | kullanıcının kendi eveti + yedek | **yetmez** (`ErrConfirmationRequired`) |
| Çok tehlikeli | branch adını yazmak | yetmez |

Kuralların (spec 7.2) karşılığı:
1. Korumalı branch'e zorla push: **her zaman engellenir**, lease ile bile. Yerel adı başka olup korumalı branch'e push eden
   branch de (`branch.<ad>.merge`) aynı kurala girer. Korumalı liste `release/*` gibi desen kabul eder.
2. Düz `--force` yok (A-019); korumasız branch'e zorla push "çok tehlikeli": branch adı yazılır, uzaktaki eski uç yedeklenir.
3. Push edilmiş commit'i amend / reset: uyarı + risk "tehlikeli"ye çıkar.
4. Kirli çalışma alanıyla pull / switch / reset --hard: sormadan **otomatik anlık görüntü** alınır (spec "stash önerilir *veya*
   otomatik güvenli stash" diyor; çalışma alanını bozmayan ikincisi seçildi).
5. Push öncesi: uzak branch ilerideyse (son fetch'e göre) engellenir. Yalnızca branch'in kendi upstream'ine push'ta bakılır.
6. Korumalı branch'e doğrudan commit: uyarı + "yeni branch açılsın mı" önerisi (öneriyi sunmak arayüzün işi, M3).
7. Commit öncesi tarama: eklenen satırlarda kimlik bilgisi (8 bilinen biçim + `password = "…"` kalıbı), sır dosyası adları,
   büyük dosya, bağımlılık / derleme çıktısı (dizin başına tek bulgu). **Bulgu sırrın kendisini asla taşımaz**, yalnızca türünü.
   Silinen satırlar taranmaz. Büyük dosya eşiği için stage'deki kopya yerine çalışma alanındaki dosyanın boyutuna bakılır
   (dosya başına git çağrısından kaçınmak için; stage'den sonra değişen dosyada sapabilir).
8. Detached HEAD'de commit: uyarı.
9. Merge / rebase yarımken amend, reset, pull, switch, zorla push engellenir; merge'ü bitiren commit ve dosya discard serbest.
10. Kuru çalıştırma özeti: `Assessment.Effects` (atılacak dosyalar, branch'ten düşecek commit sayısı, uzakta ezilecek commit sayısı…).

Ek kararlar:
- Yedekler **onaydan sonra** alınır: reddedilen işlem arkasında yedek bırakmaz.
- Onay, gösterilen duruma verilir: kullanıcı cevaplayana kadar HEAD ya da branch değiştiyse işlem `ErrRepositoryChanged` ile durur.
- Branch silme: kaybolacak commit yoksa (HEAD'de ya da bir remote'ta duruyorsa) yedek ref'i de alındığı için silme `-D` ile
  yapılır; git'in kendi "merged" ölçütü yalnızca upstream ve HEAD'e baktığından güvenle silinebilecek bazı branch'leri reddediyordu.
  Kaybolacak commit varsa `Force` olmadan **engellenir**, `Force` ile "tehlikeli" + yedek.
- Çıkış kodu eşlemesi (spec 8) arayüzde yapılacak: `*BlockedError` → 3, `ErrDeclined` → 2.

### A-030 — Yedekler (2026-11-11)
- Commit yedeği `refs/gitpal/backup/<zaman>-<işlem>`, commit'lenmemiş değişikliklerin yedeği
  `refs/gitpal/backup-worktree/<zaman>-<işlem>`. Spec tek önek söylüyordu; ikisini ada bakarak ayırt edebilmek için ikiye bölündü.
  Zaman UTC, milisaniyeli (`20261111T090100000Z`); aynı milisaniyede ikinci yedek bir sonraki boş adı alır.
- Yedek HEAD'i değil **risk altındaki commit'i** tutar: branch silmede silinen ucu, zorla push'ta uzaktaki eski ucu.
- `PruneBackups`: saklama süresinden (varsayılan 30 gün) **eski** olanları siler; tam 30 günlük olan kalır.
- `RestoreBackup`: çalışma alanı yedeği üstüne uygulanır; silinen branch'in yedeği branch'i (adı geçmişten bulunarak) yeniden
  oluşturur; diğer commit yedekleri `reset --hard` olarak `Do`'dan geçer (kendi onayı ve yedeğiyle).

### A-031 — Undo (2026-11-11)
Undo yalnızca repo işlemin bıraktığı yerde duruyorsa ilerler; değilse hiçbir şeyi değiştirmeden nedenini söyler.

| İşlem | Geri alma | Reddedilir |
|---|---|---|
| commit, amend | `reset --soft <önceki>` (değişiklikler stage'de kalır) | push edilmişse; üstüne commit atılmışsa; reponun ilk commit'iyse |
| discard | dosyalar çalışma alanı yedeğinden geri yazılır | dosya sonradan yeniden düzenlenmişse; yedek silinmişse |
| reset --hard | `reset --hard <önceki>` + anlık görüntü geri uygulanır | commit'lenmemiş değişiklik varsa; HEAD ilerlemişse |
| reset soft / mixed | aynı kipte geri | HEAD ilerlemişse |
| branch silme | branch yedekten yeniden oluşturulur | aynı adla yeni branch açılmışsa |
| pull | `reset --hard <önceki>` | commit'lenmemiş değişiklik varsa; HEAD ilerlemişse |
| switch | önceki branch'e dönülür | önceki durum detached HEAD ise |
| push, zorla push | — | her zaman (`revert` önerilir) |

HEAD'i sert biçimde geri taşıyan undo'lar önce kendi yedeğini alır. Undo'nun kendisi geçmişe `undo` olarak yazılır ve
tekrar geri alınmaz; ardışık `undo` çağrıları işlemleri birer birer geriye yürür.

### A-032 — CLI ve TUI (M3, 2026-11-11)
- Komutlar: `status`, `diff`, `add`, `unstage`, `discard`, `ignore`, `commit`, `save`, `push`, `pull`, `fetch`,
  `branch [list|new|switch|delete|rename|clean]`, `log [--graph]`, `undo`, `backups [list|restore|prune]`, `history`,
  `config [list|get|set|path|edit]`, `init`, `clone`, `doctor`. Genel bayraklar: `--repo`, `--dry-run`, `--yes`, `--json`,
  `--explain`, `--no-color`. `merge`, `rebase`, `stash` M5'te; `pr`, `issue`, `repo` M6'da; `gui` M7'de.
- Yıkıcı ya da riskli her komut `runOp` → `safety.Guard` üzerinden çalışır. Soru yalnızca stdin **ve** stdout terminalken ve
  `--json` yokken sorulur; aksi halde onay gerektiren işlem çıkış kodu 3 ile reddedilir. "Tehlikeli" işlemde kısa "e/y"
  yetmez, tam kelime ("evet"/"yes") gerekir; "çok tehlikeli"de branch adı yazılır.
- Uyarılar (sır, büyük dosya) onay sorulmasa da (`--yes`, terminal yok) her zaman stderr'e basılır.
- Çıkış kodları (spec 8): `0` başarılı, `1` hata, `2` kullanıcı vazgeçti (`ErrDeclined`, giriş sonu), `3` güvenlik katmanı
  engelledi (`BlockedError`, `ErrConfirmationRequired`). `--dry-run` engellenecek işlemde de 3 döner (script'ler için).
- Sorular ve güvenlik özeti stderr'e, sonuçlar stdout'a yazılır: `--json` çıktısı her zaman tek, temiz bir JSON belgesidir.
- `push --force` ve `--force-with-lease` aynı şeyi yapar: düz force yoktur (spec 7.2 kural 2). İlk push upstream'i ayarlar.
- `save`: seçilmeyen ama stage'de olan dosyalar stage'den çıkarılır — commit'e giren, seçilendir. Terminal yoksa ya da `--all`
  verilmişse hepsi seçilir (spec "varsayılan: hepsi").
- `branch clean`: korumalı branch'ler toplu temizlikte hiçbir zaman aday değildir; her silme ayrı ayrı güvenlik katmanından geçer.
- `log --graph`: şerit (lane) tabanlı sade çizim (`*`, `|`, `/`); çapraz bağlantı çizgisi yok — her satır şerit sayısı kadar
  geniş kalır. `--grep` / `--author` CLI'da büyük-küçük harf duyarsız.
- `config set` yalnızca genel dosyaya yazar (repo dosyası güvenilmez, A-005 ruhu); yorumlar korunur; geçersiz değer dosyayı
  hiç değiştirmez (önce geçici dosyaya yazılıp `Load` ile doğrulanır).
- `config edit` kullanıcının editörünü (`VISUAL` / `EDITOR`, argümanlar boşluktan bölünür, kabuk yok) başlatır. Bu, git dışında
  süreç başlatan **tek** yerdir; forbidigo istisnası satırında gerekçesiyle işaretli.
- TUI (bubbletea): `gitpal` alt komutsuz ve terminalde çalışınca ana ekran açılır (stage/unstage, hepsini ekle, commit, push,
  pull, fetch, yenile); terminal yoksa yardım gösterilir (script'ler değişmez). Riskli işlemler ekranda da önce değerlendirilir,
  aynı özet gösterilir, aynı onay türü istenir. Dosya seçici `add` ve `save`'de kullanılır.
- Ekranlar `cli.Env` üzerinden enjekte edilir (`Pick`, `Dashboard`, `Editor`, `Interactive`): CLI testleri gerçek terminal
  olmadan soru-cevap akışlarını sınar; TUI modelleri tuş vuruşlarıyla ayrıca test edilir.
- git paketine: `Init`, `Clone` (ilerleme çıktısı satır satır, kimlik bilgisi redakte edilerek akar — `Cmd.Progress`).

### A-033 — AI commit mesajı (M4, 2026-11-11)
- `internal/core/ai`: spec 6.6'daki `Provider` arayüzü; sağlayıcılar Anthropic, OpenAI uyumlu, Ollama.
- **Anthropic resmî Go SDK'sı** (`anthropic-sdk-go/v2`) ile çağrılır (Claude API rehberinin kuralı: dilin resmî SDK'sı varsa
  düz HTTP yazılmaz). Varsayılan model `claude-opus-5-5`. İstekte sunucu taraflı yedek model açık
  (`server-side-fallback-2026-07-01` + `fallbacks: "default"`): güvenlik sınıflandırıcısı reddederse aynı çağrıda başka bir
  model cevaplar; yine de `refusal` gelirse `KindRefused` → elle yazmaya düşülür. SDK'nın kendi yeniden deneme davranışı
  (429 / 5xx için 2 kez) korunur.
- OpenAI uyumlu ve Ollama sağlayıcıları düz HTTP (spec, "tek sağlayıcıya bağımlılık olmasın"). Bu ikisinde **model adı
  zorunlu**: gitpal başka şirketlerin model adlarını tahmin etmez; boşsa `ai.model ayarla` hatası.
- Adres değiştirme: `GITPAL_AI_BASE_URL` (OpenAI uyumlu başka bir servis, uzaktaki Ollama, test sunucusu).
- Anahtar: önce keychain (`gitpal auth ai`, ekranda görünmeden okunur, boru ile de verilebilir), yoksa `ANTHROPIC_API_KEY` /
  `OPENAI_API_KEY` ortam değişkeni (keychain olmayan makineler için). Anahtar hiçbir çıktıda, hata metninde, logda yer almaz;
  `ai.Error` sağlayıcının ham hata metnini taşımaz (istek içeriğini yansıtabilir), yalnızca tür + HTTP durum kodu.
- Gönderilen yük (`ai.Prepare`):
  - sır dosyaları (`.env*`, `*.pem`, `*.key`, `id_rsa*`, keystore…) ve özel anahtar içeren dosyalar **hiç gönderilmez**,
    yalnızca adı geçer; kullanıcıya hangilerinin gönderilmediği söylenir;
  - her satır güvenlik katmanının kalıplarıyla + entropi kontrolüyle temizlenir (`[REDACTED]`). Entropi eşiği 4,2 bit/karakter,
    harf + rakam karışık, ≥ 24 karakter; hex (commit hash'leri, ≤ 4,0) ve harf-yalnız tanımlayıcılar dokunulmaz kalır;
  - lock, minified (`.min.` ya da 1000+ karakterlik satır) ve binary dosyalar yalnızca "değişti" notu;
  - bütçe ~48.000 karakter (~12.000 token). **Spec'ten sapma:** aşılınca dosya başına ayrı model çağrısıyla özetleme yerine her
    dosyaya eşit pay verilip kesilir (tek çağrı: daha ucuz, deterministik, yine her dosya görünür). Gönderilen karakter ve
    yaklaşık token sayısı kullanıcıya gösterilir.
  - Son 10 commit mesajı ve branch adı da aynı temizlikten geçer.
- Cevap JSON istenir (`messages`, `split`); kod bloğu ya da cümle içindeki JSON da okunur, düz metin tek öneri sayılır. Boş /
  kullanılamaz cevapta en fazla 2 kez yeniden sorulur, sonra `KindEmpty` → elle yaz (spec 6.5).
- Onay (spec 6.4): ilk kullanımda sorulur, "evet" `ai.consent_given: true` olarak genel ayar dosyasına yazılır. Terminal yoksa
  onaysız gönderim yok, hata. Ollama için de sorulur (adres uzakta olabilir).
- `commit` `-m`'siz çalışınca: terminal + sağlayıcı + onay + anahtar varsa önce AI önerir (numara seç, `r`/`y` yeniden üret,
  `m` elle yaz), yoksa elle yazdırır. `--ai` açıkça ister; script'te (`--ai`, terminal yok) ilk öneri kullanılır. AI hatası
  commit'i hiçbir zaman engellemez — yalnızca script `--ai` istediyse hata olur.
- PR başlığı / açıklaması için `GeneratePRDescription` aynı yük ve temizlikle hazır (M6 kullanacak).

### A-034 — İleri git (M5, 2026-11-11)
- **İnteraktif rebase editörsüz yürür:** git hem todo listesi hem commit mesajı için bir editör çağırır. gitpal kendi ikilisini
  editör olarak verir (`GIT_SEQUENCE_EDITOR` / `GIT_EDITOR` = `'<gitpal yolu>' __gitpal-sequence-editor|__gitpal-message-editor`);
  `main()` ilk iş olarak `git.RunEditorHelper`'a bakar ve önceden yazılmış dosyayı git'in açtığı dosyanın üzerine kopyalar.
  Editör komut satırında yalnızca gitpal'ın kendi yolu ve sabit bir rol argümanı var — kullanıcı girdisi kabuğa hiç girmez.
  Mesaj kuyruğu git'in sorma sırasına göre dizilir: her `reword` için bir, `squash` içeren her zincir için (zincirin sonunda) bir.
  `commit.cleanup=strip` sabitlenir (squash'ın varsayılan birleşik mesajındaki yorumlar temizlensin diye); **bilinen sınır:**
  reword mesajında `#` ile başlayan satırlar silinir. Desteklenen eylemler: pick, reword, squash, fixup, drop (spec 4.6); `edit`
  ve `exec` bilerek yok. `rebase.abbreviateCommands`, `rebase.autoSquash`, `sequence.editor`, `core.editor` etkisiz.
- Merge kipleri açık bayraklarla (`--ff` / `--no-ff` / `--ff-only` / `--squash`), `--no-autostash`, `--no-edit`;
  `merge.ff` ve `branch.<ad>.mergeOptions` değiştiremez.
- Çakışma: merge / rebase / cherry-pick / revert / stash apply çakışmada durursa `KindConflict` (reponun durumuna bakılarak).
  Çözüm: `Conflict` üç sürümü (`:1:` / `:2:` / `:3:`) okur; `ResolveConflict` bir tarafı alır — o taraf dosyayı sildiyse dosya
  kaldırılır. "ours/theirs" git'in anlamıyla (rebase'de ters).
- `continue` / `skip` güvenlik katmanından geçer ve geçmişe yazılır; kaydın `Before`'u çakışmada duran işlemin başladığı yer
  olduğu için `undo` bütün işlemi geri alır. `abort` "dikkat" sınıfında (çakışma çözümleri kaybolur), geri alınamaz.
- Risk sınıfları: merge ve cherry-pick "dikkat" + HEAD yedeği; rebase "dikkat", push edilmiş commit yeniden yazılıyorsa
  "tehlikeli" (kural 3); revert "güvenli" (yeni commit ekler); stash drop "tehlikeli" + stash commit'i yedeklenir, undo
  `stash store` ile geri koyar. Yarım işlem varken merge / rebase / cherry-pick / revert başlatılamaz (kural 9).
- Cherry-pick: git'teki gibi birden çok revizyon tek yürüyüş oluşturur ("a..b c", c'den ulaşılabilen her şeyi de alır).
- Tag: mesaj verilirse açıklamalı tag; imzalama kullanıcının `tag.gpgSign` ayarına uyar (commit'teki gibi), ama hafif tag
  bu ayarla imzalı tag'e dönüştürülmez (`--no-sign`). Var olan tag'in üzerine yazılmaz.
- Hunk stage: seçilen parçalardan yama kurulur, atlanan parçaların kaydırdığı satır numaraları yeniden hesaplanır,
  `apply --cached --whitespace=nowarn` (geri almada `--reverse`). Yalnızca değişmiş, binary olmayan, taşınmamış dosyalar.
  Çalışma alanından parça atma (discard hunk) yok — tehlikeli işlem olarak ayrıca tasarlanmalı.
- Reflog: `log --walk-reflogs --date=unix`; seçici sıradan (`HEAD@{n}`), zaman `%gd`'den. Kurtarma = yeni branch.
- Blame: `--porcelain` ayrıştırılır. Remote ekle / sil / yeniden adlandır / adres değiştir; adres `-` ile başlayamaz.
  Submodule: durum (`submodule status` satırları) ve `update --init --recursive`. Kimlik: repo ya da genel.
- `.gitignore` şablonları gömülü (go, node, python, flutter, swift, android, dotnet, macos, windows, jetbrains, vscode, env);
  var olan kural tekrar eklenmez.
- Çoklu repo: `workspace.repos` genel ayarda (repo dosyası ayarlayamaz). `workspace pull` her zaman yalnızca ileri sarar —
  toplu işlemde hiçbir şey birleştirilmez ya da yeniden yazılmaz. Bir repodaki hata diğerlerini durdurmaz, sonunda çıkış kodu 1.
- Düzeltme (testle bulundu): diff ayrıştırıcısı adında hem boşluk hem tırnak olan dosyada `---` satırının sonundaki sekmeyi
  kırpmıyordu.

## Bilinen eksikler ve ertelenenler

| Konu | Ne zaman |
|---|---|
| Log rotasyonu | M8 |
| Cobra sabit metinlerinin çevirisi | M8 |
| M2 testlerinin Windows ve macOS'ta koşması | `main`'e alınınca (CI); yerelde yalnızca Linux + 5 platform derleme |
| `history.jsonl` boyut sınırı / rotasyonu | M8 |
| `make cover` bulut konteynerinde düşüyor (`go: no such tool "covdata"`) | Ortam sorunu, `main`'de de aynı; kapsam elle ölçüldü. Mac ve CI'da beklenmiyor |
| `.goreleaser.yaml`, `gp` kısa adı | M8 |
| GUI'de SSH parola / host anahtarı istemi | M7 (A-020) |

## Milestone durumu

### M0 — İskelet ✅ (2026-11-11)
**Biten:** Go modülü ve klasör yapısı, Makefile, GitHub Actions CI (3 işletim sistemi + lint + kapsam), golangci-lint v2
ayarı, ayar yükleme (`internal/core/config`), `git.Runner`, sürüm/kimlik okuma, keychain sarmalayıcı, loglama, i18n,
`gitpal doctor` (`--json`, `--explain`, `--no-color`).
**Doğrulama:** `go test -race ./...` geçti; lint 0 sorun; `internal/core` kapsamı %93,1; 5 hedef platformda derleme + vet temiz;
`./bin/gitpal doctor` bu makinede çalıştırıldı (3 tamam, 2 uyarı: GitHub token'ı ve AI anahtarı yok).
**Kalan:** yok.
**İlk CI koşusu (2026-11-11, `main` @ `e6e724b`, [run 42910831051](https://github.com/muhammedeminalan/gitpal/actions/runs/42910831051)): başarısız.**
lint ✅, kapsam ✅, testler Linux ✅ ve macOS ✅, **Windows ❌**. Windows'ta yalnızca `internal/cli` paketinde 4 test düşüyor:
- `TestDoctorSeesStoredSecretsWithoutPrintingThem`, `TestPanicBecomesAnErrorReport`, `TestLogRecordsCommandsButNotOutput`:
  test yardımcısı `harness.logFile()` (`internal/cli/cli_test.go`) log yolunu `state/gitpal/gitpal.log` diye sabit yazıyor;
  Windows'ta log `%LOCALAPPDATA%\gitpal\gitpal.log` altına gider (A-006). Üründe değil **testte** hata: yardımcı yolu
  `config` paketinden sormalı ya da `LOCALAPPDATA`'yı da izole etmeli.
- `TestColorOnlyGoesToTerminals/terminal`: terminalde renk bekleniyor, Windows'ta renk gelmiyor. **Nedeni incelenmedi**
  (test ortamı mı, renk tespiti mi belli değil).
Bu yüzden `main` şu an GitHub'da kırmızı görünür. Düzeltme `fix/windows-cli-tests` (`b60c051`) ile yazıldı ve M1 branch'ine alındı; `main`'e M1 ile birlikte girecek.

### M1 — Git çekirdeği ✅ (2026-11-11) — `main`'de
6 modül + ortak temel (A-016 … A-026). `main` @ `419cb40`; CI üç işletim sisteminde yeşil (S-011).
`m1-git-core` ve `fix/windows-cli-tests` branch'leri artık gereksiz, silinebilir.
**Hâlâ doğrulanmayan:** git 2.30 – 2.42 (yalnızca 2.43 ve CI'daki güncel sürümlerle koşuldu).

### M2 — Güvenlik katmanı ✅ (2026-11-11) — `claude/serene-mccarthy-w86sgq` branch'inde
**Biten:**
- `internal/core/git`: `refs/gitpal/` ref'leri, çalışma alanı anlık görüntüsü, `Reset`, `GitPath` (A-027)
- `internal/core/history`: `history.jsonl` (A-028)
- `internal/core/safety`: risk sınıfları, 10 kural, sır / büyük dosya taraması, `Guard.Assess` / `Do`, yedekler
  (liste / geri yükle / süresi dolanı sil), `Undo` (A-029 … A-031)
- TR + EN mesajlar: 14 bulgu, 14 ret / undo nedeni

**Doğrulama** (Linux, git 2.43, Go 1.26):
- lint 0 sorun; `go test -race ./...` geçti; `internal/core` kapsamı %92,7; 5 platform derleme + vet temiz
- 10 güvenlik kuralı tek tek bozularak sınandı (mutasyon): hepsi en az bir testi düşürdü — korumalı branch engeli, `--yes`'in
  tehlikeli işlemi onaylayamaması, branch adı yazma, "repo değişti" kontrolü, iki yedek türü, undo'nun "ilerledi" ve
  "push edildi" kontrolleri, push öncesi engel, birleşmemiş branch engeli.

**Doğrulanmayan / bilinen sınırlar:**
- Windows ve macOS'ta M2 testleri koşmadı (CI yalnızca `main` ve PR'da).
- Sır taraması kalıp eşleştirmedir: bilinmeyen biçimdeki bir anahtarı kaçırır, sıradan bir dizgeyi sır sanabilir. Bulgular
  uyarıdır, engel değil.
- Push öncesi "geride" kontrolü son fetch'e dayanır; fetch edilmemiş yeni commit'leri git'in kendisi reddeder (`KindRejected`).
- Kullanıcının göreceği bir şey henüz yok: `gitpal undo` / `gitpal backups` komutları ve onay ekranı M3'te.

**Sıradaki adımlar:**
1. M2'yi `main`'e al (CI Windows / macOS'ta koşar).
2. M3 — CLI + TUI: cobra komutları, `save` akışı, interaktif seçim ekranları. Buradaki her komut `safety.Guard` üzerinden
   çalışacak; `config.SafetyConfig` → `safety.Policy` eşlemesi, onay ekranı (`Approver`), çıkış kodları 2 / 3,
   `--dry-run` = `Assess`, `--explain` = `git.WithObserver`.

### M3 — CLI + TUI ✅ (2026-11-11)
Komutlar, `save` akışı, dosya seçici ve ana ekran (A-032). Doğrulama: lint 0, race testleri geçti, `internal/core` kapsamı
%92,3, 5 platform derleme. Gerçek ikiliyle elle denendi (init → save → discard reddi → log --graph).
**Bilinen sınır:** TUI gerçek bir terminalde elle denenmedi (bu ortamda TTY yok); modeller tuş vuruşu testleriyle sınandı.

### M4 — AI commit mesajı ✅ (2026-11-11)
Sağlayıcılar, temizlik, büyük diff, onay, `commit --ai` / `save --ai`, `gitpal auth ai` (A-033). Zorunlu güvenlik testi
(`.env` içeriği gönderilen yükte ve ağa giden istek gövdesinde yok) geçiyor. Doğrulama: lint 0, race testleri, kapsam %92,4,
5 platform derleme. **Doğrulanmayan:** gerçek bir API anahtarıyla gerçek bir sağlayıcıya istek atılmadı (bu ortamda anahtar yok);
sağlayıcılar sahte HTTP sunucularıyla sınandı. `gitpal save` artık add → AI commit → push akışını tamamlıyor (kabul kriteri 1).

### M5 — İleri git ✅ (2026-11-11)
merge, rebase (interaktif TUI dahil), cherry-pick, revert, continue / skip / abort, çakışma listesi ve çözümü, stash, tag,
reflog + recover, blame, hunk stage (`add -p`, `unstage -p`), remote yönetimi, submodule, kimlik, `.gitignore` şablonları,
workspace (A-034). Doğrulama: lint 0, race testleri, kapsam %91,2, 5 platform derleme.
**Doğrulanmayan:** interaktif rebase'in Windows'ta çalışması (editör komutu Git for Windows'un `sh`'ıyla çalışır varsayımı) —
`main`'e alınınca CI gösterecek. Satır bazında stage (spec "V1.1, zaman kalırsa") yapılmadı.

**Tahmini kalan süre** (tahmin, ölçüm değil):

| Milestone | Süre |
|---|---|
| M6 GitHub | ~3 saat |
| M7 GUI | 6–8 saat (en belirsiz) |
| M8 Cila ve dağıtım | 2–3 saat |

`gitpal save` (add → AI commit → push) M4 sonunda çalışır.
