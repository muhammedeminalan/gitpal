# gitpal logosu

"Gülen dal": iki göz ve üç commit'li bir dal çizgisinden oluşan gülümseme.
"pal" (İngilizce: dost, kanka) + git. Renkler: gece mavisi zemin (`#1e293b` → `#020617`),
açık gri çizgi (`#e2e8f0`), nane yeşili commit (`#34d399`), koyu zeminde kaybolmasın diye
ince açık kenar (`#475569`).

- `gitpal-logo.svg` asıl kaynaktır. PNG'ler ondan çizilir (16–1024 px).
- Uygulama ikonları (`build/darwin/gitpal.icns`, `cmd/*/rsrc_windows_*.syso`,
  `frontend/public/`) bu PNG'lerden `make icons` ile üretilir. Logoyu değiştirirsen
  önce PNG'leri yeniden çiz, sonra `make icons` çalıştır.
