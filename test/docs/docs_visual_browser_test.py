#!/usr/bin/env python3
"""Build and measure the docs hero in a real headless browser."""

from __future__ import annotations

import argparse
import html
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import threading
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_CHROME = Path("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
PROBE = r"""
<script>
addEventListener("load", () => setTimeout(() => {
  const query = new URLSearchParams(location.search);
  document.documentElement.setAttribute("data-md-color-scheme", query.get("scheme"));
  document.body.setAttribute("data-md-color-scheme", query.get("scheme"));
  const requestedWidth = Number(query.get("width"));
  document.body.style.width = requestedWidth + "px";
  document.body.style.maxWidth = requestedWidth + "px";
  const hero = document.querySelector(".kollect-hero");
  const buttons = [...hero.querySelectorAll(".md-button")];
  const box = element => {
    const rect = element.getBoundingClientRect();
    return {left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom};
  };
  const result = {
    viewport: innerWidth,
    documentClient: requestedWidth,
    documentScroll: document.body.scrollWidth,
    heroClient: hero.clientWidth,
    heroScroll: hero.scrollWidth,
    hero: box(hero),
    buttons: buttons.map(box),
  };
  const output = document.createElement("pre");
  output.id = "layout-result";
  output.textContent = JSON.stringify(result);
  document.body.append(output);
}, 250));
</script>
"""


class QuietHandler(SimpleHTTPRequestHandler):
    def log_message(self, _format: str, *_args: object) -> None:
        return


def parse_result(rendered: str) -> dict[str, object]:
    marker = '<pre id="layout-result">'
    if marker not in rendered:
        raise AssertionError("headless Chrome did not emit layout measurements")
    encoded = rendered.split(marker, 1)[1].split("</pre>", 1)[0]
    return json.loads(html.unescape(encoded))


def assert_layout(result: dict[str, object], width: int, scheme: str) -> None:
    assert result["documentScroll"] <= result["documentClient"], (width, scheme, result)
    assert result["heroScroll"] <= result["heroClient"], (width, scheme, result)
    for button in result["buttons"]:
        assert button["left"] >= 0, (width, scheme, button)
        assert button["right"] <= width, (width, scheme, button)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--screenshots", type=Path, help="optional evidence directory")
    args = parser.parse_args()
    mkdocs = os.environ.get("MKDOCS_BIN", shutil.which("mkdocs") or "mkdocs")
    chrome = Path(os.environ.get("CHROME_BIN", DEFAULT_CHROME))
    if not chrome.is_file():
        raise SystemExit(f"Chrome not found: {chrome}")

    with tempfile.TemporaryDirectory(prefix="kollect-docs-browser-") as temporary:
        site = Path(temporary) / "site"
        environment = os.environ.copy()
        subprocess.run(
            [mkdocs, "build", "--strict", "--site-dir", str(site)],
            cwd=ROOT,
            env=environment,
            check=True,
            stdout=subprocess.DEVNULL,
        )
        index = site / "index.html"
        index.write_text(index.read_text(encoding="utf-8").replace("</body>", f"{PROBE}</body>"), encoding="utf-8")

        handler = lambda *values: QuietHandler(*values, directory=str(site))
        server = ThreadingHTTPServer(("127.0.0.1", 0), handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            for scheme in ("default", "slate"):
                for width in (320, 390, 768, 1280):
                    url = f"http://127.0.0.1:{server.server_port}/?scheme={scheme}&width={width}"
                    command = [
                        str(chrome), "--headless=new", "--disable-gpu", "--hide-scrollbars",
                        "--no-first-run", "--no-default-browser-check", "--virtual-time-budget=1500",
                        "--force-device-scale-factor=1", f"--window-size={width},844", "--dump-dom", url,
                    ]
                    rendered = subprocess.run(command, check=True, capture_output=True, text=True).stdout
                    result = parse_result(rendered)
                    assert_layout(result, width, scheme)
                    print(f"{scheme} {width}px: {json.dumps(result, separators=(',', ':'))}")
                    if args.screenshots:
                        args.screenshots.mkdir(parents=True, exist_ok=True)
                        screenshot = args.screenshots / f"home-{scheme}-{width}.png"
                        screenshot_command = [
                            str(chrome), "--headless=new", "--disable-gpu", "--hide-scrollbars",
                            "--no-first-run", "--no-default-browser-check", "--virtual-time-budget=1500",
                            "--force-device-scale-factor=1", f"--window-size={width},1600",
                            f"--screenshot={screenshot}", url,
                        ]
                        subprocess.run(
                            screenshot_command,
                            check=True,
                            stdout=subprocess.DEVNULL,
                            stderr=subprocess.DEVNULL,
                        )
        finally:
            server.shutdown()
            thread.join()


if __name__ == "__main__":
    main()
