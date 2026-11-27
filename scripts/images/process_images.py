"""Convierte las imagenes generadas (carpeta local .images/) a WebP optimizado y las publica en
web/public/images/<grupo>/.

Uso (desde la raiz del repo, con el entorno de uv):
    .venv\\Scripts\\python.exe scripts\\images\\process_images.py            # todas
    .venv\\Scripts\\python.exe scripts\\images\\process_images.py --only banner-

No recorta: la aplicacion ajusta el encuadre en tiempo de ejecucion con object-fit y
object-position, asi el mismo archivo sirve para varias relaciones de aspecto. Solo reduce el
tamano al maximo util de cada grupo (nunca amplia) y convierte a WebP.
"""

from __future__ import annotations

import argparse
from pathlib import Path

from PIL import Image

REPO_ROOT = Path(__file__).resolve().parents[2]
SOURCE = REPO_ROOT / ".images"
TARGET = REPO_ROOT / "web" / "public" / "images"

# (prefijo del archivo, carpeta de destino, ancho maximo en px, calidad WebP)
RULES = [
    ("auth-mobile-bg", "auth", 1024, 78),
    ("auth-", "auth", 1920, 80),
    ("onboarding-", "onboarding", 1200, 80),
    ("template-", "templates", 960, 82),
    ("empty-", "states", 960, 82),
    ("error-", "states", 960, 82),
    ("maintenance", "states", 960, 82),
    ("dashboard-", "dashboard", 1800, 80),
    ("banner-", "banners", 1800, 78),
    ("cover-", "covers", 1200, 78),
    ("module-", "modules", 960, 82),
    ("person-", "people", 512, 85),
]


def rule_for(name: str) -> tuple[str, int, int] | None:
    for prefix, folder, width, quality in RULES:
        if name.startswith(prefix):
            return folder, width, quality
    return None


def process(path: Path) -> str:
    rule = rule_for(path.name)
    if rule is None:
        return f"omitida (sin regla): {path.name}"
    folder, max_width, quality = rule
    with Image.open(path) as source:
        image = source.convert("RGB")
    if image.width > max_width:
        height = round(image.height * max_width / image.width)
        image = image.resize((max_width, height), Image.Resampling.LANCZOS)
    destination = TARGET / folder / f"{path.stem}.webp"
    destination.parent.mkdir(parents=True, exist_ok=True)
    image.save(destination, "WEBP", quality=quality, method=6)
    kilobytes = destination.stat().st_size // 1024
    return f"{path.name} -> {destination.relative_to(REPO_ROOT)} ({image.width}x{image.height}, {kilobytes} KB)"


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--only", default="", help="solo los archivos cuyo nombre empiece por este texto")
    args = parser.parse_args()
    files = sorted(p for p in SOURCE.glob("*.png") if not p.name.startswith("logo") and p.name.startswith(args.only))
    for path in files:
        print(process(path))
    print(f"{len(files)} imagenes procesadas")


if __name__ == "__main__":
    main()
