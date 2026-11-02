"""Private easyDVD authoring helper; title encoding remains in the Go engine.

Input is a versioned JSON manifest; paths refer to private job artifacts.
Progress and the result are JSON lines. Run inside easyDVD's process tree.
"""
import json
import os
from pathlib import Path
import sys

sys.dont_write_bytecode = True

from bdmvkit import Disc, MenuButton
from udfkit.bluray import write_image


def emit(stage, **values):
    print(json.dumps({"stage": stage, **values}), flush=True)


def author(manifest):
    if manifest.get("version") != 1:
        raise ValueError("Unsupported authoring manifest version")
    ffmpeg = Path(manifest["ffmpeg"]).resolve(strict=True)
    os.environ["PATH"] = str(ffmpeg.parent) + os.pathsep + os.environ.get("PATH", "")
    output = Path(manifest["directory"])
    if output.exists():
        raise ValueError("Authoring requires a fresh generation directory")
    output.mkdir(parents=True)
    disc = Disc(first_play=manifest.get("first_play"), title_key=manifest.get("title_key"))
    for title in manifest["titles"]:
        disc.add_title(title["clip"], fps=tuple(title["fps"]), video=title["video"],
                       aspect=title["aspect"], audio=[("ac3", title["channels"], "und")],
                       end=title.get("end"))
    for page in manifest.get("menus") or []:
        buttons = [MenuButton(b["id"], b["x"], b["y"], b["selected"],
                              activated=b.get("activated"), action=b["action"],
                              nav=b.get("nav", {})) for b in page["buttons"]]
        disc.add_menu(page["id"], page["background"], buttons, fps=(24, 1))
    for playlist in manifest.get("playlists") or []:
        disc.add_playlist(playlist["id"], playlist["titles"], end=playlist.get("end"))
    emit("authoring")
    result = disc.write(str(output), play_all=False)
    if manifest.get("iso"):
        emit("iso")
        write_image(str(output), manifest["iso"], manifest.get("label", "EASYDVD_BD"))
    emit("complete", result=result)


if __name__ == "__main__":
    try:
        if sys.argv[1:] == ["--check"]:
            emit("ready", protocol=1, runtime="experimental-2026.11.1")
            sys.exit(0)
        with open(sys.argv[1], encoding="utf-8") as f:
            author(json.load(f))
    except Exception as error:
        emit("error", message=str(error))
        raise
