# gitpal — Claude Code notları

- **Tek kaynak doküman:** `docs/PROJECT_SPEC.md`. Çalışmaya başlamadan önce oku; milestone sırasına uy (Bölüm 13 ve 17).
- **Kararlar ve ilerleme:** `DECISIONS.md`. Her mimari kararı tarih + gerekçeyle ekle. Hangi milestone'da kalındığı dosyanın sonundaki "Milestone durumu" bölümünde.
- **Şu an (2026-11-11):** M0–M8 (V1) bitti ve `main`'de. GUI'de ertelenenler A-036, dağıtım A-037. Sıradaki: ilk `v` tag'i ile sürüm iş akışını denemek, sonra V1.1 / spec §14. Her `main` push'undan sonra CI sonucunu kontrol et. Ayrıntı `DECISIONS.md` sonunda.
- **Commit öncesi:** `make check` (lint + race testleri + `internal/core` kapsam eşiği + cross-compile).
- **Kurallar:**
  - `git` yalnızca `internal/core/git` içinden çalıştırılır (forbidigo lint kuralı zorlar). Argümanlar hiçbir zaman shell string'ine eklenmez.
  - Testler `internal/core/git/gittest` ile izole çalışır; gerçek repolara ve global git config'e dokunulmaz.
  - Önce test yazılır, başarısız olduğu görülür, sonra kod yazılır.
  - Kullanıcıya görünen her metin `internal/core/i18n` kataloğundan gelir (TR + EN).
  - Token, API anahtarı ve diff içeriği loga ya da hata mesajına yazılmaz.
  - Yıkıcı ya da riskli her işlem `internal/core/safety` içindeki `Guard.Do` üzerinden çalışır; arayüz kodu `git.Restore`, `git.Reset`, `git.DeleteBranch`, `git.Push` gibi fonksiyonları doğrudan çağırmaz.
- **Sürüm:** `v*` tag'i → `.github/workflows/release.yml` (goreleaser + GUI). Yerelde `goreleaser release --snapshot --clean`.
- **GUI:** `make gui` (Node + cgo gerekir). `internal/gui` saf Go, Wails yalnızca `gui` etiketli dosyalarda.
- **Ortam:** `~/go/bin` PATH'te değil; gofumpt/wails için `$(go env GOPATH)/bin/...` kullan.
