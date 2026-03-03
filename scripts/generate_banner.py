"""
Generate a professional banner image for the Auth API README.
Uses Pillow to create a gradient background with a shield+lock icon and styled text.
"""

from PIL import Image, ImageDraw, ImageFont, ImageFilter
import math


def create_gradient(width, height, colors):
    """Create a smooth multi-stop gradient."""
    img = Image.new("RGB", (width, height))
    draw = ImageDraw.Draw(img)

    for x in range(width):
        t = x / (width - 1)
        # Smooth cubic interpolation
        t = t * t * (3 - 2 * t)

        if len(colors) == 2:
            r = int(colors[0][0] + (colors[1][0] - colors[0][0]) * t)
            g = int(colors[0][1] + (colors[1][1] - colors[0][1]) * t)
            b = int(colors[0][2] + (colors[1][2] - colors[0][2]) * t)
        else:
            # Multi-stop gradient
            segment = t * (len(colors) - 1)
            idx = min(int(segment), len(colors) - 2)
            local_t = segment - idx
            local_t = local_t * local_t * (3 - 2 * local_t)
            r = int(colors[idx][0] + (colors[idx + 1][0] - colors[idx][0]) * local_t)
            g = int(colors[idx][1] + (colors[idx + 1][1] - colors[idx][1]) * local_t)
            b = int(colors[idx][2] + (colors[idx + 1][2] - colors[idx][2]) * local_t)

        draw.line([(x, 0), (x, height)], fill=(r, g, b))

    return img


def draw_shield(draw, cx, cy, size, fill_color, outline_color, outline_width=3):
    """Draw a clean, modern shield shape with smooth bezier-like curves."""
    # Render at higher resolution for anti-aliasing
    scale = 4
    s = size * scale
    ocx, ocy = cx * scale, cy * scale

    temp_size = (int(cx * scale * 2 + s * 2), int(cy * scale * 2 + s * 2))
    temp = Image.new("RGBA", temp_size, (0, 0, 0, 0))
    td = ImageDraw.Draw(temp)

    # Classic heraldic shield proportions
    half_w = s * 0.52
    top_y = ocy - s * 0.55
    bottom_y = ocy + s * 0.7
    shoulder_y = ocy - s * 0.35  # Where the sides start curving in

    # Build smooth shield outline with many points
    points = []
    steps = 40

    # Top-left corner with rounded edge
    corner_r = s * 0.08
    for i in range(steps // 4 + 1):
        t = i / (steps // 4)
        angle = math.pi + t * (math.pi / 2)  # 180 -> 270 degrees
        px = (ocx - half_w + corner_r) + corner_r * math.cos(angle)
        py = (top_y + corner_r) + corner_r * math.sin(angle)
        points.append((px, py))

    # Top edge (flat)
    points.append((ocx - half_w + corner_r, top_y))
    points.append((ocx + half_w - corner_r, top_y))

    # Top-right corner with rounded edge
    for i in range(steps // 4 + 1):
        t = i / (steps // 4)
        angle = -math.pi / 2 + t * (math.pi / 2)  # 270 -> 360 degrees
        px = (ocx + half_w - corner_r) + corner_r * math.cos(angle)
        py = (top_y + corner_r) + corner_r * math.sin(angle)
        points.append((px, py))

    # Right side: straight down to shoulder, then smooth curve to bottom point
    points.append((ocx + half_w, shoulder_y))
    for i in range(1, steps + 1):
        t = i / steps
        # Cubic bezier-like curve from (half_w, shoulder_y) to (0, bottom_y)
        # Control points create a smooth taper
        t2 = t * t
        t3 = t2 * t
        inv = 1 - t
        inv2 = inv * inv
        inv3 = inv2 * inv

        # Bezier: P0=(half_w, shoulder_y), P1=(half_w, mid), P2=(half_w*0.2, bottom-offset), P3=(0, bottom)
        p0x, p0y = half_w, shoulder_y - ocy
        p1x, p1y = half_w * 0.95, (bottom_y - ocy) * 0.4
        p2x, p2y = half_w * 0.3, (bottom_y - ocy) * 0.85
        p3x, p3y = 0, bottom_y - ocy

        bx = inv3 * p0x + 3 * inv2 * t * p1x + 3 * inv * t2 * p2x + t3 * p3x
        by = inv3 * p0y + 3 * inv2 * t * p1y + 3 * inv * t2 * p2y + t3 * p3y
        points.append((ocx + bx, ocy + by))

    # Left side: mirror of right (bottom point back up to shoulder)
    for i in range(1, steps + 1):
        t = i / steps
        t2 = t * t
        t3 = t2 * t
        inv = 1 - t
        inv2 = inv * inv
        inv3 = inv2 * inv

        p0x, p0y = 0, bottom_y - ocy
        p1x, p1y = -half_w * 0.3, (bottom_y - ocy) * 0.85
        p2x, p2y = -half_w * 0.95, (bottom_y - ocy) * 0.4
        p3x, p3y = -half_w, shoulder_y - ocy

        bx = inv3 * p0x + 3 * inv2 * t * p1x + 3 * inv * t2 * p2x + t3 * p3x
        by = inv3 * p0y + 3 * inv2 * t * p1y + 3 * inv * t2 * p2y + t3 * p3y
        points.append((ocx + bx, ocy + by))

    # Close back to top-left
    points.append((ocx - half_w, shoulder_y))

    # Draw on high-res temp image
    if fill_color and len(fill_color) >= 3:
        fill = fill_color if len(fill_color) == 4 else (*fill_color, 255)
        td.polygon(points, fill=fill)
    if outline_color and outline_width > 0:
        ol = outline_color if len(outline_color) == 4 else (*outline_color, 255)
        td.polygon(points, outline=ol, width=outline_width * scale)

    # Downscale with anti-aliasing
    target_w = temp_size[0] // scale
    target_h = temp_size[1] // scale
    temp = temp.resize((target_w, target_h), Image.LANCZOS)

    return temp


def draw_lock_icon(img, cx, cy, size, color, bg_color):
    """Draw a clean, modern lock icon with anti-aliasing."""
    scale = 4
    s = size * scale
    w = img.width * scale
    h = img.height * scale

    temp = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    td = ImageDraw.Draw(temp)

    ocx = cx * scale
    ocy = cy * scale

    # Lock body dimensions
    body_w = s * 0.28
    body_h = s * 0.26
    body_top = ocy - body_h * 0.15
    body_radius = s * 0.05

    col = color if len(color) == 4 else (*color, 255)
    bg = bg_color if len(bg_color) == 4 else (*bg_color, 255)

    # Lock body (rounded rectangle)
    td.rounded_rectangle(
        [ocx - body_w, body_top, ocx + body_w, body_top + body_h],
        radius=int(body_radius),
        fill=col,
    )

    # Shackle (thick arc above the body)
    shackle_w = s * 0.18
    shackle_h = s * 0.22
    line_w = max(int(s * 0.04), 4)

    shackle_bbox = [
        ocx - shackle_w,
        body_top - shackle_h,
        ocx + shackle_w,
        body_top + shackle_h * 0.3,
    ]
    td.arc(shackle_bbox, start=180, end=0, fill=col, width=line_w)

    # Shackle legs connecting arc to body
    leg_bottom = body_top + 2 * scale
    arc_center_y = (shackle_bbox[1] + shackle_bbox[3]) / 2
    # Left leg
    td.rectangle(
        [ocx - shackle_w - line_w // 2, arc_center_y, ocx - shackle_w + line_w // 2, leg_bottom],
        fill=col,
    )
    # Right leg
    td.rectangle(
        [ocx + shackle_w - line_w // 2, arc_center_y, ocx + shackle_w + line_w // 2, leg_bottom],
        fill=col,
    )

    # Keyhole (circle + rectangle slot)
    kh_r = s * 0.05
    kh_cy = body_top + body_h * 0.38
    td.ellipse(
        [ocx - kh_r, kh_cy - kh_r, ocx + kh_r, kh_cy + kh_r],
        fill=bg,
    )
    slot_w = kh_r * 0.5
    slot_h = kh_r * 2.0
    td.rounded_rectangle(
        [ocx - slot_w, kh_cy + kh_r * 0.3, ocx + slot_w, kh_cy + kh_r * 0.3 + slot_h],
        radius=int(slot_w * 0.8),
        fill=bg,
    )

    # Downscale
    temp = temp.resize((img.width, img.height), Image.LANCZOS)
    return temp


def add_glow(img, cx, cy, radius, color, intensity=0.3):
    """Add a subtle glow effect around a point."""
    glow = Image.new("RGBA", img.size, (0, 0, 0, 0))
    glow_draw = ImageDraw.Draw(glow)

    for r in range(radius, 0, -2):
        alpha = int(255 * intensity * (r / radius) ** 0.5 * (1 - r / radius))
        alpha = max(0, min(255, alpha))
        glow_draw.ellipse(
            [cx - r, cy - r, cx + r, cy + r],
            fill=(*color, alpha),
        )

    glow = glow.filter(ImageFilter.GaussianBlur(radius=radius // 4))
    img_rgba = img.convert("RGBA")
    img_rgba = Image.alpha_composite(img_rgba, glow)
    return img_rgba.convert("RGB")


def add_subtle_pattern(img):
    """Add a subtle dot pattern overlay for texture."""
    overlay = Image.new("RGBA", img.size, (0, 0, 0, 0))
    draw = ImageDraw.Draw(overlay)

    spacing = 30
    for y in range(0, img.height, spacing):
        for x in range(0, img.width, spacing):
            alpha = 8
            draw.ellipse([x, y, x + 2, y + 2], fill=(255, 255, 255, alpha))

    img_rgba = img.convert("RGBA")
    result = Image.alpha_composite(img_rgba, overlay)
    return result.convert("RGB")


def add_wave_decoration(draw, width, height, color, alpha_img=None):
    """Add decorative wave lines at the bottom."""
    if alpha_img is None:
        return

    overlay_draw = ImageDraw.Draw(alpha_img)

    for wave_idx, (offset_y, wave_alpha) in enumerate([(0, 20), (15, 15), (30, 10)]):
        points = []
        base_y = height - 40 - offset_y
        for x in range(0, width + 1, 2):
            y = base_y + math.sin(x / 80 + wave_idx * 1.5) * 8
            points.append((x, y))
        # Close the polygon at the bottom
        points.append((width, height))
        points.append((0, height))

        overlay_draw.polygon(points, fill=(*color, wave_alpha))


def generate_banner():
    """Generate the main banner image."""
    width = 1280
    height = 400

    # Color palette - dark navy to purple
    gradient_colors = [
        (10, 10, 35),     # Very dark navy
        (26, 26, 60),     # Dark navy (#1a1a3c)
        (50, 20, 100),    # Deep purple
        (74, 20, 140),    # Purple (#4a148c)
    ]

    # Create gradient background
    img = create_gradient(width, height, gradient_colors)

    # Add subtle dot pattern
    img = add_subtle_pattern(img)

    # Add glow behind shield area
    shield_cx = width * 0.28
    shield_cy = height * 0.48
    img = add_glow(img, int(shield_cx), int(shield_cy), 180, (100, 60, 200), intensity=0.25)

    # Add wave decoration
    overlay = Image.new("RGBA", (width, height), (0, 0, 0, 0))
    add_wave_decoration(None, width, height, (255, 255, 255), overlay)
    img_rgba = img.convert("RGBA")
    img_rgba = Image.alpha_composite(img_rgba, overlay)
    img = img_rgba.convert("RGB")

    draw = ImageDraw.Draw(img)

    # Draw shield with glow effect
    shield_size = 120
    # Shield glow layers
    for offset in range(8, 0, -1):
        glow_shield = draw_shield(
            None,
            int(shield_cx),
            int(shield_cy),
            shield_size + offset * 3,
            fill_color=(100, 70, 200, int(18 * (1 - offset / 8))),
            outline_color=None,
            outline_width=0,
        )
        img_rgba = img.convert("RGBA")
        # Crop/paste glow shield
        glow_shield_cropped = glow_shield.crop((0, 0, min(glow_shield.width, img.width), min(glow_shield.height, img.height)))
        base = Image.new("RGBA", img.size, (0, 0, 0, 0))
        base.paste(glow_shield_cropped, (0, 0))
        img_rgba = Image.alpha_composite(img_rgba, base)
        img = img_rgba.convert("RGB")

    # Main shield
    shield_fill = (40, 30, 75)
    shield_outline = (150, 115, 245)
    shield_layer = draw_shield(
        None,
        int(shield_cx),
        int(shield_cy),
        shield_size,
        fill_color=shield_fill,
        outline_color=shield_outline,
        outline_width=3,
    )
    img_rgba = img.convert("RGBA")
    shield_cropped = shield_layer.crop((0, 0, min(shield_layer.width, img.width), min(shield_layer.height, img.height)))
    base = Image.new("RGBA", img.size, (0, 0, 0, 0))
    base.paste(shield_cropped, (0, 0))
    img_rgba = Image.alpha_composite(img_rgba, base)
    img = img_rgba.convert("RGB")

    # Draw lock icon inside shield
    lock_color = (195, 175, 250)
    lock_layer = draw_lock_icon(img, int(shield_cx), int(shield_cy) - 2, shield_size, lock_color, shield_fill)
    img_rgba = img.convert("RGBA")
    img_rgba = Image.alpha_composite(img_rgba, lock_layer)
    img = img_rgba.convert("RGB")

    # Recreate draw object after compositing
    draw = ImageDraw.Draw(img)

    # Load fonts
    try:
        title_font = ImageFont.truetype("C:/Windows/Fonts/segoeuib.ttf", 72)
        subtitle_font = ImageFont.truetype("C:/Windows/Fonts/segoeuil.ttf", 26)
        tag_font = ImageFont.truetype("C:/Windows/Fonts/segoeui.ttf", 18)
    except Exception:
        title_font = ImageFont.truetype("C:/Windows/Fonts/arialbd.ttf", 72)
        subtitle_font = ImageFont.truetype("C:/Windows/Fonts/arial.ttf", 26)
        tag_font = ImageFont.truetype("C:/Windows/Fonts/arial.ttf", 18)

    # Title text
    text_x = width * 0.45
    title_y = height * 0.22

    # Title shadow
    draw.text((text_x + 2, title_y + 2), "Auth API", fill=(0, 0, 0, 80), font=title_font)
    # Title main
    draw.text((text_x, title_y), "Auth API", fill=(255, 255, 255), font=title_font)

    # Subtitle
    subtitle_y = title_y + 85
    draw.text(
        (text_x, subtitle_y),
        "Production-Ready Authentication & Authorization",
        fill=(180, 170, 210),
        font=subtitle_font,
    )

    # Feature tags
    tags = ["JWT", "OAuth2", "WebAuthn", "2FA", "RBAC", "Multi-Tenant"]
    tag_y = subtitle_y + 50
    tag_x = text_x
    tag_padding_h = 12
    tag_padding_v = 6
    tag_spacing = 10

    for tag in tags:
        bbox = tag_font.getbbox(tag)
        tx_offset = bbox[0]  # x offset from origin
        ty_offset = bbox[1]  # y offset from origin (ascent)
        tw = bbox[2] - bbox[0]
        th = bbox[3] - bbox[1]

        # Tag background (rounded pill)
        pill_w = tw + tag_padding_h * 2
        pill_h = th + tag_padding_v * 2
        tag_bg_left = tag_x
        tag_bg_top = tag_y
        tag_bg_right = tag_x + pill_w
        tag_bg_bottom = tag_y + pill_h

        # Semi-transparent tag background
        tag_overlay = Image.new("RGBA", (width, height), (0, 0, 0, 0))
        tag_draw = ImageDraw.Draw(tag_overlay)
        tag_draw.rounded_rectangle(
            [tag_bg_left, tag_bg_top, tag_bg_right, tag_bg_bottom],
            radius=12,
            fill=(255, 255, 255, 25),
            outline=(160, 140, 220, 80),
            width=1,
        )
        img_rgba = img.convert("RGBA")
        img_rgba = Image.alpha_composite(img_rgba, tag_overlay)
        img = img_rgba.convert("RGB")
        draw = ImageDraw.Draw(img)

        # Tag text - centered in the pill
        text_draw_x = tag_bg_left + (pill_w - tw) / 2 - tx_offset
        text_draw_y = tag_bg_top + (pill_h - th) / 2 - ty_offset
        draw.text(
            (text_draw_x, text_draw_y),
            tag,
            fill=(200, 190, 240),
            font=tag_font,
        )

        tag_x = tag_bg_right + tag_spacing

    # Decorative line between shield and text
    line_x = width * 0.42
    draw.line(
        [(line_x, height * 0.25), (line_x, height * 0.75)],
        fill=(100, 80, 180),
        width=2,
    )

    # Small decorative dots
    for i in range(3):
        dot_y = height * 0.25 + i * (height * 0.5) / 2
        draw.ellipse(
            [line_x - 3, dot_y - 3, line_x + 3, dot_y + 3],
            fill=(140, 120, 220),
        )

    # Save
    img.save("banner.png", "PNG", quality=95, optimize=True)
    print(f"Banner saved: {width}x{height}px -> banner.png")


if __name__ == "__main__":
    generate_banner()
