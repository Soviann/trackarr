#!/usr/bin/env python3
import math
from PIL import Image, ImageDraw

def render_trackarr_graphic(size, content_scale=0.85):
    """
    Renders the raw transparent Trackarr graphic (Monogram TR + 65/35 orbital ring)
    with clockwise progression starting at 12 o'clock (solid) and finishing with dots (35%).
    """
    scale = 4
    canvas_size = size * scale
    img = Image.new('RGBA', (canvas_size, canvas_size), (0, 0, 0, 0))

    cx = canvas_size / 2.0
    cy = canvas_size / 2.0
    radius = (canvas_size / 2.0) * content_scale * 0.90
    stroke_w = max(2.0 * scale, (canvas_size * content_scale) * 0.065)

    draw = ImageDraw.Draw(img)

    # 1. 35% Dotted Arc: Finishes the circle clockwise (from +144 deg / 8 o'clock up to +270 deg / 12 o'clock)
    dot_count = 13
    for i in range(dot_count):
        t = i / float(dot_count - 1)
        angle_deg = 144 + t * 126
        rad = math.radians(angle_deg)
        x = cx + radius * math.cos(rad)
        y = cy + radius * math.sin(rad)
        r_dot = stroke_w * 0.45
        draw.ellipse([x - r_dot, y - r_dot, x + r_dot, y + r_dot], fill=(148, 163, 184, 190))

    # 2. 65% Solid Progress Arc: Starts at 12 o'clock (-90 deg) and sweeps clockwise to +144 deg (8 o'clock)
    num_steps = 150
    for i in range(num_steps):
        t = i / float(num_steps - 1)
        angle_deg = -90 + t * 234
        rad = math.radians(angle_deg)
        x = cx + radius * math.cos(rad)
        y = cy + radius * math.sin(rad)

        # Color gradient: 0% Cyan (#06b6d4) -> 50% Blue (#3b82f6) -> 100% Purple (#8b5cf6)
        if t < 0.5:
            local_t = t / 0.5
            r = int(6 + local_t * (59 - 6))
            g = int(182 + local_t * (130 - 182))
            b = int(212 + local_t * (246 - 212))
        else:
            local_t = (t - 0.5) / 0.5
            r = int(59 + local_t * (139 - 59))
            g = int(130 + local_t * (92 - 130))
            b = int(246 + local_t * (246 - 246))

        r_stroke = stroke_w * 0.52
        draw.ellipse([x - r_stroke, y - r_stroke, x + r_stroke, y + r_stroke], fill=(r, g, b, 255))

    # 3. Monogram TR vectors (scaled into the inner area)
    def s_x(val):
        offset = (1.0 - content_scale) / 2.0 * canvas_size
        return offset + val * ((canvas_size * content_scale) / 120.0)

    def s_y(val):
        offset = (1.0 - content_scale) / 2.0 * canvas_size
        return offset + val * ((canvas_size * content_scale) / 120.0)

    # Vertical gradient shader for TR glyphs
    grad_img = Image.new('RGBA', (canvas_size, canvas_size), (0, 0, 0, 0))
    gdraw = ImageDraw.Draw(grad_img)
    for y_idx in range(canvas_size):
        t = y_idx / float(canvas_size)
        r = int(6 + t * (139 - 6))
        g = int(182 + t * (92 - 182))
        b = int(212 + t * (246 - 212))
        gdraw.line([(0, y_idx), (canvas_size, y_idx)], fill=(r, g, b, 255))

    # Mask for TR glyphs
    mask = Image.new('L', (canvas_size, canvas_size), 0)
    mdraw = ImageDraw.Draw(mask)

    # 'T' glyph
    mdraw.rectangle([s_x(25), s_y(33), s_x(57), s_y(42)], fill=255)
    mdraw.rectangle([s_x(36), s_y(42), s_x(46), s_y(87)], fill=255)

    # 'R' glyph
    mdraw.rectangle([s_x(51), s_y(33), s_x(62), s_y(87)], fill=255)
    radius_r = int(13 * (canvas_size * content_scale / 120.0))
    mdraw.rounded_rectangle([s_x(51), s_y(33), s_x(90), s_y(60)], radius=radius_r, fill=255)
    mdraw.polygon([
        (s_x(62), s_y(58)),
        (s_x(76), s_y(58)),
        (s_x(91), s_y(87)),
        (s_x(78), s_y(87)),
        (s_x(62), s_y(62)),
    ], fill=255)

    radius_eye = int(5 * (canvas_size * content_scale / 120.0))
    mdraw.rounded_rectangle([s_x(62), s_y(42), s_x(79), s_y(52)], radius=radius_eye, fill=0)

    # Composite TR glyphs with gradient
    img.paste(grad_img, (0, 0), mask)

    # Lanczos downsampling
    return img.resize((size, size), Image.Resampling.LANCZOS)

def create_solid_bg_icon(size, content_scale=0.72, bg_color=(11, 15, 25, 255)):
    base = Image.new('RGBA', (size, size), bg_color)
    graphic = render_trackarr_graphic(size, content_scale=content_scale)
    base.paste(graphic, (0, 0), graphic)
    return base

# 1. Generate Favicons (Transparent background for browser tabs)
print("Rendering browser tab favicons with clockwise progress...")
fav16 = render_trackarr_graphic(16, content_scale=0.95)
fav32 = render_trackarr_graphic(32, content_scale=0.95)
fav48 = render_trackarr_graphic(48, content_scale=0.95)
fav64 = render_trackarr_graphic(64, content_scale=0.95)

fav16.save("frontend/public/favicon-16.png")
fav32.save("frontend/public/favicon-32.png")
fav32.save("frontend/public/favicon.ico", format="ICO", sizes=[(16, 16), (32, 32), (48, 48), (64, 64)])

# 2. Generate PWA App Icons (Solid #0b0f19 background with glowing graphic)
print("Rendering PWA app icons (192, 512, maskable, apple-touch)...")

# icon-192.png (192x192) - 82% content scale on dark background
icon192 = create_solid_bg_icon(192, content_scale=0.82)
icon192.save("frontend/public/icon-192.png")

# icon-512.png (512x512) - 82% content scale on dark background
icon512 = create_solid_bg_icon(512, content_scale=0.82)
icon512.save("frontend/public/icon-512.png")

# icon-maskable.png (512x512) - 70% safe zone content scale on solid #0b0f19
icon_maskable = create_solid_bg_icon(512, content_scale=0.70)
icon_maskable.save("frontend/public/icon-maskable.png")

# apple-touch-icon.png (180x180) - 80% content scale on dark background
apple_icon = create_solid_bg_icon(180, content_scale=0.80)
apple_icon.save("frontend/public/apple-touch-icon.png")

# icon.png (512x512) - Master master icon
icon512.save("frontend/public/icon.png")

print("Successfully regenerated all icons with clockwise 65% solid progress ending in 35% dots!")
