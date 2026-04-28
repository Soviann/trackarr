"""Render PlexTracker icons from icon.svg primitives via PIL.

Outputs to frontend/public/. Renders at 4x then downsamples for anti-aliasing.
"""
from PIL import Image, ImageDraw
from pathlib import Path

OVERSAMPLE = 4
BASE = 512
SIZE = BASE * OVERSAMPLE

DISC = "#1a1714"
TAN = "#d4ad7a"
AMBER = "#E8A925"
TAN_RGB = (212, 173, 122)
AMBER_RGB = (232, 169, 37)
DISC_RGB = (26, 23, 20)


def s(v: float) -> int:
    return int(round(v * OVERSAMPLE))


def stroke_line(d: ImageDraw.ImageDraw, points, width: int, color):
    """Draw a polyline with rounded caps and joints (mimics SVG stroke-linecap=round)."""
    pts = [(s(x), s(y)) for x, y in points]
    w = s(width)
    d.line(pts, fill=color, width=w, joint="curve")
    r = w // 2
    for x, y in pts:
        d.ellipse((x - r, y - r, x + r, y + r), fill=color)


def render(transparent_corners: bool) -> Image.Image:
    bg = (0, 0, 0, 0) if transparent_corners else DISC_RGB + (255,)
    img = Image.new("RGBA", (SIZE, SIZE), bg)
    d = ImageDraw.Draw(img)

    if transparent_corners:
        # Disc
        cx, cy, r = s(256), s(256), s(240)
        d.ellipse((cx - r, cy - r, cx + r, cy + r), fill=DISC_RGB + (255,))

    # P outline (stroke 20, round caps/joints)
    stroke_line(d, [(156, 140), (156, 372)], 20, TAN_RGB + (255,))
    stroke_line(d, [(156, 140), (332, 140), (332, 260), (156, 260)], 20, TAN_RGB + (255,))
    stroke_line(d, [(156, 372), (216, 372)], 20, TAN_RGB + (255,))

    # Play triangle (filled, opacity 0.95)
    tri = [(s(212), s(172)), (s(308), s(200)), (s(212), s(228))]
    d.polygon(tri, fill=AMBER_RGB + (242,))

    # Check mark (stroke 22)
    stroke_line(d, [(256, 328), (304, 376), (392, 288)], 22, TAN_RGB + (255,))

    return img


def save(img: Image.Image, path: Path, size: int):
    out = img.resize((size, size), Image.LANCZOS)
    out.save(path, "PNG", optimize=True)
    print(f"  wrote {path} ({size}x{size}, {path.stat().st_size} bytes)")


def main():
    public = Path(__file__).resolve().parent.parent.parent / "frontend" / "public"
    transparent = render(transparent_corners=True)
    opaque = render(transparent_corners=False)

    print("Manifest icons (transparent corners):")
    save(transparent, public / "icon-192.png", 192)
    save(transparent, public / "icon-512.png", 512)
    # legacy filename kept for any external reference
    save(transparent, public / "icon.png", 512)
    save(transparent, public / "plextracker-logo.png", 512)
    save(transparent, public / "favicon-32.png", 32)

    print("Maskable icon (opaque full square for Android adaptive masks):")
    save(opaque, public / "icon-maskable.png", 512)

    print("Apple touch icon (opaque, iOS doesn't support transparency):")
    save(opaque, public / "apple-touch-icon.png", 180)


if __name__ == "__main__":
    main()
