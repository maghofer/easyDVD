"""Exercise a locally installed libVLC with HDMV menus, without GUI preferences.

Usage: python bluray-vlc-check.py --vlc <VLC directory> --disc <ISO or BDMV root>
       --output <scratch directory>
The disc is opened read-only; logs and screenshots are written to output.
"""
import argparse
import ctypes as c
import json
import os
from pathlib import Path
import time


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--vlc', type=Path, required=True)
    parser.add_argument('--disc', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    dll_dir = os.add_dll_directory(str(args.vlc.resolve()))
    os.environ['VLC_PLUGIN_PATH'] = str(args.vlc / 'plugins')
    lib = c.CDLL(str(args.vlc / 'libvlc.dll'))

    def bind(name, restype, *types):
        f = getattr(lib, name)
        f.restype, f.argtypes = restype, list(types)
        return f

    ptr, integer, text = c.c_void_p, c.c_int, c.c_char_p
    new = bind('libvlc_new', ptr, integer, c.POINTER(text))
    create_media = bind('libvlc_media_new_location', ptr, ptr, text)
    create_player = bind('libvlc_media_player_new_from_media', ptr, ptr)
    play = bind('libvlc_media_player_play', integer, ptr)
    state = bind('libvlc_media_player_get_state', integer, ptr)
    title = bind('libvlc_media_player_get_title', integer, ptr)
    clock = bind('libvlc_media_player_get_time', c.c_int64, ptr)
    navigate = bind('libvlc_media_player_navigate', None, ptr, c.c_uint)
    snapshot = bind('libvlc_video_take_snapshot', integer, ptr, c.c_uint, text, c.c_uint, c.c_uint)
    options = [b'--ignore-config', b'--no-audio', b'--no-video-title-show',
               b'--vout=dummy', b'--aout=dummy', b'--no-osd', b'--intf=dummy',
               b'--bluray-menu', b'--avcodec-hw=none', b'--file-logging', b'--verbose=2',
               ('--logfile=' + str(args.output / 'vlc.log')).encode()]
    instance = new(len(options), (text * len(options))(*options))
    if not instance:
        raise RuntimeError('libVLC initialization failed')
    mrl = ('bluray:///' + args.disc.resolve().as_posix()).encode()
    media = create_media(instance, mrl)
    bind('libvlc_media_add_option', None, ptr, text)(media, b':avcodec-hw=none')
    player = create_player(media)
    # Memory output gives the renderer a software surface for IG blending.
    pixels = c.create_string_buffer(960 * 540 * 4)
    lock_type = c.CFUNCTYPE(ptr, ptr, c.POINTER(ptr))
    @lock_type
    def lock(_opaque, planes):
        planes[0] = c.addressof(pixels)
        return None
    bind('libvlc_video_set_callbacks', None, ptr, lock_type, ptr, ptr, ptr)(player, lock, None, None, None)
    bind('libvlc_video_set_format', None, ptr, text, c.c_uint, c.c_uint, c.c_uint)(player, b'RV32', 960, 540, 960 * 4)
    if play(player) != 0:
        raise RuntimeError('libVLC could not start playback')
    events = []
    try:
        for label, key in [('menu', None), ('select-title-2', 2),
                           ('activate-title-2', 0), ('return-menu', None)]:
            if key is not None:
                navigate(player, key)
            time.sleep(1.5 if label != 'return-menu' else 3)
            record = dict(step=label, state=state(player), title=title(player),
                          time_ms=clock(player))
            frame = pixels.raw
            (args.output / (label + '.bgr0')).write_bytes(frame)
            record['red_pixels'] = sum(value > 180 for value in frame[2::4])
            record['snapshot_result'] = snapshot(player, 0,
                str(args.output / (label + '.png')).encode(), 960, 540)
            events.append(record)
            print(json.dumps(record), flush=True)
            time.sleep(.5)  # Snapshot encoding finishes on VLC's output thread.
        (args.output / 'result.json').write_text(json.dumps(events, indent=2))
        if any(e['state'] == 7 for e in events):
            raise RuntimeError('libVLC reported a playback error; inspect vlc.log')
    finally:
        bind('libvlc_media_player_stop', None, ptr)(player)
        bind('libvlc_media_player_release', None, ptr)(player)
        bind('libvlc_media_release', None, ptr)(media)
        bind('libvlc_release', None, ptr)(instance)
        dll_dir.close()


if __name__ == '__main__':
    main()
