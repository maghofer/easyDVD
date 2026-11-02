"""Real mixed-format authoring gate. Generated files stay under --scratch.

Run with the prepared private Python runtime; --reader is an independently
built bluray-navcheck.exe linked to libbluray (which also reads the UDF ISO).
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys


def run(args):
    result = subprocess.run([str(a) for a in args], capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError(f"{args[0]} exited {result.returncode}: {result.stdout}\n{result.stderr}")
    return result.stdout


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--scratch", type=Path, required=True)
    parser.add_argument("--runtime", type=Path, required=True)
    parser.add_argument("--ffmpeg", type=Path, required=True)
    parser.add_argument("--reader", type=Path, required=True)
    parser.add_argument("--large", action="store_true")
    args = parser.parse_args()
    root = args.scratch.resolve()
    root.mkdir(parents=True, exist_ok=False)
    os.environ["TEMP"] = os.environ["TMP"] = str(root)
    ff = [args.ffmpeg, "-hide_banner", "-v", "error", "-nostdin", "-y"]
    # Format/aspect/rate codes are independently checked through libbluray.
    cases = [
        ("576", 720, 576, [25, 1], 2, 2, ":fake-interlaced=1", ""),
        ("720", 1280, 720, [50, 1], 3, 6, "", ""),
        ("1080", 1920, 1080, [24, 1], 3, 2, "", ""),
        ("1080", 1920, 1080, [30000, 1001], 3, 2, ":interlaced=1:tff=1", "tinterlace=interleave_top"),
    ]
    titles = []
    for n, (video, width, height, fps, aspect, channels, extra, vf) in enumerate(cases, 1):
        clip = root / f"title-{n}.m2ts"
        rate = f"{fps[0] * (2 if vf else 1)}/{fps[1]}"
        cmd = ff + ["-f", "lavfi", "-i", f"testsrc2=size={width}x{height}:rate={rate}:duration=2",
                    "-f", "lavfi", "-i", f"anullsrc=r=48000:cl={'5.1' if channels == 6 else 'stereo'}"]
        if vf:
            cmd += ["-vf", vf]
        params = (f"bluray-compat=1:aud=1:slices=4:nal-hrd=vbr:force-cfr=1:"
                  f"keyint={round(fps[0]/fps[1])}:min-keyint=1:open-gop=0" + extra)
        cmd += ["-c:v", "libx264", "-preset", "medium", "-crf", "20", "-profile:v", "high",
                "-level:v", "4.1", "-maxrate", "38000k", "-bufsize", "30000k", "-x264-params", params,
                "-pix_fmt", "yuv420p", "-aspect", "4:3" if aspect == 2 else "16:9", "-c:a", "ac3",
                "-ar", "48000", "-ac", str(channels), "-b:a", "640k" if channels == 6 else "448k",
                "-t", "2", "-streamid", "0:4113", "-streamid", "1:4352", "-mpegts_pmt_start_pid", "256",
                "-f", "mpegts", "-mpegts_m2ts_mode", "1", clip]
        run(cmd)
        run(ff + ["-i", clip, "-f", "null", "-"])
        titles.append(dict(clip=str(clip), video=video, fps=fps, aspect=aspect, channels=channels,
                           end=f"menu:page{1 if n <= 2 else 2}"))
    for name, size, color in [("background", "1920x1080", "navy"), ("selected", "500x80", "orange")]:
        run(ff + ["-f", "lavfi", "-i", f"color=c={color}:s={size}", "-frames:v", "1", root / f"{name}.png"])
    menus = []
    for page in (1, 2):
        actions = [f"title:{page*2-1}", f"title:{page*2}", "playlist:all", f"menu:page{3-page}"]
        menus.append(dict(id=f"page{page}", background=str(root / "background.png"), buttons=[
            dict(id=f"b{k}", x=160, y=180+k*150, selected=str(root / "selected.png"), action=action)
            for k, action in enumerate(actions)]))
    manifest = dict(version=1, ffmpeg=str(args.ffmpeg), directory=str(root / "disc"),
                    iso=str(root / "disc.iso"), titles=titles, menus=menus,
                    playlists=[dict(id="all", titles=[1, 2, 3, 4], end="menu:page1")])
    path = root / "manifest.json"
    path.write_text(json.dumps(manifest), encoding="utf-8")
    print(run([sys.executable, args.runtime / "author.py", path]), flush=True)
    checks = {}
    for disc in (root / "disc", root / "disc.iso"):
        metadata = [json.loads(line) for line in run([args.reader, disc, "inspect", 7]).splitlines()]
        assert [(m["format"], m["rate"], m["aspect"], m["audio"]) for m in metadata[:4]] == [
            (2, 3, 2, 3), (5, 6, 3, 6), (6, 2, 3, 3), (4, 4, 3, 3)], metadata
        for m in metadata[:4]:
            assert abs(m["seconds"] - 2) < 0.08, m
        assert metadata[4]["clips"] == 4, metadata
        print(run([args.reader, disc, "seek", 4]), flush=True)
        def navigation(keys, expected):
            events = [json.loads(line) for line in run([args.reader, disc, "play", keys]).splitlines()]
            got = [e["playlist"] for e in events if "playlist" in e]
            assert got == expected, (disc, keys, got, expected)
            assert events[-1]["draws"] > 0, events
            return events
        checks[str(disc)] = dict(metadata=metadata,
            select=navigation("16", [6, 1, 6]),
            next_page=navigation("13,13,13,16,16", [6, 7, 3, 7]),
            play_all=navigation("13,13,16", [6, 5, 6]))
    autoplay = dict(manifest, directory=str(root / "autoplay"), iso=str(root / "autoplay.iso"), menus=[], playlists=[])
    autoplay["titles"] = [dict(t, end=f"title:{n+1}" if n < 4 else "stop") for n, t in enumerate(titles, 1)]
    path.write_text(json.dumps(autoplay), encoding="utf-8")
    print(run([sys.executable, args.runtime / "author.py", path]), flush=True)
    for disc in (root / "autoplay", root / "autoplay.iso"):
        events = [json.loads(line) for line in run([args.reader, disc, "play", ""]).splitlines()]
        assert [e["playlist"] for e in events if "playlist" in e] == [1, 2, 3, 4], events
        assert any(e.get("end") == 4 for e in events), events
        checks[str(disc)] = events
    if args.large:
        from udfkit.bluray import write_image
        payload = root / "disc" / "BDMV" / "AUXDATA" / "large.bin"
        with payload.open("wb") as f:
            f.seek(2**32 + 4096)
            f.write(b"EASYDVD-LARGE!")
        print("Writing large-file UDF ISO", flush=True)
        write_image(str(root / "disc"), str(root / "large.iso"), "EASYDVD_LARGE")
        checks["large"] = run([args.reader, root / "large.iso", "large"])
        print(checks["large"], flush=True)
    (root / "result.json").write_text(json.dumps(checks, indent=2), encoding="utf-8")
    print("PASS: mixed formats, menus, Play All and autoplay in BDMV directory and UDF ISO")


if __name__ == "__main__":
    main()
