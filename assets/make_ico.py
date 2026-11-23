"""把 AI 生成的吉祥物插画转成标准圆角 app icon 与多尺寸 ICO。"""
from io import BytesIO
from pathlib import Path
import struct

from PIL import Image, ImageDraw

OUT = Path(__file__).resolve().parent
SRC = OUT / "ai_cropped.png"
SIZES = [16, 24, 32, 48, 64, 128, 256]


def render(size: int, src: Image.Image) -> Image.Image:
    # 在 4x 精度上做圆角遮罩，边缘更干净
    hi = size * 4
    base = src.resize((hi, hi), Image.Resampling.LANCZOS).convert("RGBA")
    mask = Image.new("L", (hi, hi), 0)
    ImageDraw.Draw(mask).rounded_rectangle(
        [0, 0, hi - 1, hi - 1], radius=int(hi * 0.18), fill=255
    )
    out = Image.new("RGBA", (hi, hi), (0, 0, 0, 0))
    out.paste(base, (0, 0), mask)
    return out.resize((size, size), Image.Resampling.LANCZOS)


def build_ico(entries, path: Path):
    count = len(entries)
    buf = bytearray(struct.pack("<HHH", 0, 1, count))
    offset = 6 + 16 * count
    for size, png in entries:
        w = 0 if size == 256 else size
        buf += struct.pack("<BBBBHHII", w, w, 0, 0, 1, 32, len(png), offset)
        offset += len(png)
    for _, png in entries:
        buf += png
    path.write_bytes(buf)


def main():
    src = Image.open(SRC).convert("RGBA")
    entries = []
    for s in SIZES:
        b = BytesIO()
        render(s, src).save(b, "PNG", optimize=True)
        entries.append((s, b.getvalue()))
    ico = OUT / "HonestBoy-CLI.ico"
    build_ico(entries, ico)

    # 预览合成到桌面底色，圆角效果即实际观感（透明区不再显示为白）
    prev = render(256, src).resize((768, 768), Image.Resampling.LANCZOS)
    desktop = Image.new("RGBA", (768, 768), (32, 33, 36, 255))
    desktop.alpha_composite(prev)
    desktop.convert("RGB").save(OUT / "preview.png")

    top, bottom = [16, 24, 32, 48, 64], [128, 256]
    sheet = Image.new("RGBA", (1160, 800), (245, 246, 248, 255))
    sd = ImageDraw.Draw(sheet)
    x = 30
    for s in top:
        sheet.alpha_composite(render(s, src).resize((100, 100), Image.Resampling.NEAREST), (x, 30))
        sd.text((x + 38, 142), f"{s}px", fill=(60, 64, 72, 255)); x += 130
    x = 200
    for s in bottom:
        sheet.alpha_composite(render(s, src).resize((300, 300), Image.Resampling.NEAREST), (x, 200))
        sd.text((x + 138, 520), f"{s}px", fill=(60, 64, 72, 255)); x += 340
    sheet.convert("RGB").save(OUT / "sizes.png")

    print(f"ICO: {ico} ({ico.stat().st_size} bytes) | sizes: {SIZES}")


if __name__ == "__main__":
    main()
