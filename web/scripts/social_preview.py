"""Generate a social preview from the synthetic dark overview screenshot."""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parents[2]
SHOT = ROOT / "assets/screenshots/overview-dark.png"
OUT = ROOT / "assets/social-preview.png"
FONT = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
BOLD = "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"


def main():
    image = Image.new("RGB", (1280, 640), "#080808")
    draw = ImageDraw.Draw(image)
    for offset, alpha in ((0, 70), (18, 145), (36, 235)):
        draw.rounded_rectangle((72 + offset, 102 + offset, 116 + offset, 146 + offset), radius=9,
                               fill=(alpha, alpha, alpha))
    draw.text((190, 99), "Fleetglass", font=ImageFont.truetype(BOLD, 58), fill="#eeeeee")
    draw.text((74, 225), "One cockpit for your server and the AI agents running on it.",
              font=ImageFont.truetype(FONT, 25), fill="#cccccc")
    source = Image.open(SHOT).convert("RGB")
    crop = source.crop((0, 0, min(source.width, 1440), min(source.height, 400)))
    crop.thumbnail((1132, 300), Image.Resampling.LANCZOS)
    image.paste(crop, (74, 318))
    draw.rectangle((73, 317, 74 + crop.width, 318 + crop.height), outline="#777777", width=2)
    image.save(OUT, optimize=True)
    print(f"Wrote {OUT}")


if __name__ == "__main__":
    main()
