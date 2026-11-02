/* Independent validation utility. Build against libbluray, never shipped. */
#include <libbluray/bluray.h>
#include <libbluray/keys.h>
#include <libbluray/log_control.h>
#include <libbluray/overlay.h>
#include <libbluray/filesystem.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static unsigned draws;
static void overlay(void *handle, const BD_OVERLAY *o) {
    (void)handle;
    if (o && o->cmd == BD_OVERLAY_DRAW) ++draws;
}

int main(int argc, char **argv) {
    if (argc < 3) return 2;
    bd_set_debug_mask(0);
    BLURAY *bd = bd_open(argv[1], NULL);
    if (!bd) { fprintf(stderr, "bd_open failed\n"); return 1; }
    const BLURAY_DISC_INFO *info = bd_get_disc_info(bd);
    if (!info || !info->bluray_detected || !info->first_play_supported) return 3;
    if (!strcmp(argv[2], "large")) {
        BD_FILE_H *f = bd_open_file_dec(bd, "BDMV/AUXDATA/large.bin");
        if (!f) return 10;
        char marker[14];
        if (f->seek(f, ((int64_t)1 << 32) + 4096, SEEK_SET) < 0 ||
            f->read(f, (uint8_t *)marker, sizeof(marker)) != sizeof(marker) ||
            memcmp(marker, "EASYDVD-LARGE!", sizeof(marker))) return 11;
        f->close(f);
        puts("PASS: marker read beyond 4 GiB through libbluray's UDF reader");
        bd_close(bd);
        return 0;
    }
    if (!strcmp(argv[2], "seek")) {
        unsigned count = argc > 3 ? (unsigned)atoi(argv[3]) : 1;
        bd_get_titles(bd, 0, 0);
        for (unsigned n = 1; n <= count; ++n) {
            BLURAY_TITLE_INFO *t = bd_get_playlist_info(bd, n, 0);
            if (!t || !bd_select_playlist(bd, n) || bd_seek_time(bd, t->duration / 2) < 0) return 12;
            unsigned char buf[6144];
            if (bd_read(bd, buf, sizeof(buf)) <= 0) return 13;
            bd_free_title_info(t);
        }
        puts("PASS: title seeks returned media");
        bd_close(bd);
        return 0;
    }
    if (!strcmp(argv[2], "inspect")) {
        unsigned count = argc > 3 ? (unsigned)atoi(argv[3]) : info->num_hdmv_titles;
        bd_get_titles(bd, 0, 0);
        for (unsigned n = 1; n <= count; ++n) {
            BLURAY_TITLE_INFO *t = bd_get_playlist_info(bd, n, 0);
            if (!t || !t->clip_count) return 4;
            BLURAY_CLIP_INFO *c = &t->clips[0];
            if (!c->video_stream_count) return 5;
            BLURAY_STREAM_INFO *v = &c->video_streams[0];
            printf("{\"playlist\":%u,\"seconds\":%.6f,\"clips\":%u,\"format\":%u,\"rate\":%u,\"aspect\":%u,\"audio\":%u}\n",
                   n, t->duration / 90000.0, t->clip_count, v->format, v->rate, v->aspect,
                   c->audio_stream_count ? c->audio_streams[0].format : 0);
            bd_free_title_info(t);
        }
        bd_close(bd);
        return 0;
    }
    unsigned keys[256], nkeys = 0, next = 0;
    if (argc > 3) {
        char *token = strtok(argv[3], ",");
        while (token && nkeys < 256) { keys[nkeys++] = (unsigned)atoi(token); token = strtok(NULL, ","); }
    }
    bd_register_overlay_proc(bd, NULL, overlay);
    if (!bd_play(bd)) return 6;
    unsigned idle = 0, bytes = 0, last = 0;
    unsigned char buffer[6144 * 16];
    BD_EVENT ev;
    for (unsigned i = 0; i < 100000 && bytes < 256 * 1024 * 1024; ++i) {
        int n = bd_read_ext(bd, buffer, sizeof(buffer), &ev);
        if (n < 0 || ev.event == BD_EVENT_ERROR) { fprintf(stderr, "navigation error\n"); return 7; }
        bytes += n;
        if (ev.event == BD_EVENT_PLAYLIST) {
            printf("{\"playlist\":%u}\n", ev.param);
            last = ev.param;
        }
        if (ev.event == BD_EVENT_END_OF_TITLE) printf("{\"end\":%u}\n", last);
        int waiting = !n && (ev.event == BD_EVENT_NONE || ev.event == BD_EVENT_STILL ||
                            ev.event == BD_EVENT_STILL_TIME || ev.event == BD_EVENT_IDLE ||
                            ev.event == BD_EVENT_END_OF_TITLE);
        idle = waiting ? idle + 1 : 0;
        if (idle > 30 && next < nkeys) {
            if (!bd_user_input(bd, -1, keys[next++])) return 8;
            idle = 0;
        } else if (idle > 60) break;
    }
    printf("{\"keys\":%u,\"draws\":%u}\n", next, draws);
    bd_close(bd);
    return next == nkeys ? 0 : 9;
}
