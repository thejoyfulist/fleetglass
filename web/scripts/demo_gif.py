"""Capture an 11-second loop from the synthetic dark-theme showcase fixture."""
import os
import subprocess
import tempfile
import threading
from http.server import ThreadingHTTPServer
from pathlib import Path
from playwright.sync_api import sync_playwright
import showcase

OUT = Path(__file__).resolve().parents[2] / "assets" / "screenshots"
FPS = 10
SECONDS = 11


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    server = ThreadingHTTPServer(("127.0.0.1", 0), showcase.ShowcaseHandler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        with tempfile.TemporaryDirectory(prefix="fleetglass-demo-") as tmp:
            with sync_playwright() as playwright:
                browser = playwright.chromium.launch(headless=True)
                page = browser.new_page(viewport={"width": 1440, "height": 900}, device_scale_factor=1)
                page.add_init_script("window.localStorage.setItem('fg-theme','dark');window.EventSource=class {constructor(){setTimeout(()=>this.onopen?.(),20)} addEventListener(){} close(){}};")
                page.goto(f"http://127.0.0.1:{server.server_port}/", wait_until="networkidle")
                page.locator("main").wait_for()
                page.wait_for_timeout(250)
                for frame in range(FPS * SECONDS):
                    if frame == 20:
                        page.keyboard.press("2")
                        page.locator(".agent-table").wait_for()
                    if frame == 42:
                        page.locator(".agent-table .terminal-link").first.click()
                        page.locator("#session-detail").wait_for()
                    if frame == 67:
                        page.keyboard.press("Escape")
                        page.keyboard.press("1")
                    if frame >= 87:
                        page.evaluate("frame => window.scrollTo(0, Math.min(400, (frame-87)*20))", frame)
                    page.screenshot(path=str(Path(tmp) / f"{frame:03}.png"))
                browser.close()
            source = str(Path(tmp) / "%03d.png")
            subprocess.run(["/usr/bin/ffmpeg", "-v", "error", "-y", "-framerate", str(FPS), "-i", source,
                            "-c:v", "libx264", "-pix_fmt", "yuv420p", "-crf", "30", "-movflags", "+faststart",
                            str(OUT / "demo.mp4")], check=True)
            subprocess.run(["/usr/bin/ffmpeg", "-v", "error", "-y", "-framerate", str(FPS), "-i", source,
                            "-filter_complex", "[0:v]split[a][b];[a]palettegen=max_colors=256:reserve_transparent=0:stats_mode=full[p];[b][p]paletteuse=dither=bayer:bayer_scale=5:diff_mode=rectangle",
                            "-loop", "0", str(OUT / "demo.gif")], check=True)
    finally:
        server.shutdown()
        server.server_close()
    print("Wrote synthetic demo.mp4 and demo.gif")


if __name__ == "__main__":
    main()
