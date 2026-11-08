#include "proto.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "assets.h"
#include "board.h"
#include "esp_app_desc.h"
#include "esp_heap_caps.h"
#include "esp_log.h"
#include "esp_lvgl_port.h"
#include "freertos/task.h"
#include "store.h"
#include "ui.h"
#include "usb.h"

static const char *TAG = "proto";

static void send_json(cJSON *o)
{
    char *s = cJSON_PrintUnformatted(o);
    if (s) {
        usb_send_line(s);
        free(s);
    }
    cJSON_Delete(o);
}

void proto_hello(void)
{
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", "hello");
    cJSON_AddStringToObject(o, "fw", esp_app_get_description()->version); // git describe at build
    cJSON_AddNumberToObject(o, "proto", 1);
    cJSON_AddStringToObject(o, "cfg", store_sha());
    cJSON_AddNumberToObject(o, "w", 360);
    cJSON_AddNumberToObject(o, "h", 360);
    cJSON_AddStringToObject(o, "touch", touch_info);
    send_json(o);
}

void proto_view(const char *profile, const char *page, const char *kind, bool awake)
{
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", "view");
    cJSON_AddStringToObject(o, "profile", profile);
    cJSON_AddStringToObject(o, "page", page);
    cJSON_AddStringToObject(o, "kind", kind);
    cJSON_AddBoolToObject(o, "awake", awake);
    send_json(o);
}

void proto_action(const char *profile, const char *page, const char *btn, bool hold, cJSON *action)
{
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", "action");
    cJSON_AddStringToObject(o, "profile", profile);
    cJSON_AddStringToObject(o, "page", page);
    cJSON_AddStringToObject(o, "btn", btn ? btn : "");
    cJSON_AddBoolToObject(o, "hold", hold);
    cJSON_AddItemToObject(o, "action", cJSON_Duplicate(action, true));
    send_json(o);
}

void proto_dial(const char *source, int value)
{
    char buf[80];
    snprintf(buf, sizeof(buf), "{\"t\":\"dial\",\"source\":\"%s\",\"value\":%d}", source, value);
    usb_send_line(buf);
}

void proto_log(const char *msg)
{
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", "log");
    cJSON_AddStringToObject(o, "msg", msg);
    send_json(o);
}

static void ack(const char *id, bool ok, const char *err)
{
    if (!id) return;
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", "ack");
    cJSON_AddStringToObject(o, "id", id);
    cJSON_AddBoolToObject(o, "ok", ok);
    if (err) cJSON_AddStringToObject(o, "err", err);
    send_json(o);
}

// ---- album art: chunks of base64 RGB565 into two alternating PSRAM buffers ----

#define COVER_MAX (360 * 360 * 2)
static uint8_t *cover_buf[2];
static int cover_back;          // buffer being filled
static int cover_seq = -1;
static size_t cover_len;

static int b64val(char c)
{
    if (c >= 'A' && c <= 'Z') return c - 'A';
    if (c >= 'a' && c <= 'z') return c - 'a' + 26;
    if (c >= '0' && c <= '9') return c - '0' + 52;
    if (c == '+') return 62;
    if (c == '/') return 63;
    return -1;
}

static size_t b64decode(const char *in, uint8_t *out, size_t cap)
{
    size_t n = 0;
    uint32_t acc = 0;
    int bits = 0;
    for (; *in; in++) {
        int v = b64val(*in);
        if (v < 0) continue; // padding / whitespace
        acc = (acc << 6) | (uint32_t)v;
        bits += 6;
        if (bits >= 8) {
            bits -= 8;
            if (n < cap) out[n++] = (uint8_t)(acc >> bits);
        }
    }
    return n;
}

static void handle_game_art(cJSON *m);
static void handle_cover(cJSON *m);

static size_t b64encode(const uint8_t *in, size_t n, char *out)
{
    static const char T[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    size_t o = 0;
    for (size_t i = 0; i < n; i += 3) {
        uint32_t v = in[i] << 16 | (i + 1 < n ? in[i + 1] << 8 : 0) | (i + 2 < n ? in[i + 2] : 0);
        out[o++] = T[v >> 18 & 63];
        out[o++] = T[v >> 12 & 63];
        out[o++] = i + 1 < n ? T[v >> 6 & 63] : '=';
        out[o++] = i + 2 < n ? T[v & 63] : '=';
    }
    out[o] = 0;
    return o;
}

// {"t":"screenshot"}: the active screen plus overlays as RGB565, in chunks.
static void send_screenshot(void)
{
    lvgl_port_lock(0);
    lv_draw_buf_t *shot = lv_snapshot_take(lv_screen_active(), LV_COLOR_FORMAT_RGB565);
    lv_draw_buf_t *top = lv_snapshot_take(lv_layer_top(), LV_COLOR_FORMAT_ARGB8888);
    lvgl_port_unlock();
    if (!shot) return;
    int w = shot->header.w, h = shot->header.h;
    if (top && top->header.w == w && top->header.h == h) { // blend panels / toasts over the page
        for (int y = 0; y < h; y++) {
            uint16_t *d = (uint16_t *)(shot->data + y * shot->header.stride);
            const uint8_t *t = top->data + y * top->header.stride;
            for (int x = 0; x < w; x++) {
                uint8_t a = t[4 * x + 3];
                if (!a) continue;
                uint16_t p = d[x];
                int r = (p >> 11) << 3, g = ((p >> 5) & 63) << 2, b = (p & 31) << 3;
                r = (t[4 * x + 2] * a + r * (255 - a)) / 255;
                g = (t[4 * x + 1] * a + g * (255 - a)) / 255;
                b = (t[4 * x + 0] * a + b * (255 - a)) / 255;
                d[x] = (uint16_t)((r >> 3) << 11 | (g >> 2) << 5 | (b >> 3));
            }
        }
    }
    const size_t raw_chunk = 24 * 1024;
    size_t total = (size_t)h * shot->header.stride;
    int n = (int)((total + raw_chunk - 1) / raw_chunk);
    char *line = heap_caps_malloc(raw_chunk * 4 / 3 + 256, MALLOC_CAP_SPIRAM);
    for (int i = 0; line && i < n; i++) {
        size_t len = LV_MIN(raw_chunk, total - i * raw_chunk);
        int o = snprintf(line, 160, "{\"t\":\"shot\",\"w\":%d,\"h\":%d,\"stride\":%d,\"n\":%d,\"i\":%d,\"data\":\"", w, h,
                         (int)shot->header.stride, n, i);
        o += b64encode(shot->data + i * raw_chunk, len, line + o);
        strcpy(line + o, "\"}");
        usb_send_line(line);
    }
    free(line);
    lv_draw_buf_destroy(shot);
    if (top) lv_draw_buf_destroy(top);
}

static const char *str(cJSON *o, const char *k) { return cJSON_GetStringValue(cJSON_GetObjectItem(o, k)); }
static int num(cJSON *o, const char *k, int def)
{
    cJSON *v = cJSON_GetObjectItem(o, k);
    return cJSON_IsNumber(v) ? v->valueint : def;
}
static bool flag(cJSON *o, const char *k) { return cJSON_IsTrue(cJSON_GetObjectItem(o, k)); }

static void handle(cJSON *m)
{
    const char *t = str(m, "t");
    const char *id = str(m, "id");
    if (!t) return;

    if (!strcmp(t, "hello")) {
        proto_hello();
        lvgl_port_lock(0);
        ui_announce();
        lvgl_port_unlock();
    } else if (!strcmp(t, "config")) {
        cJSON *cfg = cJSON_GetObjectItem(m, "cfg");
        const char *sha = str(m, "sha");
        char *raw = cfg ? cJSON_PrintUnformatted(cfg) : NULL;
        bool ok = raw && sha && store_save(raw, sha);
        free(raw);
        ack(id, ok, ok ? NULL : "invalid config");
        if (ok) {
            lvgl_port_lock(0);
            ui_reload();
            lvgl_port_unlock();
        }
    } else if (!strcmp(t, "profile")) {
        lvgl_port_lock(0);
        ui_set_profile(str(m, "id"));
        lvgl_port_unlock();
    } else if (!strcmp(t, "show")) {
        lvgl_port_lock(0);
        ui_show(str(m, "profile"), str(m, "page"));
        lvgl_port_unlock();
    } else if (!strcmp(t, "app")) {
        const char *name = str(m, "name");
        if (name) {
            lvgl_port_lock(0);
            ui_toast(name);
            lvgl_port_unlock();
        }
    } else if (!strcmp(t, "media")) {
        lvgl_port_lock(0);
        ui_media(str(m, "title"), str(m, "artist"), str(m, "player"), flag(m, "playing"));
        lvgl_port_unlock();
    } else if (!strcmp(t, "volume") || !strcmp(t, "mic")) {
        lvgl_port_lock(0);
        ui_level(t, num(m, "value", 0), flag(m, "muted"));
        lvgl_port_unlock();
    } else if (!strcmp(t, "stats")) {
        lvgl_port_lock(0);
        ui_stats(m);
        lvgl_port_unlock();
    } else if (!strcmp(t, "time") || !strcmp(t, "weather")) {
        lvgl_port_lock(0);
        if (t[0] == 't') ui_time(m);
        else ui_weather(m);
        lvgl_port_unlock();
    } else if (!strcmp(t, "icon")) {
        static uint8_t *ibuf;
        if (!ibuf) ibuf = heap_caps_malloc(128 * 128 * 3, MALLOC_CAP_SPIRAM); // app icons 48², avatars 96² (RGB565A8)
        const char *data = str(m, "data");
        size_t n = ibuf && data ? b64decode(data, ibuf, 128 * 128 * 3) : 0;
        lvgl_port_lock(0);
        ui_icon(str(m, "name"), num(m, "w", 0), num(m, "h", 0), ibuf, n);
        lvgl_port_unlock();
    } else if (!strcmp(t, "windows") || !strcmp(t, "clips") || !strcmp(t, "notifs")) {
        cJSON *list = cJSON_GetObjectItem(m, "list");
        lvgl_port_lock(0);
        if (t[0] == 'w') ui_windows(list);
        else if (t[0] == 'c') ui_clips(list);
        else ui_notifs(list);
        lvgl_port_unlock();
    } else if (!strcmp(t, "discord")) {
        lvgl_port_lock(0);
        ui_discord(cJSON_GetObjectItem(m, "state"));
        lvgl_port_unlock();
    } else if (!strcmp(t, "mixer") || !strcmp(t, "notif")) {
        lvgl_port_lock(0);
        if (!strcmp(t, "mixer")) ui_mixer(m);
        else ui_notif_toast(m);
        lvgl_port_unlock();
    } else if (!strcmp(t, "assetsWant")) {
        cJSON *o = cJSON_CreateObject();
        cJSON_AddStringToObject(o, "t", "assets");
        cJSON_AddItemToObject(o, "have", assets_list());
        cJSON_AddNumberToObject(o, "free", (double)assets_free());
        send_json(o);
    } else if (!strcmp(t, "asset")) {
        static uint8_t *chunk;
        if (!chunk) chunk = heap_caps_malloc(33 * 1024, MALLOC_CAP_SPIRAM);
        const char *data = str(m, "data");
        size_t n = chunk && data ? b64decode(data, chunk, 33 * 1024) : 0;
        bool ok = n > 0 && assets_write(str(m, "name"), num(m, "i", 0), num(m, "n", 1), chunk, n);
        ack(id, ok, ok ? NULL : "write failed");
    } else if (!strcmp(t, "assetsPrune")) {
        cJSON *keep = cJSON_GetObjectItem(m, "keep");
        if (cJSON_IsArray(keep)) assets_prune(keep);
    } else if (!strcmp(t, "assetsDone")) {
        lvgl_port_lock(0);
        ui_assets_changed();
        lvgl_port_unlock();
    } else if (!strcmp(t, "gameArt")) {
        handle_game_art(m);
    } else if (!strcmp(t, "mangohud")) {
        lvgl_port_lock(0);
        ui_mangohud(m);
        lvgl_port_unlock();
    } else if (!strcmp(t, "obs") || !strcmp(t, "game")) {
        lvgl_port_lock(0);
        if (t[0] == 'o') ui_obs(m);
        else ui_game(m);
        lvgl_port_unlock();
    } else if (!strcmp(t, "cover")) {
        handle_cover(m);
    } else if (!strcmp(t, "mediaTheme")) {
        cJSON *pal = cJSON_GetObjectItem(m, "palette");
        lvgl_port_lock(0);
        ui_media_theme(cJSON_IsObject(pal) ? pal : NULL);
        lvgl_port_unlock();
    } else if (!strcmp(t, "hid")) {
        if (str(m, "keys") && num(m, "hold", 0) > 0) usb_hid_hold(str(m, "keys"), num(m, "hold", 0), id);
        else if (str(m, "keys")) usb_hid(HID_KEYS, str(m, "keys"), id);
        else if (str(m, "text")) usb_hid(HID_TEXT, str(m, "text"), id);
        else if (str(m, "media")) usb_hid(HID_MEDIA, str(m, "media"), id);
        else ack(id, false, "nothing to type");
    } else if (!strcmp(t, "perf")) {
        lvgl_port_lock(0);
        ui_perf(num(m, "ms", 2000));
        lvgl_port_unlock();
    } else if (!strcmp(t, "screenshot")) {
        send_screenshot();
    } else if (!strcmp(t, "bootloader")) {
        vTaskDelay(pdMS_TO_TICKS(100));
        usb_reboot_to_bootloader();
    }
}

static void handle_cover(cJSON *m)
{
    int seq = num(m, "seq", 0), n = num(m, "n", 0), i = num(m, "i", 0);
    int w = num(m, "w", 360), h = num(m, "h", 360);
    if (n == 0) {
        cover_seq = -1;
        lvgl_port_lock(0);
        ui_cover(NULL, 0, 0);
        lvgl_port_unlock();
        return;
    }
    if (w * h * 2 > COVER_MAX) return;
    if (!cover_buf[0]) {
        cover_buf[0] = heap_caps_malloc(COVER_MAX, MALLOC_CAP_SPIRAM);
        cover_buf[1] = heap_caps_malloc(COVER_MAX, MALLOC_CAP_SPIRAM);
        if (!cover_buf[0] || !cover_buf[1]) return;
    }
    if (i == 0) {
        cover_seq = seq;
        cover_len = 0;
    }
    if (seq != cover_seq) return; // a chunk from an older image
    const char *data = str(m, "data");
    if (data) cover_len += b64decode(data, cover_buf[cover_back] + cover_len, COVER_MAX - cover_len);
    if (i == n - 1 && cover_len == (size_t)(w * h * 2)) {
        lvgl_port_lock(0);
        ui_cover(cover_buf[cover_back], w, h);
        lvgl_port_unlock();
        cover_back ^= 1; // the shown buffer stays untouched until the next image
    }
}

// Steam library art for game pages: same chunk format as covers, own buffers.
static uint8_t *game_buf[2];
static int game_back, game_seq = -1;
static size_t game_len;

static void handle_game_art(cJSON *m)
{
    int seq = num(m, "seq", 0), n = num(m, "n", 0), i = num(m, "i", 0);
    int w = num(m, "w", 360), h = num(m, "h", 360);
    if (n == 0 || w * h * 2 > COVER_MAX) return;
    if (!game_buf[0]) {
        game_buf[0] = heap_caps_malloc(COVER_MAX, MALLOC_CAP_SPIRAM);
        game_buf[1] = heap_caps_malloc(COVER_MAX, MALLOC_CAP_SPIRAM);
        if (!game_buf[0] || !game_buf[1]) return;
    }
    if (i == 0) {
        game_seq = seq;
        game_len = 0;
    }
    if (seq != game_seq) return;
    const char *data = str(m, "data");
    if (data) game_len += b64decode(data, game_buf[game_back] + game_len, COVER_MAX - game_len);
    if (i == n - 1 && game_len == (size_t)(w * h * 2)) {
        lvgl_port_lock(0);
        ui_game_art(game_buf[game_back], w, h);
        lvgl_port_unlock();
        game_back ^= 1;
    }
}

static void proto_task(void *arg)
{
    QueueHandle_t q = arg;
    char *line;
    for (;;) {
        if (xQueueReceive(q, &line, portMAX_DELAY) != pdTRUE) continue;
        cJSON *m = cJSON_Parse(line);
        free(line);
        if (!m) {
            ESP_LOGW(TAG, "bad line");
            continue;
        }
        handle(m);
        cJSON_Delete(m);
    }
}

void proto_start(QueueHandle_t lines)
{
    xTaskCreatePinnedToCore(proto_task, "proto", 8192, lines, 4, NULL, 0);
}
