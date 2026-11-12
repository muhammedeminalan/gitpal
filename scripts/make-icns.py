"""docs/logo altındaki PNG'lerden macOS uygulama ikonu (.icns) üretir.

macOS'taki iconutil'e gerek kalmasın diye (Linux CI'da da çalışsın) dosya
elle yazılır. Biçim basit: "icns" + toplam uzunluk, ardından her boyut için
4 baytlık tür + uzunluk + PNG verisi. Her tür belirli bir piksel boyutu ister;
yanlış boyut dock'ta bulanık ikon demektir.

Kullanım: python3 -I scripts/make-icns.py docs/logo build/darwin/gitpal.icns
"""

import struct
import sys
from pathlib import Path

# (tür, piksel) — @2x türleri (ic11..ic14) Retina ekranlar içindir.
TYPES = [
    ("icp4", 16), ("icp5", 32), ("ic11", 32), ("ic12", 64), ("ic07", 128),
    ("ic13", 256), ("ic08", 256), ("ic14", 512), ("ic09", 512), ("ic10", 1024),
]


def main(src: str, out: str) -> None:
    body = b""
    for kind, px in TYPES:
        png = (Path(src) / f"gitpal-{px}.png").read_bytes()
        if png[:8] != b"\x89PNG\r\n\x1a\n":
            sys.exit(f"gitpal-{px}.png bir PNG değil")
        body += kind.encode() + struct.pack(">I", len(png) + 8) + png
    Path(out).write_bytes(b"icns" + struct.pack(">I", len(body) + 8) + body)


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
