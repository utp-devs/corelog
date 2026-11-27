"""Procesa los assets de marca de CoreLog.

Toma el banner y el icono originales, recorta los margenes transparentes y genera
las variantes de tamano que usan el README (docs/assets) y la aplicacion React
(web/public/brand).

Uso (desde la raiz del repo, con el entorno de uv):
    .venv\\Scripts\\python.exe scripts\\images\\process_brand.py

Los masters viven en docs/assets (banner 1200 px, icono 512 px); el script los usa
como fuente, asi que puede regenerar todas las variantes sin la carpeta original.
"""

from __future__ import annotations

import argparse
from pathlib import Path

from PIL import Image

REPO_ROOT = Path(__file__).resolve().parents[2]

MASTER_BANNER = "corelog-banner.png"
MASTER_ICON = "corelog-icon.png"

# (ruta relativa a la raiz, ancho maximo en px)
BANNER_TARGETS = [
    ("docs/assets/corelog-banner.png", 1200),
    ("web/public/brand/corelog-banner-480.png", 480),
    ("web/public/brand/corelog-banner-960.png", 960),
]
ICON_TARGETS = [
    ("docs/assets/corelog-icon.png", 512),
    ("web/public/brand/corelog-icon-64.png", 64),
    ("web/public/brand/corelog-icon-128.png", 128),
    ("web/public/brand/corelog-icon-256.png", 256),
]
# Favicon: mismo icono sin letras, sin margen, en los tamanos que piden navegadores y SO.
FAVICON_TARGETS = [
    ("web/public/brand/favicon-16.png", 16),
    ("web/public/brand/favicon-32.png", 32),
    ("web/public/brand/apple-touch-icon.png", 180),
    ("web/public/brand/android-chrome-192.png", 192),
    ("web/public/brand/android-chrome-512.png", 512),
]
FAVICON_ICO = "web/public/favicon.ico"
FAVICON_ICO_SIZES = [16, 32, 48]


def trim_transparent(img: Image.Image, padding_ratio: float = 0.02, alpha_threshold: int = 16) -> Image.Image:
    """Recorta los margenes con alpha bajo el umbral y deja un pequeno padding."""
    rgba = img.convert("RGBA")
    alpha = rgba.getchannel("A").point(lambda a: 255 if a > alpha_threshold else 0)
    box = alpha.getbbox()
    if box is None:
        raise ValueError("la imagen no tiene pixeles visibles")
    left, top, right, bottom = box
    pad_x = int((right - left) * padding_ratio)
    pad_y = int((bottom - top) * padding_ratio)
    box = (
        max(0, left - pad_x),
        max(0, top - pad_y),
        min(rgba.width, right + pad_x),
        min(rgba.height, bottom + pad_y),
    )
    return rgba.crop(box)


def resize_to_width(img: Image.Image, max_width: int) -> Image.Image:
    """Escala manteniendo proporcion. Nunca amplia imagenes mas pequenas que el objetivo."""
    if img.width <= max_width:
        return img.copy()
    height = round(img.height * max_width / img.width)
    return img.resize((max_width, height), Image.Resampling.LANCZOS)


def resize_to_square(img: Image.Image, side: int, padding_ratio: float = 0.04) -> Image.Image:
    """Ajusta la imagen dentro de un cuadrado transparente de `side` px, centrada.

    `padding_ratio` reserva un margen proporcional en cada borde (0 para favicons).
    """
    inner = round(side * (1 - 2 * padding_ratio))
    scale = inner / max(img.width, img.height)
    resized = img.resize((max(1, round(img.width * scale)), max(1, round(img.height * scale))), Image.Resampling.LANCZOS)
    canvas = Image.new("RGBA", (side, side), (0, 0, 0, 0))
    canvas.paste(resized, ((side - resized.width) // 2, (side - resized.height) // 2), resized)
    return canvas


def save_png(img: Image.Image, relative_path: str) -> Path:
    target = REPO_ROOT / relative_path
    target.parent.mkdir(parents=True, exist_ok=True)
    img.save(target, format="PNG", optimize=True)
    return target


def save_ico(img: Image.Image, relative_path: str, sizes: list[int]) -> Path:
    """Genera un .ico multi-tamano (cada tamano se redibuja desde el master para mantener nitidez)."""
    target = REPO_ROOT / relative_path
    target.parent.mkdir(parents=True, exist_ok=True)
    frames = [resize_to_square(img, side, padding_ratio=0.0) for side in sizes]
    frames[-1].save(target, format="ICO", sizes=[(s, s) for s in sizes], append_images=frames[:-1])
    return target


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument(
        "--src",
        type=Path,
        default=REPO_ROOT / "docs" / "assets",
        help="carpeta con los masters: corelog-banner.png (1200 px) y corelog-icon.png (512 px)",
    )
    args = parser.parse_args()

    banner = trim_transparent(Image.open(args.src / MASTER_BANNER))
    icon = trim_transparent(Image.open(args.src / MASTER_ICON))

    for relative_path, width in BANNER_TARGETS:
        out = save_png(resize_to_width(banner, width), relative_path)
        print(f"banner {width}px -> {out.relative_to(REPO_ROOT)}")

    for relative_path, side in ICON_TARGETS:
        out = save_png(resize_to_square(icon, side), relative_path)
        print(f"icono {side}px -> {out.relative_to(REPO_ROOT)}")

    for relative_path, side in FAVICON_TARGETS:
        out = save_png(resize_to_square(icon, side, padding_ratio=0.0), relative_path)
        print(f"favicon {side}px -> {out.relative_to(REPO_ROOT)}")

    out = save_ico(icon, FAVICON_ICO, FAVICON_ICO_SIZES)
    print(f"favicon multi-tamano -> {out.relative_to(REPO_ROOT)}")


if __name__ == "__main__":
    main()
