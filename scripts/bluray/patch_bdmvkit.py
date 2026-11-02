"""Version-checked, reproducible easyDVD changes to bdmvkit 0.7.0.

Keep modified upstream files in the private runtime, with their LGPL notices.
Never apply these substitutions to an arbitrary installed package.
"""
from pathlib import Path
import sys


def patch(root):
    def edit(name, changes):
        path = root / name
        text = path.read_text(encoding="utf-8")
        for before, after in changes:
            if text.count(before) != 1:
                raise RuntimeError(f"Unexpected bdmvkit 0.7.0 source: {name}: {before!r}")
            text = text.replace(before, after)
        notice = "# Modified for easyDVD, 2026-11-02; see ../patch_bdmvkit.py.\n"
        path.write_text(notice + text, encoding="utf-8", newline="\n")

    edit("formats.py", [
        ("FORMATS = {", "FORMATS = {\n    (24, 1): (6, 2),"),
        ("FORMATS_720 = {", "FORMATS_720 = {\n    (24, 1): (5, 2),"),
        ('return (FORMATS_720 if video == "720" else FORMATS).get(disc_rate(fps, video))',
         'tables = {"1080": FORMATS, "720": FORMATS_720,\n'
         '              "576": {(25, 1): (2, 3)}, "480": {(30000, 1001): (1, 4)}}\n'
         '    return tables.get(video, {}).get(disc_rate(fps, video))'),
    ])
    edit("program.py", [
        ('    hold: bool = False', '    video: str = "1080"\n    aspect: int = 3  # CLPI: 2 = 4:3, 3 = 16:9\n    hold: bool = False'),
    ])
    edit("disc.py", [
        ('WIDTH, lang3, rate_code', 'WIDTH, lang3, rate_code, video_codes'),
        ('    prohibit: list = dataclasses.field(default_factory=list)           # uops.KEYS names\n\n\n@dataclasses.dataclass\nclass MenuButton:',
         '    prohibit: list = dataclasses.field(default_factory=list)           # uops.KEYS names\n'
         '    video: str = "1080"\n    aspect: int = 3\n\n\n@dataclasses.dataclass\nclass MenuButton:'),
        ('                  prohibit=()):', '                  prohibit=(), video="1080", aspect=3):'),
        ('        if tuple(fps) not in FORMATS:\n            raise BdmvError(f"{fps[0]}/{fps[1]} fps has no Blu-ray format")\n        for codec',
         '        if video_codes(tuple(fps), video) is None or aspect not in (2, 3):\n'
         '            raise BdmvError(f"Unsupported clip format: {video}, {fps}, aspect {aspect}")\n        for codec'),
        ('                                 list(prohibit)))', '                                 list(prohibit), video, aspect))'),
        ('                popup=n in self.popups,', '                video=t.video, aspect=t.aspect, popup=n in self.popups,'),
    ])
    edit("clip.py", [
        ('def clpi(scan, video_format, frame_rate, audio, pg=(), ig=False):',
         'def clpi(scan, video_format, frame_rate, audio, pg=(), ig=False, aspect=3):'),
        ('video_format << 4 | frame_rate, 3 << 4,', 'video_format << 4 | frame_rate, aspect << 4,'),
    ])
    edit("nav.py", [
        ('codes = video_codes(fps, program.video)', 'codes = video_codes(fps, getattr(program, "video", "1080"))'),
        ('{program.video} format', '{getattr(program, "video", "1080")} format'),
        ('video_format, frame_rate = _codes(t.fps, p)', 'video_format, frame_rate = _codes(t.fps, t)'),
        ('video_format, frame_rate = _codes(m.fps or fps, p)',
         'video_format, frame_rate = _codes(m.fps or fps, m)'),
        ('clpi(scan, video_format, frame_rate, audio, pg, ig=t.popup)',
         'clpi(scan, video_format, frame_rate, audio, pg, ig=t.popup, aspect=t.aspect)'),
    ])
    edit("playlist.py", [
        ('fmt = 6 if channels > 2 else 3', 'fmt = 1 if channels == 1 else 6 if channels > 2 else 3'),
    ])
    edit("encode.py", [
        ('"force-cfr=1")', '"force-cfr=1:colorprim=bt709:transfer=bt709:colormatrix=bt709:fullrange=off")'),
    ])


if __name__ == "__main__":
    patch(Path(sys.argv[1]))
