"""Verify a closed TCP target from the agent check through the live cockpit."""

import os
import subprocess
import sys
import tempfile
import time
from pathlib import Path

from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parents[2]
OUT = Path(sys.argv[1])
OUT.mkdir(parents=True, exist_ok=True)

with tempfile.TemporaryDirectory(prefix="fleetglass-closed-probe-") as temp:
    url_file = Path(temp) / "url"
    done_file = Path(temp) / "done"
    env = os.environ.copy()
    env["FLEETGLASS_E2E_URL_FILE"] = str(url_file)
    env["FLEETGLASS_E2E_DONE_FILE"] = str(done_file)
    proc = subprocess.Popen(
        ["go", "test", "./cmd/fleetglass-agent", "-run", "^TestClosedInternalTCPReachesOverviewAndAPI$", "-count=1", "-v"],
        cwd=ROOT, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
    )
    try:
        deadline = time.monotonic() + 60
        while not url_file.exists() and proc.poll() is None and time.monotonic() < deadline:
            time.sleep(.1)
        if not url_file.exists():
            raise RuntimeError("closed probe test did not expose its temporary server")
        base = url_file.read_text().strip()
        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)
            try:
                for theme in ("light", "dark"):
                    for width, height in ((1440, 900), (390, 844)):
                        page = browser.new_page(viewport={"width": width, "height": height})
                        errors = []
                        page.on("pageerror", lambda error: errors.append(str(error)))
                        page.add_init_script(f"localStorage.setItem('fg-theme', '{theme}')")
                        page.goto(base, wait_until="domcontentloaded")
                        page.get_by_label("Status", exact=True).get_by_text("1 check down").wait_for(timeout=15000)
                        overview = page.request.get(base + "/api/v1/hosts/fixture/overview").json()
                        probes = page.request.get(base + "/api/v1/hosts/fixture/probes").json()
                        state = next((r["state"] for r in overview["results"] if r["name"] == "internal:closed"), None)
                        if state != "crit" or len(probes) != 1 or probes[0]["state"] != "crit":
                            raise AssertionError(f"closed target state: overview={state}, probes={probes}")
                        if "DOWN" not in page.get_by_label("Status", exact=True).inner_text() or errors:
                            raise AssertionError(f"cockpit did not show Down: {errors}")
                        page.screenshot(path=str(OUT / f"closed-probe-{theme}-{width}.png"), full_page=True)
                        page.close()
            finally:
                browser.close()
    finally:
        done_file.write_text("done")
        try:
            output, _ = proc.communicate(timeout=20)
        except subprocess.TimeoutExpired:
            proc.terminate()
            output, _ = proc.communicate(timeout=10)
        if proc.returncode != 0:
            raise RuntimeError(f"closed probe integration test failed:\n{output}")

print(f"Closed TCP target reached Down in Overview, API and cockpit; 4 captures: {OUT}")
