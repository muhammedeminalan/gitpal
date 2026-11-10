# gitpal — Claude Code notları

- **Tek kaynak doküman:** `docs/PROJECT_SPEC.md`. Çalışmaya başlamadan önce oku; milestone sırasına uy (Bölüm 13 ve 17).
- **Kararlar ve ilerleme:** `DECISIONS.md`. Her mimari kararı tarih + gerekçeyle ekle. Hangi milestone'da kalındığı dosyanın sonundaki "Milestone durumu" bölümünde.
- **Commit öncesi:** `make check` (lint + race testleri + `internal/core` kapsam eşiği + cross-compile).
- **Kurallar:**
  - `git` yalnızca `internal/core/git` içinden çalıştırılır (forbidigo lint kuralı zorlar). Argümanlar hiçbir zaman shell string'ine eklenmez.
  - Testler `internal/core/git/gittest` ile izole çalışır; gerçek repolara ve global git config'e dokunulmaz.
  - Önce test yazılır, başarısız olduğu görülür, sonra kod yazılır.
  - Kullanıcıya görünen her metin `internal/core/i18n` kataloğundan gelir (TR + EN).
  - Token, API anahtarı ve diff içeriği loga ya da hata mesajına yazılmaz.
- **Ortam:** `~/go/bin` PATH'te değil; gofumpt/wails için `$(go env GOPATH)/bin/...` kullan.
