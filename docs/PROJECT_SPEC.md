# gitpal — Proje Spesifikasyonu

> Bu dosya Claude Code için tek kaynak (source of truth) dokümandır. Çalışmaya başlamadan önce baştan sona oku. Belirsiz bir nokta olursa varsayım yapıp ilerle, varsayımı `DECISIONS.md` dosyasına yaz.

**Çalışma adı:** `gitpal` (değiştirilebilir)
**Tür:** Hem terminal (CLI/TUI) hem masaüstü GUI olan, tek çekirdeği paylaşan kişisel geliştirici aracı
**Dil:** Go
**Hedef kullanıcı:** Önce geliştiricinin kendisi (unutkan, hızlı çalışan biri). Sonra git'e yeni başlayanlar.

---

## 1. Amaç

Git'in günlük akışını (`add → commit → push`, branch, pull, merge, rebase vb.) tek bir yerden, hatasız ve hızlı yapmak:

1. Repo yolunu seç, değişiklikleri gör, hangilerinin commit'leneceğini seç.
2. AI değişikliği analiz edip anlamlı bir commit mesajı üretsin.
3. Yanlış push, yanlış rebase, yanlış reset gibi geri dönüşü zor hatalara karşı **güvenlik katmanı** korusun.
4. V1'de git ve GitHub'ın tüm yaygın işlemleri kapsansın. V2'de geliştiricilerin sık kullandığı ya da eksik bulduğu özellikler eklensin.

### Tasarım ilkeleri

- **Güvenli varsayılanlar:** Tehlikeli işlem asla sessizce çalışmaz. Önce ne olacağı gösterilir, onay alınır.
- **Geri alınabilirlik:** Yıkıcı işlemden önce otomatik yedek (backup ref) alınır.
- **Şeffaflık:** Araç arka planda hangi `git` komutunu çalıştırdığını her zaman gösterebilir (`--explain` / GUI'de "Komut" paneli). Kullanıcı git öğrenir.
- **CLI ve GUI aynı çekirdeği kullanır.** İş mantığı yalnızca `internal/core` altında yaşar, arayüzler ince katmandır.
- **Çevrimdışı çalışır:** AI ve GitHub özellikleri yoksa bile temel git akışı çalışmalıdır.
- **Tek binary:** Kurulum için Go/Python/Node gerekmez.

---

## 2. Teknoloji Seçimleri

| Katman | Seçim | Gerekçe |
|---|---|---|
| Dil | Go 1.23+ | Tek statik binary, kolay cross-compile, iyi eşzamanlılık |
| Git erişimi | Sistemdeki `git` binary'sini `os/exec` ile çağır | `go-git` her komutu desteklemez; gerçek git ile birebir davranış garanti olur |
| CLI | `spf13/cobra` + `spf13/viper` | Standart, alt komut ve config desteği |
| TUI | `charmbracelet/bubbletea`, `bubbles`, `lipgloss` | İnteraktif seçim ekranları |
| GUI | **Wails v2** (Go backend + web frontend, Svelte veya React + TypeScript + Tailwind) | Aynı Go çekirdeğini doğrudan çağırır, tek binary üretir |
| GitHub | `google/go-github` + OAuth device flow; alternatif olarak `gh auth token` okuma | Token yönetimi için kullanıcıya iki yol sunulur |
| Gizli bilgi saklama | `zalando/go-keyring` | API anahtarları OS keychain'de tutulur, düz dosyada tutulmaz |
| AI | Sağlayıcı soyutlaması (Anthropic varsayılan, OpenAI uyumlu, Ollama yerel) | Tek sağlayıcıya bağımlılık olmasın |
| Test | `testing`, `testify`, geçici git repo fixture'ları | Gerçek repo üzerinde entegrasyon testi |
| Lint/format | `golangci-lint`, `gofumpt` | |
| Dağıtım | `goreleaser` + GitHub Actions | macOS, Linux, Windows binary |

> Not: Kullanıcı başlangıçta Python düşünüyordu ve Go'ya geçmeyi tercih etti. Python/Flutter alternatifine dönülmeyecek. Karar `DECISIONS.md` içinde yazılı kalsın.

### 2.1 Ortam Kurulumu (Claude Code yapacak)

Geliştirme ortamını **sen kur**, kullanıcıdan manuel kurulum isteme. Önce işletim sistemini algıla (`uname -s`, Windows'ta `$env:OS`), sonra eksik olanları kur. Zaten kuruluysa dokunma.

Gerekli araçlar ve kontrol komutları:

| Araç | Kontrol | Minimum |
|---|---|---|
| Go | `go version` | 1.23 |
| git | `git --version` | 2.30 |
| Node.js + npm (Wails frontend için) | `node -v` | 20 LTS |
| Wails CLI | `wails version` | v2 |
| golangci-lint | `golangci-lint --version` | güncel |
| gofumpt | `gofumpt -version` | güncel |
| goreleaser (M8'de) | `goreleaser --version` | güncel |

Kurulum yöntemleri:

- **macOS:** `brew install go node golangci-lint goreleaser`. Homebrew yoksa kullanıcıya sor, kendin kurma.
- **Linux:** Paket yöneticisi sürümü eskiyse resmi tarball'u kullan: `https://go.dev/dl/` adresinden güncel `linux-amd64` (veya `arm64`) arşivini indir, `/usr/local/go` altına aç, `PATH`'e `/usr/local/go/bin` ve `$HOME/go/bin` ekle. Node için `nvm` veya dağıtımın paket yöneticisi. Wails için ayrıca `libgtk-3-dev` ve `libwebkit2gtk-4.0-dev` (veya 4.1) gerekir.
- **Windows:** `winget install GoLang.Go OpenJS.NodeJS.LTS Git.Git`. Wails için WebView2 runtime gerekir (Windows 11'de hazır gelir).
- **Go araçları (tüm platformlarda):**
  ```
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  go install mvdan.cc/gofumpt@latest
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
  ```
- Kurulumdan sonra `wails doctor` çalıştır ve çıktıdaki eksikleri gider.

Kurallar:
- `sudo` gerektiren her komutu çalıştırmadan önce kullanıcıya göster ve onay al.
- Yeni kurduğun her şeyi `DECISIONS.md` içindeki "Ortam" başlığına sürümüyle yaz.
- `PATH` değişikliğini kalıcı yapmak için kullanıcının shell profiline (`~/.zshrc`, `~/.bashrc`) eklemeden önce onay al.
- Ağ yüzünden kurulum başarısız olursa nedenini ve kullanıcının elle yapacağı adımı açıkça yaz, tahmin yürütüp devam etme.

---

## 3. Proje Yapısı

```
gitpal/
├── cmd/
│   └── gitpal/
│       └── main.go              # CLI giriş noktası
├── internal/
│   ├── core/
│   │   ├── git/                 # git komut sarmalayıcıları (exec)
│   │   │   ├── runner.go        # komut çalıştırma, timeout, stderr parse
│   │   │   ├── status.go
│   │   │   ├── diff.go
│   │   │   ├── commit.go
│   │   │   ├── branch.go
│   │   │   ├── remote.go
│   │   │   ├── rebase.go
│   │   │   ├── stash.go
│   │   │   ├── log.go
│   │   │   └── reflog.go
│   │   ├── safety/              # güvenlik kuralları ve yedekleme
│   │   │   ├── rules.go
│   │   │   ├── backup.go
│   │   │   └── guard.go
│   │   ├── ai/
│   │   │   ├── provider.go      # arayüz
│   │   │   ├── anthropic.go
│   │   │   ├── openai.go
│   │   │   ├── ollama.go
│   │   │   ├── prompt.go
│   │   │   └── redact.go        # sır temizleme
│   │   ├── github/              # PR, issue, repo, auth
│   │   ├── config/              # config yükleme/kaydetme
│   │   └── history/             # araç içi işlem günlüğü (undo için)
│   ├── cli/                     # cobra komutları
│   ├── tui/                     # bubbletea ekranları
│   └── gui/                     # Wails bind'ları (ince katman)
├── frontend/                    # Wails frontend
├── testdata/
├── docs/
├── DECISIONS.md
├── README.md
├── Makefile
├── .goreleaser.yaml
└── go.mod
```

**Kural:** `internal/cli`, `internal/tui` ve `internal/gui` hiçbir zaman doğrudan `git` çalıştırmaz. Sadece `internal/core` fonksiyonlarını çağırır.

---

## 4. V1 Kapsamı — Git İşlemleri

Her işlem core katmanında bir fonksiyon, CLI'da bir komut, GUI'de bir ekran/eylem olarak bulunur.

### 4.1 Repo yönetimi
- Repo seçme (yol girme, klasör seçici, son kullanılanlar listesi)
- Geçerli git reposu olup olmadığını doğrulama
- `init`, `clone` (URL + hedef klasör, ilerleme göstergesi)
- Çalışma alanı durumu: branch, upstream, ahead/behind, temiz/kirli

### 4.2 Değişiklikleri görme ve seçme
- `status` ayrıştırması (`git status --porcelain=v2 -z` kullan)
- Dosya bazında stage/unstage
- **Hunk bazında stage/unstage** (`git apply --cached` ile)
- Satır bazında stage (V1.1, zaman kalırsa)
- Diff görüntüleme (unified ve yan yana), binary dosya algılama
- Untracked dosyaları gösterme, `.gitignore`'a ekleme kısayolu
- Discard (değişikliği atma): **güvenlik katmanından geçer**, öncesinde stash-benzeri yedek alınır

### 4.3 Commit
- Mesaj yazma veya AI ile üretme (bkz. Bölüm 6)
- `commit --amend` (push edilmemişse izin ver, edilmişse uyar)
- Commit şablonu ve Conventional Commits desteği (config'ten aç/kapa)
- İmzalı commit (`-S`) desteği, mevcut git config'e saygı göster
- Boş commit engelleme, büyük dosya uyarısı (varsayılan eşik 5 MB)
- Pre-commit hook'ları çalışsın, hata çıktısı kullanıcıya düzgün gösterilsin

### 4.4 Push / Pull / Fetch
- `fetch` (tüm remote'lar, `--prune`)
- `pull` (varsayılan strateji config'ten: merge / rebase / ff-only)
- `push` (upstream ayarlama dahil), `--force-with-lease` desteği
- Ahead/behind görünürlüğü, divergence tespiti ve çözüm önerisi

### 4.5 Branch
- Listele (yerel/uzak), oluştur, geç (checkout/switch), yeniden adlandır, sil
- Merge edilmiş branch'leri toplu temizleme
- Upstream ayarlama
- Branch adı doğrulama ve öneri (örn. `feat/…`, `fix/…`)

### 4.6 Merge / Rebase / Cherry-pick
- Merge (ff, no-ff, squash)
- Rebase (normal ve interaktif; interaktif için GUI/TUI'de sürükle-bırak benzeri düzenleyici: pick, reword, squash, fixup, drop)
- Cherry-pick (tekli ve aralık)
- **Çakışma çözümü:** çakışan dosya listesi, ours/theirs/manuel seçimi, çözüm sonrası `--continue`, vazgeçme `--abort`
- Devam eden işlem durumunu algılama (`.git/rebase-merge`, `MERGE_HEAD` vb.) ve ana ekranda gösterme

### 4.7 Geçmiş ve inceleme
- `log` (graph görünümü, arama, yazara/tarihe/dosyaya göre filtre)
- Commit detayı, dosya geçmişi, `blame`
- `reflog` görüntüleme ve oradan kurtarma ("kaybolan commit'i geri getir")

### 4.8 Geri alma
- `revert` (commit veya aralık)
- `reset` (soft / mixed / hard) — **hard her zaman güvenlik katmanından geçer**
- `restore` (dosya bazında)
- **Araç içi Undo:** Aracın yaptığı son işlemi geri alma (bkz. Bölüm 7.4)

### 4.9 Stash
- Oluştur (mesajlı, untracked dahil/hariç), listele, uygula, pop, sil, diff göster

### 4.10 Tag
- Oluştur (lightweight / annotated), listele, sil, push

### 4.11 Remote ve submodule
- Remote ekle/sil/yeniden adlandır, URL değiştir
- Submodule durumu gösterme ve güncelleme (temel seviye)

### 4.12 Ek
- `.gitignore` şablonları
- Git config okuma/yazma (user.name, user.email; repo veya global)
- Çoklu repo çalışma alanı (workspace): birden çok repoyu listeleyip toplu `status`/`fetch`/`pull` (basit versiyon)

---

## 5. V1 Kapsamı — GitHub İşlemleri

GitHub özellikleri, token yoksa devre dışı kalır. Çekirdek git akışı etkilenmez.

- **Kimlik doğrulama:** OAuth device flow veya `gh auth token` okuma. Token keychain'de saklanır.
- Repo oluşturma (public/private, açıklama, README, .gitignore, lisans), mevcut klasörü GitHub'a bağlama ve ilk push
- Repo listeleme ve klonlama
- **Pull Request:** oluşturma (başlık/açıklama AI ile üretilebilir), listeleme, görüntüleme, merge (merge/squash/rebase), kapatma, checkout, yorum ekleme
- **Issues:** listeleme, oluşturma, kapatma, etiketleme, commit mesajında `Fixes #123` önerisi
- Release oluşturma (tag'den)
- Actions durumu: son çalışmanın başarılı/başarısız olduğunu gösterme (salt okunur)
- Branch protection ihlalini önceden uyarma (push reddedilmeden önce)
- Rate limit ve ağ hatalarını anlaşılır mesajla gösterme

---

## 6. AI ile Commit Mesajı

### 6.1 Akış
1. Kullanıcı stage'lenmiş (veya seçilmiş) değişiklikleri onaylar.
2. Araç diff'i toplar, **sır temizleme** (redaction) uygular, boyutu sınırlar.
3. AI sağlayıcısına gönderilir, 1–3 mesaj önerisi döner.
4. Kullanıcı seçer, düzenler veya yeniden üretir.
5. Onaylanınca commit atılır.

### 6.2 Prompt gereksinimleri
- Girdi: stage'lenmiş diff, değişen dosya listesi, son 10 commit mesajı (stil öğrenmek için), branch adı, config'teki stil tercihi
- Çıktı: Conventional Commits formatı (`feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:` …) veya kullanıcının seçtiği stil
- Özet satırı en fazla 72 karakter, gövde "ne" değil **"neden"** odaklı
- Dil seçeneği: Türkçe veya İngilizce (config)
- Birden fazla ilgisiz değişiklik varsa **commit'i bölmeyi öner** (hangi dosyaların birlikte gitmesi gerektiğini listele)

### 6.3 Büyük diff yönetimi
- Eşik: yaklaşık 12.000 token. Aşılırsa dosya bazlı özetleme yap (her dosya için kısa özet → birleştir).
- Lock dosyaları (`package-lock.json`, `go.sum`, `pubspec.lock` vb.), minified ve binary dosyalar diff'e tam girmez, sadece "değişti" notu düşülür.
- Kullanıcıya kaç token/karakter gönderildiği gösterilir.

### 6.4 Gizlilik ve güvenlik (zorunlu)
- **İlk kullanımda açık onay:** "Diff'in X sağlayıcısına gönderilecek" uyarısı ve kalıcı onay kutusu.
- `redact.go`: API anahtarı, token, private key, `.env` içeriği, parola benzeri desenleri (regex + entropi kontrolü) gönderilmeden önce `[REDACTED]` ile değiştir.
- `.env`, `*.pem`, `*.key`, `id_rsa*` gibi dosyalar **hiç gönderilmez**; bu dosyalar stage'deyse kullanıcıyı uyar.
- Yerel model (Ollama) seçeneği: hiçbir veri dışarı çıkmaz.
- API anahtarı keychain'de, loglarda asla yazılmaz.

### 6.5 Hata durumları
- Ağ yok / API hatası / zaman aşımı → kullanıcıya bildir, manuel mesaj yazmaya izin ver, işlemi bloklama
- Boş veya anlamsız cevap → yeniden dene (en fazla 2 kez), sonra manuele düş

### 6.6 Sağlayıcı arayüzü
```go
type Provider interface {
    Name() string
    GenerateCommitMessages(ctx context.Context, in CommitInput) ([]string, error)
    GeneratePRDescription(ctx context.Context, in PRInput) (PRText, error)
}
```

---

## 7. Güvenlik Katmanı (Safety Layer)

Bu projenin ana farklılaştırıcısı. `internal/core/safety` altında merkezi olarak uygulanır. **Her yıkıcı veya riskli işlem bu katmandan geçmeden çalışamaz.**

### 7.1 Risk sınıfları

| Sınıf | Örnekler | Davranış |
|---|---|---|
| Güvenli | status, log, diff, fetch, add, commit | Doğrudan çalışır |
| Dikkat | pull (rebase), merge, amend (push edilmemiş) | Ne olacağını özetle, tek tıkla onay |
| Tehlikeli | `reset --hard`, discard, branch silme (birleşmemiş), rebase (push edilmiş commit'ler), stash drop | Etki özeti + **yedek al** + açık onay |
| Çok tehlikeli | `push --force` (korumalı branch), geçmiş yeniden yazma sonrası push | Varsayılan olarak **engelle**; yalnızca `--force-with-lease` ve branch adını yazarak onay |

### 7.2 Kurallar (en az)
1. `main`/`master`/`develop` ve config'teki **korumalı branch'lere** force push engellenir.
2. `push --force` kullanıcı isterse otomatik olarak `--force-with-lease` olarak çalıştırılır; düz `--force` sunulmaz.
3. Başkalarının push ettiği commit'leri içeren (remote'ta olan) commit'ler üzerinde rebase/amend/reset yapılmak istenirse **uyarı ve açıklama** gösterilir.
4. Uncommitted değişiklik varken rebase/checkout/reset başlatılacaksa: önce stash önerilir veya otomatik güvenli stash alınır.
5. Push etmeden önce: ahead/behind kontrol edilir; remote'ta yeni commit varsa önce pull gerektiği söylenir.
6. Yanlış branch uyarısı: korumalı branch'e doğrudan commit atılırken "Yeni branch açılsın mı?" önerisi.
7. Commit öncesi: sır tespiti (API key, private key, `.env`), büyük dosya ve `node_modules`/build çıktısı uyarısı.
8. Detached HEAD durumunda commit atılırken uyarı.
9. Devam eden merge/rebase varken başka işlem başlatılmasını engelle.
10. Her tehlikeli işlemden önce işlemin **kuru çalıştırma (dry-run) özeti**: "Şu 3 commit etkilenecek, şu 5 dosya silinecek".

### 7.3 Otomatik yedekleme
- Tehlikeli işlemden önce `refs/gitpal/backup/<timestamp>-<işlem>` ref'i oluştur (HEAD'i işaret eder).
- Uncommitted değişiklikler için `git stash create` ile commit nesnesi üret ve bir ref altında sakla (çalışma alanını bozmadan).
- Yedekler varsayılan 30 gün saklanır, `gitpal backups list/restore/prune` ile yönetilir.

### 7.4 Undo
- Araç her işlemi `.git/gitpal/history.jsonl` içine yazar: zaman, komut, öncesi/sonrası HEAD, backup ref.
- `gitpal undo` son işlemi, mümkünse backup ref üzerinden geri alır.
- Geri alınamayan işlemlerde (push edilmiş) bunu açıkça söyler ve alternatifi (revert) önerir.

---

## 8. CLI Tasarımı

Binary adı: `gitpal` (kısa alias: `gp`).

```
gitpal                       # interaktif TUI (varsayılan)
gitpal status                # sade durum
gitpal add [yollar...]       # yoksa interaktif dosya/hunk seçimi
gitpal commit                # stage'lenmişi AI ile commit et (interaktif onay)
gitpal commit -m "mesaj"     # manuel
gitpal commit --ai           # AI mesajı üret, onay iste
gitpal save                  # add (seçim) + commit (AI) + push tek akış
gitpal push | pull | fetch
gitpal branch [list|new|switch|delete|clean]
gitpal merge <branch>
gitpal rebase [-i] <branch>
gitpal stash [save|list|apply|pop|drop]
gitpal log [--graph]
gitpal undo                  # son aracı işlemi geri al
gitpal backups [list|restore|prune]
gitpal pr [create|list|view|merge|checkout]
gitpal issue [list|create|close]
gitpal repo [create|clone|list]
gitpal config [get|set|edit]
gitpal doctor                # git sürümü, kimlik, token, AI anahtarı kontrolü
gitpal gui                   # GUI'yi aç
```

**Genel bayraklar:** `--repo <yol>`, `--dry-run`, `--explain` (arka planda çalışan git komutlarını göster), `--yes` (yalnızca "Güvenli" ve "Dikkat" sınıfında onayı atlar, tehlikeli sınıfta asla), `--json` (script için), `--no-color`.

**`gitpal save` akışı (ana kullanım senaryosu):**
1. Repo ve branch durumu özeti
2. Değişen dosyaları listele, seçim al (varsayılan: hepsi)
3. Sır/büyük dosya kontrolü
4. AI commit mesajı öner, düzenlemeye izin ver
5. Commit
6. Push (güvenlik kuralları uygulanır, upstream yoksa ayarla)
7. Özet: ne yapıldı, hangi komutlar çalıştı

Çıkış kodları: `0` başarılı, `1` genel hata, `2` kullanıcı iptal, `3` güvenlik katmanı engelledi.

---

## 9. GUI Tasarımı (Wails)

### Ekranlar
1. **Repo seçici / karşılama:** Son repolar, klasör seç, klonla
2. **Ana çalışma ekranı (3 panel):**
   - Sol: branch'ler, remote'lar, stash'ler, tag'ler
   - Orta: değişen dosyalar (staged / unstaged), hunk seçimi
   - Sağ: diff görüntüleyici
   - Alt: commit kutusu + "AI ile üret" düğmesi + Commit / Commit & Push
3. **Geçmiş:** Graph görünümlü log, commit detayı, dosya geçmişi
4. **Çakışma çözücü:** 3 yönlü görünüm (ours / theirs / sonuç)
5. **Interaktif rebase düzenleyici**
6. **GitHub:** PR ve issue listesi, PR oluşturma formu
7. **Güvenlik onay penceresi:** Risk sınıfı rengi, etki özeti, yedek bilgisi, "Komutu göster" açılır paneli
8. **Ayarlar:** Git kimliği, AI sağlayıcı ve anahtar, korumalı branch'ler, tema, dil
9. **İşlem geçmişi ve Undo paneli**

### GUI gereksinimleri
- Açık/koyu tema, sistem temasını takip et
- Klavye kısayolları (stage: `s`, commit: `Cmd/Ctrl+Enter`, vb.) ve komut paleti (`Cmd/Ctrl+K`)
- Uzun işlemler arka planda, ilerleme göstergeli ve iptal edilebilir
- Dosya sistemi izleme (`fsnotify`) ile durumun otomatik yenilenmesi
- Büyük repolarda sanallaştırılmış listeler (performans)
- Arayüz dili: Türkçe ve İngilizce (i18n dosyaları)

---

## 10. Konfigürasyon

Dosya: `~/.config/gitpal/config.yaml` (Windows'ta `%APPDATA%\gitpal`). Repo bazlı override: `.gitpal.yaml`.

```yaml
language: tr                  # arayüz dili
commit:
  style: conventional         # conventional | free
  message_language: tr        # tr | en
  suggestions: 3
  max_subject_length: 72
ai:
  provider: anthropic         # anthropic | openai | ollama | none
  model: ""                   # boşsa sağlayıcı varsayılanı
  consent_given: false
safety:
  protected_branches: [main, master, develop]
  large_file_mb: 5
  backup_retention_days: 30
  warn_on_secrets: true
pull:
  strategy: ff-only           # merge | rebase | ff-only
github:
  auth: device-flow           # device-flow | gh-cli
ui:
  theme: system
```

API anahtarları ve token'lar config dosyasına **yazılmaz**, keychain'de tutulur.

---

## 11. Hata Yönetimi ve Loglama

- Git'in stderr çıktısı ayrıştırılıp insan diliyle açıklanır (örn. "non-fast-forward" → "Uzak repoda senin olmayan yeni commit'ler var. Önce pull yapmalısın.").
- Yaygın hatalar için eşleştirme tablosu: kimlik doğrulama hatası, çakışma, upstream yok, detached HEAD, kirli çalışma alanı, hook reddi.
- `git` kurulu değilse veya sürümü eskiyse (`< 2.30`) `doctor` ve başlangıçta net uyarı.
- Log seviyeleri: error/warn/info/debug; `~/.local/state/gitpal/gitpal.log`. Loglarda token, API anahtarı ve diff içeriği **asla** yer almaz.
- Panic'ler yakalanır, kullanıcıya "hata raporu" bilgisi gösterilir, repo asla yarım durumda bırakılmaz.

---

## 12. Test Stratejisi

- **Birim testler:** safety kuralları, redaction, status/diff ayrıştırıcıları, prompt oluşturma
- **Entegrasyon testleri:** geçici dizinde gerçek git repoları oluşturup senaryo çalıştır (commit, branch, merge çakışması, rebase, force-push engelleme, backup/undo)
- **Sağlayıcı testleri:** AI ve GitHub için mock HTTP sunucusu; gerçek ağ çağrısı yapan test varsayılan olarak kapalı
- **Güvenlik testleri (zorunlu):**
  - Korumalı branch'e force push engellenir
  - `reset --hard` öncesi yedek oluşur ve `undo` ile geri gelir
  - `.env` içeriği AI'ye gönderilen payload'da bulunmaz
  - API anahtarı log ve çıktıda görünmez
- **CLI golden testleri:** Çıktı snapshot'ları
- **Platform matrisi:** macOS, Linux, Windows (CI)
- Hedef: `internal/core` için en az %80 kapsam

---

## 13. Geliştirme Aşamaları (Milestone)

**M0 — İskelet:** Go modülü, klasör yapısı, Makefile, CI, lint, `doctor` komutu, config yükleme
**M1 — Git çekirdeği:** runner, status, diff, add/restore, commit, log, branch, push/pull/fetch + entegrasyon testleri
**M2 — Güvenlik katmanı:** risk sınıfları, kurallar, backup ref'leri, history.jsonl, `undo`
**M3 — CLI + TUI:** cobra komutları, `save` akışı, interaktif seçim ekranları
**M4 — AI commit mesajı:** sağlayıcı arayüzü, Anthropic + Ollama, redaction, büyük diff yönetimi, onay akışı
**M5 — İleri git:** merge, rebase (interaktif dahil), cherry-pick, stash, tag, çakışma çözümü, reflog kurtarma, hunk stage
**M6 — GitHub:** auth, repo, PR, issue, release, actions durumu
**M7 — GUI:** Wails kabuğu, ana ekran, diff, commit kutusu, güvenlik penceresi, geçmiş, ayarlar
**M8 — Cila ve dağıtım:** i18n, kısayollar, performans, goreleaser, README, kurulum betikleri, dokümantasyon

Her milestone sonunda: çalışan, test edilmiş, commit'lenmiş bir durum bırak. Bir sonraki milestone'a geçmeden önce kısa bir özet yaz.

---

## 14. V2 Backlog (V1 bittikten sonra)

- Satır bazında stage
- AI destekli: PR açıklaması ve changelog üretimi, merge çakışması çözüm önerisi, "bu commit ne yapıyor" açıklaması, hatalı commit'i bulma (bisect yardımcısı)
- Commit'i otomatik bölme önerisi (ilgisiz değişiklikleri ayrı commit'lere ayır)
- Git hook yönetimi (GUI'den ekle/kapat)
- Birden çok hesap/kimlik profili (iş/kişisel) ve repo bazlı otomatik geçiş
- GitLab ve Bitbucket desteği
- Monorepo ve sparse-checkout desteği
- Worktree yönetimi
- Git LFS desteği
- Büyük dosya/sır tarayıcı (geçmişte sızmış anahtar tespiti)
- Branch temizlik asistanı (eski/unutulmuş branch'ler)
- Zamanlanmış otomatik fetch ve bildirim
- Eklenti sistemi
- Mobil/uzak kullanım (gelecek fikir)

---

## 15. Kapsam Dışı (V1)

- Git sunucusu yönetimi
- Kod inceleme (review) araçlarının tam kopyası
- Kendi diff/merge algoritmasını yazmak (git'inkini kullan)
- Bulut senkronizasyonu veya kullanıcı hesabı sistemi

---

## 16. Kabul Kriterleri (V1 "bitti" sayılması için)

1. `gitpal save` ile tek akışta add → AI commit → push tamamlanabiliyor.
2. Korumalı branch'e force push engelleniyor; `reset --hard`, discard, rebase öncesi yedek alınıyor ve `undo` çalışıyor.
3. Bölüm 4'teki tüm git işlemleri CLI'dan, ana olanlar GUI'den yapılabiliyor.
4. Bölüm 5'teki GitHub işlemleri çalışıyor (token yoksa zarifçe devre dışı).
5. AI sağlayıcısı olmadan da tüm temel akış çalışıyor.
6. Sırlar AI'ye gönderilmiyor (testle doğrulanmış).
7. macOS, Linux, Windows için tek binary üretiliyor.
8. README'de kurulum, hızlı başlangıç ve güvenlik modeli anlatılıyor.

---

## 17. Claude Code İçin Çalışma Kuralları

- Önce `DECISIONS.md` oluştur, aldığın her mimari kararı tarih ve gerekçeyle ekle.
- Milestone sırasına uy. Bir milestone bitmeden sonrakine geçme.
- Her özellik için test yaz. Test yazılamayan yerde nedenini `DECISIONS.md`'ye not et.
- `git` komutlarını yalnızca `internal/core/git` içinden çalıştır; kullanıcıdan gelen girdiyi **asla** shell string'ine ekleme, argümanları `exec.Command` ile ayrı ayrı ver (command injection önleme).
- Test sırasında kullanıcının gerçek repolarına veya global git config'ine **dokunma**: her test geçici dizin ve izole `GIT_CONFIG_GLOBAL` kullanır.
- Gizli bilgiyi (token, API anahtarı, diff içeriği) loga veya hata mesajına yazma.
- Kod stili: `gofumpt`, `golangci-lint` temiz olmalı. Hata sarmalama (`fmt.Errorf("...: %w", err)`), `context` kullanımı, küçük ve test edilebilir fonksiyonlar.
- Belirsizlikte soru sormak yerine en güvenli varsayılanı seç ve belgele.
- Her milestone sonunda kısa bir ilerleme özeti ver: ne bitti, ne kaldı, bilinen sorunlar.

**Başlangıç görevi:**
1. Bölüm 2.1'e göre ortamı kontrol et, eksik araçları kur (Go dahil).
2. `go.mod` yoksa oluştur, Bölüm 3'teki klasör yapısını kur.
3. M0'ı tamamla (iskelet, CI, `doctor`, config), ardından M1'e geç.
