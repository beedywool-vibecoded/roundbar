// Material 3 Expressive UI for the 360x360 round screen.
//
// One LVGL screen per profile. Its pages live in a horizontal tileview (swipe
// to change page). Buttons morph circle -> rounded square while pressed and
// spring back with an overshoot; value pages use a 270° arc along the bezel.
#include "ui.h"

#include <math.h>
#include <stdio.h>
#include "esp_timer.h"
#include <time.h>
#include <string.h>

#include "assets.h"
#include "jpeg_decoder.h"
#include "esp_heap_caps.h"
#include "board.h"
#include "proto.h"
#include "store.h"
#include "theme.h"
#include "usb.h"

#define MAX_PAGES     8
#define MAX_PROFILES  32
#define ARC_SIZE      348
#define ARC_START     135
#define ARC_END       45
#define LABEL_SHOW_MS 900

static lv_display_t *disp;
static lv_indev_t *indev;

static char cur_profile[25] = "default";
static cJSON *prof;   // current profile object inside store_config()
static lv_obj_t *scr, *tv, *dots_box, *bubble, *host_dot;
static lv_obj_t *dots[MAX_PAGES];
static int page_count, cur_page;
static cJSON *pages[MAX_PAGES];

static struct {
    char id[25];
    int page;
} page_mem[MAX_PROFILES];

// Widgets that the PC updates. Reset on every rebuild.
#define MAX_LIVE 8
static struct {
    lv_obj_t *vol_arc[MAX_LIVE], *vol_lbl[MAX_LIVE];
    int nvol;
    lv_obj_t *mic_arc[MAX_LIVE], *mic_lbl[MAX_LIVE];
    int nmic;
    struct {
        lv_obj_t *title, *artist, *player, *play, *play_icon, *cover;
        uint8_t play_shape, playing_shape;
    } media[MAX_LIVE];
    int nmedia;
    struct {
        lv_obj_t *arc, *val;
        char key[10];
    } stat[MAX_PAGES * 4];
    int nstat;
} live;

// Memory owned by a screen (image files), freed when that screen is deleted.
typedef struct {
    void *bufs[64];
    lv_image_dsc_t dscs[64];
    int n;
} screen_res_t;
static screen_res_t *res; // of the screen being built / shown

static struct {
    lv_obj_t *gif[MAX_PAGES];         // per page, paused when not visible
} media_live;

static bool autoswitch_paused;
static lv_obj_t *pause_badge, *qs;
static void qs_open(void);
static void screen_off(void);
static void goto_index(int idx, bool wrap_anim);
static void update_dots(void);
static void wake(void);

static int vol = 50, mic = 50;
static bool vol_muted, mic_muted;
static bool dragging;
static char m_title[128], m_artist[128], m_player[48];
static palette_t PM;        // media-page palette from the album art
static bool pm_active;
static lv_image_dsc_t cover_dsc;
static bool cover_set;
static const uint8_t *cover_src; // as received (RGB565)
static uint8_t *cover_blend;     // with the readability scrim baked in
static int cover_w, cover_h;
static uint32_t cover_key;

static int cover_scrim_pct(void)
{
    cJSON *opa = cJSON_GetObjectItem(cJSON_GetObjectItem(store_config(), "media"), "coverOpacity");
    return 100 - (cJSON_IsNumber(opa) ? LV_CLAMP(0, opa->valueint, 100) : 50);
}

static void game_session_update(void);
static void game_place_name(void);
// out = in blended toward `surface` by pct %, in RGB565.
static void blend_scrim(const uint16_t *in, uint16_t *out, size_t n, lv_color_t surface, int pct)
{
    int a = pct * 256 / 100, ia = 256 - a;
    int sr = surface.red >> 3, sg = surface.green >> 2, sb = surface.blue >> 3;
    for (size_t i = 0; i < n; i++) {
        uint16_t v = in[i];
        int r = ((v >> 11) * ia + sr * a) >> 8, g = (((v >> 5) & 63) * ia + sg * a) >> 8, b = ((v & 31) * ia + sb * a) >> 8;
        out[i] = (uint16_t)(r << 11 | g << 5 | b);
    }
}

// Bakes the surface-colored scrim into the artwork once, so a swipe only copies
// one opaque image per frame instead of blending a full-screen layer on top.
static void cover_prepare(lv_color_t surface, int scrim_pct)
{
    if (!cover_src) return;
    uint32_t key = (uint32_t)(uintptr_t)cover_src ^ lv_color_to_u32(surface) * 31u ^ (uint32_t)scrim_pct << 24;
    if (key == cover_key && cover_dsc.data == cover_blend) return;
    size_t n = (size_t)cover_w * cover_h;
    if (!cover_blend) cover_blend = heap_caps_malloc(360 * 360 * 2, MALLOC_CAP_SPIRAM);
    if (!cover_blend || n > 360 * 360) return;
    blend_scrim((const uint16_t *)cover_src, (uint16_t *)cover_blend, n, surface, scrim_pct);
    cover_dsc.header.magic = LV_IMAGE_HEADER_MAGIC;
    cover_dsc.header.cf = LV_COLOR_FORMAT_RGB565;
    cover_dsc.header.w = cover_w;
    cover_dsc.header.h = cover_h;
    cover_dsc.header.stride = cover_w * 2;
    cover_dsc.data_size = (uint32_t)n * 2;
    cover_dsc.data = cover_blend;
    cover_key = key;
    lv_image_cache_drop(&cover_dsc); // same descriptor, new pixels
}
static bool m_playing;

typedef enum { AWAKE, DIM, ASLEEP, OFF } power_t; // OFF: quick settings / PC gone, double tap wakes
static power_t power = AWAKE;
static lv_obj_t *shield;

// ---- small helpers -------------------------------------------------------------

static const char *jstr(cJSON *o, const char *k) { return cJSON_GetStringValue(cJSON_GetObjectItem(o, k)); }

static lv_obj_t *box(lv_obj_t *parent)
{
    lv_obj_t *o = lv_obj_create(parent);
    lv_obj_remove_style_all(o);
    lv_obj_remove_flag(o, LV_OBJ_FLAG_SCROLLABLE);
    return o;
}

static lv_obj_t *text(lv_obj_t *parent, const lv_font_t *f, lv_color_t c, const char *s)
{
    lv_obj_t *l = lv_label_create(parent);
    lv_obj_set_style_text_font(l, f, 0);
    lv_obj_set_style_text_color(l, c, 0);
    lv_obj_set_style_text_align(l, LV_TEXT_ALIGN_CENTER, 0);
    lv_label_set_text(l, s ? s : "");
    return l;
}

static const char *page_kind(cJSON *pg)
{
    const char *t = jstr(pg, "type");
    return t ? t : "radial";
}

static void remember_page(void)
{
    int free_slot = -1;
    for (int i = 0; i < MAX_PROFILES; i++) {
        if (!strcmp(page_mem[i].id, cur_profile)) {
            page_mem[i].page = cur_page;
            return;
        }
        if (!page_mem[i].id[0] && free_slot < 0) free_slot = i;
    }
    if (free_slot >= 0) {
        strlcpy(page_mem[free_slot].id, cur_profile, sizeof(page_mem[0].id));
        page_mem[free_slot].page = cur_page;
    }
}

static int recalled_page(void)
{
    for (int i = 0; i < MAX_PROFILES; i++)
        if (!strcmp(page_mem[i].id, cur_profile)) return page_mem[i].page;
    return 0;
}

void ui_announce(void)
{
    if (!page_count) return;
    proto_view(cur_profile, jstr(pages[cur_page], "id") ?: "", page_kind(pages[cur_page]), power != ASLEEP);
}

// ---- label bubble & toast ---------------------------------------------------------

static lv_timer_t *bubble_timer; // one persistent timer, paused between uses

static void bubble_hide(lv_timer_t *t)
{
    if (bubble) lv_obj_fade_out(bubble, 200, 0);
    lv_timer_pause(t);
}

static void show_bubble(const char *s)
{
    if (!bubble || !s || !s[0]) return;
    lv_label_set_text(lv_obj_get_child(bubble, 0), s);
    lv_obj_fade_in(bubble, 120, 0);
    lv_timer_reset(bubble_timer);
    lv_timer_resume(bubble_timer);
}

static lv_obj_t *toast;
void ui_toast(const char *s)
{
    if (!toast) return;
    lv_label_set_text(lv_obj_get_child(toast, 0), s);
    lv_obj_fade_in(toast, 160, 0);
    lv_obj_fade_out(toast, 260, 1400);
}

static lv_obj_t *pill(lv_obj_t *parent, lv_color_t bg, lv_color_t fg, const lv_font_t *f)
{
    lv_obj_t *p = box(parent);
    lv_obj_set_size(p, LV_SIZE_CONTENT, LV_SIZE_CONTENT);
    lv_obj_set_style_bg_color(p, bg, 0);
    lv_obj_set_style_bg_opa(p, LV_OPA_COVER, 0);
    lv_obj_set_style_radius(p, LV_RADIUS_CIRCLE, 0);
    lv_obj_set_style_pad_hor(p, 16, 0);
    lv_obj_set_style_pad_ver(p, 8, 0);
    lv_obj_remove_flag(p, LV_OBJ_FLAG_CLICKABLE);
    text(p, f, fg, "");
    lv_obj_set_style_opa(p, LV_OPA_TRANSP, 0);
    return p;
}

// ---- actions -------------------------------------------------------------------

static void goto_page(const char *id)
{
    for (int i = 0; i < page_count; i++) {
        const char *pid = jstr(pages[i], "id");
        if (pid && id && !strcmp(pid, id)) {
            lv_tileview_set_tile_by_index(tv, i, 0, LV_ANIM_ON);
            return;
        }
    }
}

static void run_action(cJSON *a, cJSON *btn, bool hold)
{
    const char *type = jstr(a, "type");
    if (!type) return;
    if (!strcmp(type, "keys")) usb_hid(HID_KEYS, jstr(a, "keys"), NULL);
    else if (!strcmp(type, "text")) usb_hid(HID_TEXT, jstr(a, "text"), NULL);
    else if (!strcmp(type, "media")) usb_hid(HID_MEDIA, jstr(a, "cmd"), NULL);
    else if (!strcmp(type, "page")) goto_page(jstr(a, "page"));
    else if (usb_host_listening())
        proto_action(cur_profile, jstr(pages[cur_page], "id") ?: "", btn ? jstr(btn, "id") : "", hold, a);
    else show_bubble("PC app not running");
}

// ---- buttons with the expressive press morph ----------------------------------------

static void btn_event(lv_event_t *e)
{
    cJSON *b = lv_event_get_user_data(e);
    lv_event_code_t code = lv_event_get_code(e);
    if (code == LV_EVENT_SHORT_CLICKED) {
        show_bubble(jstr(b, "label"));
        run_action(cJSON_GetObjectItem(b, "tap"), b, false);
    } else if (code == LV_EVENT_LONG_PRESSED) {
        cJSON *h = cJSON_GetObjectItem(b, "hold");
        if (h) {
            show_bubble(jstr(b, "label"));
            run_action(h, b, true);
        }
    }
}

// Pressing only squishes and lightens; the shape stays (user preference).
static const lv_style_prop_t press_props[] = {LV_STYLE_TRANSFORM_SCALE_X, LV_STYLE_TRANSFORM_SCALE_Y, LV_STYLE_BG_COLOR,
                                              LV_STYLE_IMAGE_RECOLOR, 0};
static lv_style_transition_dsc_t tr_press, tr_release;

static void squish_on_press(lv_obj_t *o, int size)
{
    lv_obj_set_style_transform_pivot_x(o, size / 2, 0);
    lv_obj_set_style_transform_pivot_y(o, size / 2, 0);
    lv_obj_set_style_transform_scale(o, 232, LV_STATE_PRESSED);
    lv_obj_set_style_transition(o, &tr_release, 0);
    lv_obj_set_style_transition(o, &tr_press, LV_STATE_PRESSED);
}

// ---- Material 3 Expressive shapes, rasterised once into A8 masks ----------------------

enum { SH_CIRCLE, SH_SQUARE, SH_COOKIE, SH_CLOVER, SH_SUNNY, SH_FLOWER, SH_BURST, SH_COUNT };
static const char *const SHAPE_NAMES[SH_COUNT] = {"circle", "square", "cookie", "clover", "sunny", "flower", "burst"};
static const float SHAPE_LOBES[SH_COUNT] = {0, 0, 9, 4, 8, 6, 12};
static const float SHAPE_DEPTH[SH_COUNT] = {0, 0, .07f, .17f, .10f, .13f, .05f};

static int shape_id(const char *name, int def)
{
    for (int i = 0; name && i < SH_COUNT; i++)
        if (!strcmp(name, SHAPE_NAMES[i])) return i;
    return def;
}

static struct {
    uint8_t shape;
    uint8_t size;
    lv_draw_buf_t *buf;
} shape_cache[SH_COUNT * 6];
static int nshape_cache;

static const lv_draw_buf_t *shape_mask(int shape, int size)
{
    for (int i = 0; i < nshape_cache; i++)
        if (shape_cache[i].shape == shape && shape_cache[i].size == size) return shape_cache[i].buf;
    lv_draw_buf_t *buf = lv_draw_buf_create(size, size, LV_COLOR_FORMAT_A8, LV_STRIDE_AUTO);
    if (!buf) return NULL;
    const float c = size / 2.0f, lobes = SHAPE_LOBES[shape], depth = SHAPE_DEPTH[shape];
    for (int y = 0; y < size; y++) {
        uint8_t *row = buf->data + y * buf->header.stride;
        for (int x = 0; x < size; x++) {
            float fx = x + 0.5f - c, fy = y + 0.5f - c;
            float d = sqrtf(fx * fx + fy * fy), r = c;
            if (d > 0.001f) {
                if (shape == SH_SQUARE) { // squircle |x|^4 + |y|^4 = c^4
                    float co = fx / d, si = fy / d;
                    r = c / sqrtf(sqrtf(co * co * co * co + si * si * si * si));
                } else if (lobes > 0) {
                    r = c / (1 + depth) * (1 + depth * cosf(lobes * atan2f(fy, fx)));
                }
            }
            float cov = r - d + 0.5f; // one-pixel analytic anti-aliasing
            row[x] = cov <= 0 ? 0 : cov >= 1 ? 255 : (uint8_t)(cov * 255);
        }
    }
    if (nshape_cache < (int)(sizeof(shape_cache) / sizeof(shape_cache[0]))) {
        shape_cache[nshape_cache].shape = shape;
        shape_cache[nshape_cache].size = size;
        shape_cache[nshape_cache++].buf = buf;
    }
    return buf;
}

// A clickable button drawn from a shape mask in `bg`, squishing when pressed.
static lv_obj_t *mask_button(lv_obj_t *parent, int shape, int size, lv_color_t bg)
{
    lv_obj_t *o = lv_image_create(parent);
    lv_image_set_src(o, shape_mask(shape, size));
    lv_obj_set_size(o, size, size);
    lv_obj_add_flag(o, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_remove_flag(o, LV_OBJ_FLAG_SCROLLABLE);
    lv_obj_set_style_image_recolor(o, bg, 0);
    lv_obj_set_style_image_recolor_opa(o, LV_OPA_COVER, 0);
    lv_obj_set_style_image_recolor(o, lv_color_mix(lv_color_white(), bg, 30), LV_STATE_PRESSED);
    squish_on_press(o, size);
    return o;
}

static lv_obj_t *config_button(lv_obj_t *parent, cJSON *b, int size, bool center, const lv_font_t *ifont)
{
    lv_color_t bg, fg;
    theme_role(jstr(b, "color"), &bg, &fg);
    lv_obj_t *o = mask_button(parent, shape_id(jstr(b, "shape"), center ? SH_SQUARE : SH_CIRCLE), size, bg);
    lv_obj_t *ic = text(o, ifont, fg, theme_icon(jstr(b, "icon")));
    lv_obj_center(ic);
    lv_obj_add_event_cb(o, btn_event, LV_EVENT_SHORT_CLICKED, b);
    lv_obj_add_event_cb(o, btn_event, LV_EVENT_LONG_PRESSED, b);
    return o;
}

// ---- page: radial ------------------------------------------------------------------

static void build_radial(lv_obj_t *tile, cJSON *pg)
{
    cJSON *btns = cJSON_GetObjectItem(pg, "buttons");
    int n = cJSON_GetArraySize(btns);
    if (n > 8) n = 8;
    int size = n <= 4 ? 92 : n <= 6 ? 82 : 72;
    int radius = n <= 4 ? 110 : n <= 6 ? 120 : 128;
    const lv_font_t *ifont = n <= 6 ? F.icon_m : F.icon_s;
    for (int i = 0; i < n; i++) {
        int16_t ang = (int16_t)(270 + i * 360 / n) % 360; // start at 12 o'clock
        int x = radius * lv_trigo_cos(ang) / LV_TRIGO_SIN_MAX;
        int y = radius * lv_trigo_sin(ang) / LV_TRIGO_SIN_MAX;
        lv_obj_t *o = config_button(tile, cJSON_GetArrayItem(btns, i), size, false, ifont);
        lv_obj_align(o, LV_ALIGN_CENTER, x, y);
    }
    cJSON *center = cJSON_GetObjectItem(pg, "center");
    if (center) {
        lv_obj_t *c = config_button(tile, center, 112, true, F.icon_l);
        lv_obj_center(c);
    } else {
        lv_obj_t *t = text(tile, F.title, P.on_sv, jstr(pg, "title"));
        lv_obj_set_width(t, 150);
        lv_obj_center(t);
    }
}

// ---- bezel arc ---------------------------------------------------------------------
//
// On a round screen a swipe naturally starts at the edge, i.e. on the arc. The
// first ~10 px decide: moving along the ring drags the value, moving across it
// horizontally scrolls the pages under the finger. Nothing changes until then.

typedef enum { ARC_IDLE, ARC_UNDECIDED, ARC_DRAG, ARC_SWIPE, ARC_NONE } arc_mode_t;
static arc_mode_t arc_mode;
static lv_point_t arc_p0;
static int32_t arc_v0, arc_scroll0;

static void arc_gate_event(lv_event_t *e)
{
    lv_obj_t *a = lv_event_get_target_obj(e);
    lv_event_code_t c = lv_event_get_code(e);
    lv_point_t p;
    lv_indev_get_point(lv_indev_active(), &p);
    if (c == LV_EVENT_PRESSED) {
        arc_mode = ARC_UNDECIDED;
        arc_p0 = p;
        arc_v0 = lv_arc_get_value(a);
        arc_scroll0 = lv_obj_get_scroll_x(tv);
        return;
    }
    int32_t dx = p.x - arc_p0.x, dy = p.y - arc_p0.y;
    if (c == LV_EVENT_PRESSING) {
        if (arc_mode == ARC_UNDECIDED) {
            if (dx * dx + dy * dy < 100) {
                lv_arc_set_value(a, arc_v0);
                return;
            }
            int32_t rx = arc_p0.x - LCD_W / 2, ry = arc_p0.y - LCD_H / 2;
            int32_t radial = dx * rx + dy * ry, tangential = dx * ry - dy * rx;
            bool across = LV_ABS(radial) > LV_ABS(tangential) && LV_ABS(dx) > LV_ABS(dy);
            if (across && dy > 0 && arc_p0.y < 70) arc_mode = ARC_NONE; // pull-down: quick settings
            else arc_mode = across && page_count > 1 ? ARC_SWIPE : ARC_DRAG;
        }
        if (arc_mode == ARC_NONE) {
            lv_arc_set_value(a, arc_v0);
            return;
        }
        if (arc_mode == ARC_SWIPE) {
            lv_arc_set_value(a, arc_v0);
            int32_t x = LV_CLAMP(0, arc_scroll0 - dx, (page_count - 1) * LCD_W);
            lv_obj_scroll_to_x(tv, x, LV_ANIM_OFF);
        }
        return;
    }
    if (c == LV_EVENT_RELEASED || c == LV_EVENT_PRESS_LOST) {
        if (arc_mode == ARC_SWIPE) {
            lv_arc_set_value(a, arc_v0);
            if (dx < -40) goto_index(cur_page + 1, true);
            else if (dx > 40) goto_index(cur_page - 1, true);
            else lv_tileview_set_tile_by_index(tv, cur_page, 0, LV_ANIM_ON);
        }
        arc_mode = ARC_IDLE;
    }
}

#define ARC_DRAGGING() (arc_mode == ARC_DRAG)

static lv_obj_t *bezel_arc(lv_obj_t *parent, lv_color_t ind, bool interactive)
{
    lv_obj_t *a = lv_arc_create(parent);
    lv_obj_set_size(a, ARC_SIZE, ARC_SIZE);
    lv_obj_center(a);
    lv_arc_set_bg_angles(a, ARC_START, ARC_END);
    lv_arc_set_range(a, 0, 100);
    lv_obj_set_style_arc_width(a, 14, LV_PART_MAIN);
    lv_obj_set_style_arc_width(a, 14, LV_PART_INDICATOR);
    lv_obj_set_style_arc_color(a, P.schh, LV_PART_MAIN);
    lv_obj_set_style_arc_color(a, ind, LV_PART_INDICATOR);
    lv_obj_set_style_arc_rounded(a, true, LV_PART_MAIN);
    lv_obj_set_style_arc_rounded(a, true, LV_PART_INDICATOR);
    if (interactive) {
        lv_obj_set_style_bg_color(a, P.on_primary, LV_PART_KNOB);
        lv_obj_set_style_border_color(a, ind, LV_PART_KNOB);
        lv_obj_set_style_border_width(a, 4, LV_PART_KNOB);
        lv_obj_set_style_pad_all(a, 3, LV_PART_KNOB);
        lv_arc_set_change_rate(a, 360);
        lv_obj_add_event_cb(a, arc_gate_event, LV_EVENT_PRESSED, NULL);
        lv_obj_add_event_cb(a, arc_gate_event, LV_EVENT_PRESSING, NULL);
        lv_obj_add_event_cb(a, arc_gate_event, LV_EVENT_RELEASED, NULL);
        lv_obj_add_event_cb(a, arc_gate_event, LV_EVENT_PRESS_LOST, NULL);
    } else {
        lv_obj_set_style_bg_opa(a, LV_OPA_TRANSP, LV_PART_KNOB);
        lv_obj_set_style_pad_all(a, 0, LV_PART_KNOB);
        lv_obj_remove_flag(a, LV_OBJ_FLAG_CLICKABLE);
    }
    return a;
}

// Volume throttle: the arc fires on every pixel, the PC gets ≤ 20 updates/s.
static int pending_vol = -1, pending_mic = -1;
static void level_flush(lv_timer_t *t)
{
    if (pending_vol >= 0) { proto_dial("volume", pending_vol); pending_vol = -1; }
    if (pending_mic >= 0) { proto_dial("mic", pending_mic); pending_mic = -1; }
}

static void level_label(lv_obj_t *l, int v, bool muted)
{
    if (!l) return;
    if (muted) lv_label_set_text(l, "Muted");
    else lv_label_set_text_fmt(l, "%d", v);
}

static void arc_drag_event(lv_event_t *e)
{
    lv_event_code_t c = lv_event_get_code(e);
    if (c == LV_EVENT_PRESSED) dragging = true;
    else if (c == LV_EVENT_RELEASED || c == LV_EVENT_PRESS_LOST) dragging = false;
}

static void vol_event(lv_event_t *e)
{
    if (!ARC_DRAGGING()) return;
    lv_obj_t *a = lv_event_get_target_obj(e);
    int v = lv_arc_get_value(a);
    if (usb_host_listening()) {
        pending_vol = v;
    } else {
        // No daemon: approximate with volume keys (GNOME steps by ~5 %).
        int steps = (v - vol) / 5;
        for (int i = 0; i < (steps < 0 ? -steps : steps); i++) usb_hid(HID_MEDIA, steps > 0 ? "vol_up" : "vol_down", NULL);
        if (steps == 0) return;
        v = vol + steps * 5;
    }
    vol = v;
    vol_muted = false;
    for (int i = 0; i < live.nvol; i++) {
        if (live.vol_arc[i] != a) lv_arc_set_value(live.vol_arc[i], v);
        level_label(live.vol_lbl[i], v, false);
    }
}

static void mic_event(lv_event_t *e)
{
    if (!ARC_DRAGGING()) return;
    lv_obj_t *a = lv_event_get_target_obj(e);
    mic = lv_arc_get_value(a);
    pending_mic = mic;
    for (int i = 0; i < live.nmic; i++) level_label(live.mic_lbl[i], mic, mic_muted);
}

static void add_level(bool is_mic, lv_obj_t *arc, lv_obj_t *lbl)
{
    if (is_mic) {
        if (live.nmic >= MAX_LIVE) return;
        live.mic_arc[live.nmic] = arc;
        live.mic_lbl[live.nmic++] = lbl;
        lv_arc_set_value(arc, mic);
        level_label(lbl, mic, mic_muted);
        lv_obj_add_event_cb(arc, mic_event, LV_EVENT_VALUE_CHANGED, NULL);
    } else {
        if (live.nvol >= MAX_LIVE) return;
        live.vol_arc[live.nvol] = arc;
        live.vol_lbl[live.nvol++] = lbl;
        lv_arc_set_value(arc, vol);
        level_label(lbl, vol, vol_muted);
        lv_obj_add_event_cb(arc, vol_event, LV_EVENT_VALUE_CHANGED, NULL);
    }
    lv_obj_add_event_cb(arc, arc_drag_event, LV_EVENT_ALL, NULL);
}

// ---- page: media -------------------------------------------------------------------

static void play_morph(lv_obj_t *btn, int play_shape, int playing_shape, bool playing)
{
    // The play button switches shape with the playback state (default circle -> rounded square).
    lv_image_set_src(btn, shape_mask(playing ? playing_shape : play_shape, lv_obj_get_style_width(btn, 0)));
}

static void media_btn_event(lv_event_t *e)
{
    const char *cmd = lv_event_get_user_data(e);
    usb_hid(HID_MEDIA, cmd, NULL);
    if (!strcmp(cmd, "play_pause") && !usb_host_listening()) {
        m_playing = !m_playing; // no daemon to tell us; flip optimistically
        ui_media(m_title, m_artist, m_player, m_playing);
    }
}

static lv_obj_t *media_btn(lv_obj_t *tile, int shape, int size, lv_color_t bg, lv_color_t fg, const char *icon, const lv_font_t *f,
                           const char *cmd)
{
    lv_obj_t *b = mask_button(tile, shape, size, bg);
    lv_obj_t *ic = text(b, f, fg, theme_icon(icon));
    lv_obj_center(ic);
    lv_obj_add_event_cb(b, media_btn_event, LV_EVENT_CLICKED, (void *)cmd);
    return b;
}

static void build_media_inner(lv_obj_t *tile, cJSON *pg);

static void build_media(lv_obj_t *tile, cJSON *pg)
{
    // Media pages may use the album-art palette; every helper reads P, so swap it.
    palette_t saved = P;
    if (pm_active) P = PM;
    build_media_inner(tile, pg);
    P = saved;
}

static void build_media_inner(lv_obj_t *tile, cJSON *pg)
{
    lv_obj_set_style_bg_color(tile, P.surface, 0);
    lv_obj_set_style_bg_opa(tile, LV_OPA_COVER, 0);
    lv_obj_t *cover = lv_image_create(tile);
    lv_obj_set_size(cover, LCD_W, LCD_H);
    lv_obj_center(cover);
    lv_obj_remove_flag(cover, LV_OBJ_FLAG_CLICKABLE);
    if (cover_set) {
        cover_prepare(P.surface, cover_scrim_pct()); // P is the media palette here
        lv_image_set_src(cover, &cover_dsc);
        lv_obj_set_style_bg_opa(tile, LV_OPA_TRANSP, 0); // the opaque cover covers it
    } else {
        lv_obj_add_flag(cover, LV_OBJ_FLAG_HIDDEN);
    }

    lv_obj_t *arc = bezel_arc(tile, P.primary, true);
    lv_obj_t *vl = text(tile, F.label, P.on_sv, "");
    lv_obj_align(vl, LV_ALIGN_CENTER, 0, 132);
    add_level(false, arc, vl);

    if (live.nmedia >= MAX_LIVE) return;
    typeof(live.media[0]) *m = &live.media[live.nmedia++];
    m->cover = cover;
    m->player = text(tile, F.label, P.on_sv, m_player[0] ? m_player : "Nothing playing");
    lv_obj_align(m->player, LV_ALIGN_CENTER, 0, -104);

    m->title = text(tile, F.title, P.on_surface, m_title);
    lv_obj_set_width(m->title, 236);
    lv_label_set_long_mode(m->title, LV_LABEL_LONG_SCROLL_CIRCULAR);
    lv_obj_align(m->title, LV_ALIGN_CENTER, 0, -66);

    m->artist = text(tile, F.body, P.on_sv, m_artist);
    lv_obj_set_width(m->artist, 220);
    lv_label_set_long_mode(m->artist, LV_LABEL_LONG_DOT);
    lv_obj_align(m->artist, LV_ALIGN_CENTER, 0, -36);

    cJSON *shapes = cJSON_GetObjectItem(pg, "shapes");
    m->play_shape = shape_id(jstr(shapes, "play"), SH_CIRCLE);
    m->playing_shape = shape_id(jstr(shapes, "playing"), SH_SQUARE);
    lv_obj_t *prev = media_btn(tile, shape_id(jstr(shapes, "prev"), SH_CIRCLE), 64, P.secc, P.on_secc, "skip_previous", F.icon_m, "prev");
    lv_obj_align(prev, LV_ALIGN_CENTER, -94, 44);
    lv_obj_t *next = media_btn(tile, shape_id(jstr(shapes, "next"), SH_CIRCLE), 64, P.secc, P.on_secc, "skip_next", F.icon_m, "next");
    lv_obj_align(next, LV_ALIGN_CENTER, 94, 44);
    m->play = media_btn(tile, m->play_shape, 100, P.pc, P.on_pc, m_playing ? "pause" : "play_arrow", F.icon_l, "play_pause");
    lv_obj_align(m->play, LV_ALIGN_CENTER, 0, 44);
    m->play_icon = lv_obj_get_child(m->play, 0);
    play_morph(m->play, m->play_shape, m->playing_shape, m_playing);
}

void ui_media(const char *title, const char *artist, const char *player, bool playing)
{
    if (title != m_title) strlcpy(m_title, title ? title : "", sizeof(m_title));
    if (artist != m_artist) strlcpy(m_artist, artist ? artist : "", sizeof(m_artist));
    if (player != m_player) strlcpy(m_player, player ? player : "", sizeof(m_player));
    m_playing = playing;
    for (int i = 0; i < live.nmedia; i++) {
        lv_label_set_text(live.media[i].title, m_title);
        lv_label_set_text(live.media[i].artist, m_artist);
        lv_label_set_text(live.media[i].player, m_player[0] ? m_player : "Nothing playing");
        lv_label_set_text(live.media[i].play_icon, theme_icon(playing ? "pause" : "play_arrow"));
        play_morph(live.media[i].play, live.media[i].play_shape, live.media[i].playing_shape, playing);
    }
}

void ui_cover(const uint8_t *rgb565, int w, int h)
{
    cover_set = rgb565 != NULL;
    cover_src = rgb565;
    cover_w = w;
    cover_h = h;
    if (cover_set) cover_prepare(pm_active ? PM.surface : P.surface, cover_scrim_pct());
    for (int i = 0; i < live.nmedia; i++) {
        lv_obj_t *tile = lv_obj_get_parent(live.media[i].cover);
        if (cover_set) {
            lv_image_set_src(live.media[i].cover, &cover_dsc);
            lv_obj_remove_flag(live.media[i].cover, LV_OBJ_FLAG_HIDDEN);
            lv_obj_set_style_bg_opa(tile, LV_OPA_TRANSP, 0);
            lv_obj_invalidate(live.media[i].cover);
        } else {
            lv_obj_add_flag(live.media[i].cover, LV_OBJ_FLAG_HIDDEN);
            lv_obj_set_style_bg_opa(tile, LV_OPA_COVER, 0);
        }
    }
}

static void build(bool animate);
void ui_media_theme(cJSON *palette)
{
    bool was = pm_active;
    if (palette) {
        PM = P;
        theme_parse_palette(palette, &PM);
        pm_active = true;
    } else {
        pm_active = false;
    }
    if (was || pm_active) {
        remember_page();
        build(false); // media pages pick up PM; the rest keeps the theme
    }
}

void ui_level(const char *kind, int value, bool muted)
{
    bool is_mic = !strcmp(kind, "mic");
    if (dragging) return; // the finger wins
    if (is_mic) {
        mic = value;
        mic_muted = muted;
        for (int i = 0; i < live.nmic; i++) {
            lv_arc_set_value(live.mic_arc[i], value);
            level_label(live.mic_lbl[i], value, muted);
        }
    } else {
        vol = value;
        vol_muted = muted;
        for (int i = 0; i < live.nvol; i++) {
            lv_arc_set_value(live.vol_arc[i], value);
            level_label(live.vol_lbl[i], value, muted);
        }
    }
}

// ---- page: stats -------------------------------------------------------------------

static const char *stat_caption(const char *k)
{
    if (!strcmp(k, "cpu")) return "CPU";
    if (!strcmp(k, "cpuTemp")) return "CPU °C";
    if (!strcmp(k, "ram")) return "RAM";
    if (!strcmp(k, "gpu")) return "GPU";
    if (!strcmp(k, "gpuTemp")) return "GPU °C";
    if (!strcmp(k, "vram")) return "VRAM";
    return k;
}

static void build_stats(lv_obj_t *tile, cJSON *pg)
{
    cJSON *items = cJSON_GetObjectItem(pg, "items");
    int n = cJSON_GetArraySize(items);
    if (n < 1) return;
    if (n > 4) n = 4;
    static const int pos[4][4][2] = {
        {{0, 0}},
        {{-84, 0}, {84, 0}},
        {{0, -80}, {-82, 62}, {82, 62}},
        {{-74, -74}, {74, -74}, {-74, 74}, {74, 74}},
    };
    static const int sizes[4] = {300, 160, 148, 136};
    int size = sizes[n - 1];
    for (int i = 0; i < n && live.nstat < MAX_PAGES * 4; i++) {
        const char *k = cJSON_GetStringValue(cJSON_GetArrayItem(items, i));
        if (!k) continue;
        bool gpu = k[0] == 'g' || k[0] == 'v';
        lv_color_t c = gpu ? P.tertiary : (!strcmp(k, "ram") ? P.on_secc : P.primary);
        lv_obj_t *holder = box(tile);
        lv_obj_set_size(holder, size, size);
        lv_obj_align(holder, LV_ALIGN_CENTER, pos[n - 1][i][0], pos[n - 1][i][1]);
        lv_obj_remove_flag(holder, LV_OBJ_FLAG_CLICKABLE);

        lv_obj_t *a = lv_arc_create(holder);
        lv_obj_set_size(a, size, size);
        lv_obj_center(a);
        lv_arc_set_bg_angles(a, ARC_START, ARC_END);
        lv_arc_set_range(a, 0, 100);
        lv_arc_set_value(a, 0);
        int w = n == 1 ? 20 : 12;
        lv_obj_set_style_arc_width(a, w, LV_PART_MAIN);
        lv_obj_set_style_arc_width(a, w, LV_PART_INDICATOR);
        lv_obj_set_style_arc_color(a, P.schh, LV_PART_MAIN);
        lv_obj_set_style_arc_color(a, c, LV_PART_INDICATOR);
        lv_obj_set_style_arc_rounded(a, true, LV_PART_INDICATOR);
        lv_obj_set_style_bg_opa(a, LV_OPA_TRANSP, LV_PART_KNOB);
        lv_obj_remove_flag(a, LV_OBJ_FLAG_CLICKABLE);

        lv_obj_t *v = text(holder, n == 1 ? F.big : F.value, P.on_surface, "–");
        lv_obj_align(v, LV_ALIGN_CENTER, 0, n == 1 ? -8 : -6);
        lv_obj_t *cap = text(holder, n == 1 ? F.body : F.label, P.on_sv, stat_caption(k));
        lv_obj_align(cap, LV_ALIGN_CENTER, 0, n == 1 ? 44 : 26);

        live.stat[live.nstat].arc = a;
        live.stat[live.nstat].val = v;
        strlcpy(live.stat[live.nstat].key, k, sizeof(live.stat[0].key));
        live.nstat++;
    }
}

void ui_stats(cJSON *msg)
{
    for (int i = 0; i < live.nstat; i++) {
        cJSON *v = cJSON_GetObjectItem(msg, live.stat[i].key);
        if (!cJSON_IsNumber(v)) continue;
        int x = v->valueint;
        lv_arc_set_value(live.stat[i].arc, x > 100 ? 100 : x);
        bool temp = strstr(live.stat[i].key, "Temp") != NULL;
        lv_label_set_text_fmt(live.stat[i].val, temp ? "%d°" : "%d%%", x);
        if (temp) lv_obj_set_style_arc_color(live.stat[i].arc, x >= 85 ? P.error : P.primary, LV_PART_INDICATOR);
    }
}

// ---- page: dial --------------------------------------------------------------------

typedef struct {
    cJSON *pg;
    int acc;
} custom_dial_t;
static custom_dial_t customs[MAX_PAGES];
static int ncustom;

static void bright_event(lv_event_t *e)
{
    lv_obj_t *a = lv_event_get_target_obj(e);
    lv_obj_t *lbl = lv_event_get_user_data(e);
    int v = lv_arc_get_value(a);
    if (v < 5) v = 5;
    if (lv_event_get_code(e) == LV_EVENT_VALUE_CHANGED) {
        if (!ARC_DRAGGING()) return;
        board_backlight(v);
        lv_label_set_text_fmt(lbl, "%d", v);
    } else {
        store_set_brightness(v); // flash write once, on release
    }
}

static void snap_back_exec(void *arc, int32_t v) { lv_arc_set_value(arc, v); }

static void custom_event(lv_event_t *e)
{
    custom_dial_t *cd = lv_event_get_user_data(e);
    lv_obj_t *a = lv_event_get_target_obj(e);
    if (lv_event_get_code(e) == LV_EVENT_RELEASED) {
        lv_anim_t an;
        lv_anim_init(&an);
        lv_anim_set_var(&an, a);
        lv_anim_set_values(&an, lv_arc_get_value(a), 50);
        lv_anim_set_duration(&an, 300);
        lv_anim_set_path_cb(&an, lv_anim_path_overshoot);
        lv_anim_set_exec_cb(&an, snap_back_exec);
        lv_anim_start(&an);
        cd->acc = 50;
        return;
    }
    if (!ARC_DRAGGING()) return; // page swipe, or the snap-back animation
    int v = lv_arc_get_value(a);
    cJSON *step_j = cJSON_GetObjectItem(cd->pg, "step");
    int step = cJSON_IsNumber(step_j) && step_j->valueint > 0 ? step_j->valueint : 5;
    while (v - cd->acc >= step) {
        cd->acc += step;
        run_action(cJSON_GetObjectItem(cd->pg, "onUp"), NULL, false);
    }
    while (cd->acc - v >= step) {
        cd->acc -= step;
        run_action(cJSON_GetObjectItem(cd->pg, "onDown"), NULL, false);
    }
}

static void build_dial(lv_obj_t *tile, cJSON *pg)
{
    const char *src = jstr(pg, "source");
    if (!src) src = "volume";
    lv_obj_t *arc = bezel_arc(tile, !strcmp(src, "mic") ? P.tertiary : P.primary, true);
    lv_obj_t *ic = text(tile, F.icon_xl, P.primary, theme_icon(jstr(pg, "icon") ?: "tune"));
    lv_obj_align(ic, LV_ALIGN_CENTER, 0, -54);
    lv_obj_t *val = text(tile, F.big, P.on_surface, "");
    lv_obj_align(val, LV_ALIGN_CENTER, 0, 14);
    lv_obj_t *cap = text(tile, F.body, P.on_sv, jstr(pg, "label") ?: "");
    lv_obj_align(cap, LV_ALIGN_CENTER, 0, 62);

    if (!strcmp(src, "volume") || !strcmp(src, "mic")) {
        add_level(!strcmp(src, "mic"), arc, val);
    } else if (!strcmp(src, "brightness")) {
        int b = store_brightness();
        lv_arc_set_value(arc, b);
        lv_label_set_text_fmt(val, "%d", b);
        lv_obj_add_event_cb(arc, bright_event, LV_EVENT_VALUE_CHANGED, val);
        lv_obj_add_event_cb(arc, bright_event, LV_EVENT_RELEASED, val);
    } else if (ncustom < MAX_PAGES) {
        custom_dial_t *cd = &customs[ncustom++];
        cd->pg = pg;
        cd->acc = 50;
        lv_arc_set_value(arc, 50);
        lv_obj_add_event_cb(arc, arc_drag_event, LV_EVENT_ALL, NULL);
        lv_obj_add_event_cb(arc, custom_event, LV_EVENT_VALUE_CHANGED, cd);
        lv_obj_add_event_cb(arc, custom_event, LV_EVENT_RELEASED, cd);
    }
}


// ---- page: image (picture, GIF, slideshow) ------------------------------------------------
//
// JPEGs are decoded once into RGB565 with esp_jpeg (ROM TJpgDec), so drawing is a plain
// copy. A slideshow keeps the compressed files and two decoded buffers: the one on
// screen and the next one, decoded just before its cross-fade.

static void *res_keep(void *buf)
{
    if (!res || res->n >= 64 || !buf) {
        free(buf);
        return NULL;
    }
    res->bufs[res->n++] = buf;
    return buf;
}

static uint8_t *load_raw(const char *name, size_t *len)
{
    return res_keep(assets_load(name, len));
}

static bool decode_jpeg(const uint8_t *raw, size_t len, lv_image_dsc_t *dst, uint8_t *out, size_t cap)
{
    esp_jpeg_image_cfg_t cfg = {
        .indata = (uint8_t *)raw,
        .indata_size = len,
        .outbuf = out,
        .outbuf_size = cap,
        .out_format = JPEG_IMAGE_FORMAT_RGB565,
        .out_scale = JPEG_IMAGE_SCALE_0,
    };
    esp_jpeg_image_output_t img;
    if (esp_jpeg_decode(&cfg, &img) != ESP_OK) return false;
    memset(dst, 0, sizeof(*dst));
    dst->header.magic = LV_IMAGE_HEADER_MAGIC;
    dst->header.cf = LV_COLOR_FORMAT_RGB565;
    dst->header.w = img.width;
    dst->header.h = img.height;
    dst->header.stride = img.width * 2;
    dst->data = out;
    dst->data_size = (uint32_t)img.width * img.height * 2;
    lv_image_cache_drop(dst);
    return true;
}

static void device_name(char *out, size_t n, const char *id, const char *fit)
{
    const char *dot = strrchr(id, '.');
    int stem = dot ? (int)(dot - id) : (int)strlen(id);
    snprintf(out, n, "%.*s-%s2%s", stem, id, fit && !strcmp(fit, "contain") ? "f" : "c", dot ? dot : "");
}

#define SLIDE_BUF (LCD_W * LCD_H * 2)

typedef struct {
    const uint8_t *raw[20];
    size_t len[20];
    int n, i, front;
    lv_obj_t *img[2];
    uint8_t *px[2];
    lv_image_dsc_t dsc[2];
    uint32_t ms, last;
} slideshow_t;
static slideshow_t *slides[MAX_PAGES];

static void slide_fade_exec(void *o, int32_t v) { lv_obj_set_style_opa(o, (lv_opa_t)v, 0); }

static void slideshow_tick(lv_timer_t *t)
{
    if (power != AWAKE && power != DIM) return;
    int p = cur_page;
    slideshow_t *ss = p >= 0 && p < page_count ? slides[p] : NULL;
    if (!ss || ss->n < 2) return;
    uint32_t now = lv_tick_get();
    if (now - ss->last < ss->ms) return;
    ss->last = now;
    int next = (ss->i + 1) % ss->n, back = ss->front ^ 1;
    if (!decode_jpeg(ss->raw[next], ss->len[next], &ss->dsc[back], ss->px[back], SLIDE_BUF)) return;
    lv_obj_t *o = ss->img[back];
    lv_obj_set_style_opa(o, LV_OPA_TRANSP, 0);
    lv_image_set_src(o, &ss->dsc[back]);
    lv_obj_move_foreground(o);
    lv_anim_t a;
    lv_anim_init(&a);
    lv_anim_set_var(&a, o);
    lv_anim_set_values(&a, 0, 255);
    lv_anim_set_duration(&a, 600);
    lv_anim_set_path_cb(&a, lv_anim_path_ease_in_out);
    lv_anim_set_exec_cb(&a, slide_fade_exec);
    lv_anim_start(&a);
    ss->i = next;
    ss->front = back;
}

static void build_image(lv_obj_t *tile, cJSON *pg, int page)
{
    cJSON *imgs = cJSON_GetObjectItem(pg, "images");
    const char *fit = jstr(pg, "fit");
    slideshow_t *ss = res_keep(heap_caps_calloc(1, sizeof(slideshow_t), MALLOC_CAP_SPIRAM));
    if (!ss) return;
    bool is_gif = false;
    cJSON *it;
    cJSON_ArrayForEach(it, imgs)
    {
        if (!cJSON_IsString(it) || ss->n >= 20) continue;
        char name[48];
        device_name(name, sizeof(name), it->valuestring, fit);
        is_gif = strstr(name, ".gif") != NULL;
        size_t len = 0;
        const uint8_t *raw = load_raw(name, &len);
        if (raw) {
            ss->raw[ss->n] = raw;
            ss->len[ss->n++] = len;
        }
        if (is_gif) break;
    }
    if (ss->n == 0) { // not synced yet (or the PC never sent it)
        lv_obj_t *ic = text(tile, F.icon_xl, P.on_sv, theme_icon("image"));
        lv_obj_align(ic, LV_ALIGN_CENTER, 0, -20);
        lv_obj_t *l = text(tile, F.body, P.on_sv, usb_host_listening() ? "Syncing images…" : "Connect the PC to sync images");
        lv_obj_align(l, LV_ALIGN_CENTER, 0, 34);
        return;
    }
    if (is_gif) {
        lv_image_dsc_t *d = &res->dscs[res->n - 1];
        memset(d, 0, sizeof(*d));
        d->header.magic = LV_IMAGE_HEADER_MAGIC;
        d->header.cf = LV_COLOR_FORMAT_RAW;
        d->data = ss->raw[0];
        d->data_size = ss->len[0];
        lv_obj_t *g = lv_gif_create(tile);
        lv_gif_set_src(g, d);
        lv_obj_center(g);
        lv_obj_remove_flag(g, LV_OBJ_FLAG_CLICKABLE);
        media_live.gif[page] = g;
        if (page != cur_page) lv_gif_pause(g);
        return;
    }
    int nbuf = ss->n > 1 ? 2 : 1;
    for (int k = 0; k < nbuf; k++) ss->px[k] = res_keep(heap_caps_malloc(SLIDE_BUF, MALLOC_CAP_SPIRAM));
    if (!ss->px[0] || !decode_jpeg(ss->raw[0], ss->len[0], &ss->dsc[0], ss->px[0], SLIDE_BUF)) return;
    for (int k = 0; k < nbuf; k++) {
        lv_obj_t *im = lv_image_create(tile);
        lv_obj_center(im);
        lv_obj_remove_flag(im, LV_OBJ_FLAG_CLICKABLE);
        ss->img[k] = im;
    }
    lv_image_set_src(ss->img[0], &ss->dsc[0]);
    if (nbuf == 2) lv_obj_set_style_opa(ss->img[1], LV_OPA_TRANSP, 0);
    ss->front = 0;
    cJSON *iv = cJSON_GetObjectItem(pg, "interval");
    ss->ms = (cJSON_IsNumber(iv) && iv->valueint >= 2 ? iv->valueint : 8) * 1000;
    ss->last = lv_tick_get();
    slides[page] = ss;
}

static void sync_gifs(void)
{
    for (int i = 0; i < page_count; i++) {
        lv_obj_t *g = media_live.gif[i];
        if (!g) continue;
        if (i == cur_page && power != ASLEEP && power != OFF) lv_gif_resume(g);
        else lv_gif_pause(g);
    }
    if (cur_page >= 0 && cur_page < page_count && slides[cur_page]) slides[cur_page]->last = lv_tick_get();
}

static void free_screen_res(lv_event_t *e)
{
    screen_res_t *r = lv_event_get_user_data(e);
    // Cached entries are keyed by descriptor address; drop them all before the
    // memory can be reused for a different picture.
    lv_image_cache_drop(NULL);
    lv_image_header_cache_drop(NULL);
    for (int i = 0; i < r->n; i++) free(r->bufs[i]);
    free(r);
}

// ---- page: touchpad ------------------------------------------------------------------------

#define RING_R 140 // presses further out than this are on the scroll ring

typedef enum { TP_IDLE, TP_MOVE, TP_RING, TP_SCROLL, TP_SWIPE, TP_DRAG } tp_mode_t;
static struct {
    tp_mode_t mode;
    lv_point_t p0, last;
    uint32_t t0, last_tap;
    lv_point_t last_tap_pt;
    bool moved, long_done;
    int32_t scroll0, angle_acc, last_angle;
    float rem_x, rem_y;
    int speed;
} tp;

static int angle_of(lv_point_t p) { return lv_atan2(p.y - LCD_H / 2, p.x - LCD_W / 2); } // 0..359

static void touchpad_event(lv_event_t *e)
{
    lv_event_code_t c = lv_event_get_code(e);
    lv_point_t p;
    lv_indev_get_point(lv_indev_active(), &p);
    int32_t rx = p.x - LCD_W / 2, ry = p.y - LCD_H / 2;
    if (c == LV_EVENT_PRESSED) {
        tp.p0 = tp.last = p;
        tp.t0 = lv_tick_get();
        tp.moved = tp.long_done = false;
        tp.rem_x = tp.rem_y = 0;
        if (rx * rx + ry * ry > RING_R * RING_R) {
            tp.mode = TP_RING;
            tp.scroll0 = lv_obj_get_scroll_x(tv);
            tp.last_angle = angle_of(p);
            tp.angle_acc = 0;
        } else if (lv_tick_get() - tp.last_tap < 300 && LV_ABS(p.x - tp.last_tap_pt.x) < 40 &&
                   LV_ABS(p.y - tp.last_tap_pt.y) < 40) {
            tp.mode = TP_DRAG; // tap, then press again: hold the left button and drag
            usb_mouse_buttons(1);
        } else {
            tp.mode = TP_MOVE;
        }
        return;
    }
    if (c == LV_EVENT_PRESSING) {
        int32_t dx = p.x - tp.p0.x, dy = p.y - tp.p0.y;
        if (tp.mode == TP_RING && dx * dx + dy * dy >= 100) {
            int32_t ox = tp.p0.x - LCD_W / 2, oy = tp.p0.y - LCD_H / 2;
            int32_t radial = dx * ox + dy * oy, tangential = dx * oy - dy * ox;
            bool across = LV_ABS(radial) > LV_ABS(tangential);
            if (across && LV_ABS(dx) > LV_ABS(dy) && page_count > 1) tp.mode = TP_SWIPE;
            else if (across && dy > 0 && tp.p0.y < 90) {
                tp.mode = TP_IDLE;
                qs_open();
                return;
            } else tp.mode = TP_SCROLL;
        }
        if (tp.mode == TP_SCROLL) {
            int a = angle_of(p), d = a - tp.last_angle;
            if (d > 180) d -= 360;
            if (d < -180) d += 360;
            tp.last_angle = a;
            tp.angle_acc += d;
            while (tp.angle_acc >= 12) { tp.angle_acc -= 12; usb_mouse_wheel(-1); } // clockwise scrolls down
            while (tp.angle_acc <= -12) { tp.angle_acc += 12; usb_mouse_wheel(1); }
        } else if (tp.mode == TP_SWIPE) {
            lv_obj_scroll_to_x(tv, LV_CLAMP(0, tp.scroll0 - dx, (page_count - 1) * LCD_W), LV_ANIM_OFF);
        } else if (tp.mode == TP_MOVE || tp.mode == TP_DRAG) {
            int32_t mx = p.x - tp.last.x, my = p.y - tp.last.y;
            if (dx * dx + dy * dy > 64) tp.moved = true;
            // speed 1..10 and a little acceleration for fast flicks
            float v = sqrtf((float)(mx * mx + my * my));
            float gain = (0.4f + tp.speed * 0.16f) * (1.0f + v * 0.06f);
            tp.rem_x += mx * gain;
            tp.rem_y += my * gain;
            int ix = (int)tp.rem_x, iy = (int)tp.rem_y;
            tp.rem_x -= ix;
            tp.rem_y -= iy;
            if (ix || iy) usb_mouse_move(ix, iy);
        }
        tp.last = p;
        return;
    }
    if (c == LV_EVENT_LONG_PRESSED) {
        if (tp.mode == TP_MOVE && !tp.moved) {
            usb_mouse_click(2);
            tp.long_done = true;
        }
        return;
    }
    if (c == LV_EVENT_RELEASED || c == LV_EVENT_PRESS_LOST) {
        int32_t dx = p.x - tp.p0.x;
        if (tp.mode == TP_MOVE && !tp.moved && !tp.long_done) {
            usb_mouse_click(1);
            tp.last_tap = lv_tick_get();
            tp.last_tap_pt = p;
        } else if (tp.mode == TP_DRAG) {
            usb_mouse_buttons(0);
        } else if (tp.mode == TP_SWIPE) {
            if (dx < -40) goto_index(cur_page + 1, true);
            else if (dx > 40) goto_index(cur_page - 1, true);
            else lv_tileview_set_tile_by_index(tv, cur_page, 0, LV_ANIM_ON);
        }
        tp.mode = TP_IDLE;
    }
}

static void build_touchpad(lv_obj_t *tile, cJSON *pg)
{
    cJSON *sp = cJSON_GetObjectItem(pg, "speed");
    tp.speed = cJSON_IsNumber(sp) ? LV_CLAMP(1, sp->valueint, 10) : 5;
    // scroll ring
    lv_obj_t *ring = lv_arc_create(tile);
    lv_obj_set_size(ring, 344, 344);
    lv_obj_center(ring);
    lv_arc_set_bg_angles(ring, 0, 360);
    lv_obj_set_style_arc_width(ring, 22, LV_PART_MAIN);
    lv_obj_set_style_arc_color(ring, P.sch, LV_PART_MAIN);
    lv_obj_set_style_arc_opa(ring, LV_OPA_TRANSP, LV_PART_INDICATOR);
    lv_obj_set_style_bg_opa(ring, LV_OPA_TRANSP, LV_PART_KNOB);
    lv_obj_remove_flag(ring, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_t *ic = text(tile, F.icon_xl, P.on_sv, theme_icon("mouse"));
    lv_obj_align(ic, LV_ALIGN_CENTER, 0, -18);
    lv_obj_set_style_opa(ic, LV_OPA_60, 0);
    lv_obj_t *l = text(tile, F.body, P.on_sv, jstr(pg, "title") ?: "Touchpad");
    lv_obj_align(l, LV_ALIGN_CENTER, 0, 34);
    lv_obj_set_style_opa(l, LV_OPA_60, 0);
    // the input surface: catches every touch on this page, never scrolls the pages itself
    lv_obj_t *pad = box(tile);
    lv_obj_set_size(pad, LCD_W, LCD_H);
    lv_obj_center(pad);
    lv_obj_add_flag(pad, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_remove_flag(pad, LV_OBJ_FLAG_SCROLL_CHAIN_HOR | LV_OBJ_FLAG_SCROLL_CHAIN_VER | LV_OBJ_FLAG_GESTURE_BUBBLE);
    lv_obj_add_event_cb(pad, touchpad_event, LV_EVENT_PRESSED, NULL);
    lv_obj_add_event_cb(pad, touchpad_event, LV_EVENT_PRESSING, NULL);
    lv_obj_add_event_cb(pad, touchpad_event, LV_EVENT_LONG_PRESSED, NULL);
    lv_obj_add_event_cb(pad, touchpad_event, LV_EVENT_RELEASED, NULL);
    lv_obj_add_event_cb(pad, touchpad_event, LV_EVENT_PRESS_LOST, NULL);
}

// ---- navigation: wrap-around & gestures ----------------------------------------------------

static lv_obj_t *wrap_shot;
static void wrap_x_exec(void *o, int32_t v) { lv_obj_set_x(o, v); }
static void wrap_shot_x_exec(void *o, int32_t v) { lv_obj_set_x(o, v); }
static void dc_flush_pending(void);
static void wrap_done(lv_anim_t *a)
{
    if (!wrap_shot) return;
    lv_draw_buf_t *buf = (lv_draw_buf_t *)lv_image_get_src(wrap_shot);
    lv_obj_delete(wrap_shot);
    wrap_shot = NULL;
    if (buf) lv_draw_buf_destroy(buf);
    if (a) dc_flush_pending();
}

// Wrapping can't scroll the tileview past its ends, so fake the slide: the old
// page leaves as a snapshot while the tileview, already on the target, slides in.
static void wrap_slide(int dir)
{
    if (wrap_shot) return;
    lv_draw_buf_t *shot = lv_snapshot_take(tv, LV_COLOR_FORMAT_RGB565);
    if (!shot) return;
    wrap_shot = lv_image_create(lv_obj_get_parent(tv));
    lv_image_set_src(wrap_shot, shot);
    lv_obj_set_pos(wrap_shot, 0, 0);
    lv_obj_remove_flag(wrap_shot, LV_OBJ_FLAG_CLICKABLE);
    lv_anim_t a;
    lv_anim_init(&a);
    lv_anim_set_duration(&a, 300);
    lv_anim_set_path_cb(&a, lv_anim_path_ease_out);
    lv_anim_set_var(&a, tv);
    lv_anim_set_values(&a, dir * LCD_W, 0);
    lv_anim_set_exec_cb(&a, wrap_x_exec);
    lv_anim_start(&a);
    lv_anim_set_var(&a, wrap_shot);
    lv_anim_set_values(&a, 0, -dir * LCD_W);
    lv_anim_set_exec_cb(&a, wrap_shot_x_exec);
    lv_anim_set_completed_cb(&a, wrap_done);
    lv_anim_start(&a);
}

static void goto_index(int idx, bool wrap_anim)
{
    if (page_count < 1) return;
    bool wrapped = idx < 0 || idx >= page_count;
    int dir = idx < 0 ? -1 : 1;
    idx = (idx % page_count + page_count) % page_count;
    if (wrapped && wrap_anim) wrap_slide(dir);
    lv_tileview_set_tile_by_index(tv, idx, 0, wrapped || !wrap_anim ? LV_ANIM_OFF : LV_ANIM_ON);
    if (wrapped) { // no scroll animation means no VALUE_CHANGED from the tileview
        cur_page = idx;
        remember_page();
        update_dots();
        sync_gifs();
        ui_announce();
    }
}

static void screen_gesture(lv_event_t *e)
{
    if (arc_mode == ARC_DRAG || arc_mode == ARC_SWIPE || (qs && !lv_obj_has_flag(qs, LV_OBJ_FLAG_HIDDEN))) return;
    lv_dir_t dir = lv_indev_get_gesture_dir(lv_indev_active());
    if (dir == LV_DIR_RIGHT && cur_page == 0 && page_count > 1) {
        goto_index(-1, true); // finger moved right on the first page: wrap to the last
    } else if (dir == LV_DIR_LEFT && cur_page == page_count - 1 && page_count > 1) {
        goto_index(page_count, true);
    }
}

// ---- quick settings ------------------------------------------------------------------------

static lv_obj_t *qs_auto_tile;

static void qs_y_exec(void *o, int32_t v) { lv_obj_set_y(o, v); }
static void qs_hide_done(lv_anim_t *a) { lv_obj_add_flag(a->var, LV_OBJ_FLAG_HIDDEN); }

static void qs_close(void)
{
    if (!qs || lv_obj_has_flag(qs, LV_OBJ_FLAG_HIDDEN)) return;
    lv_anim_t a;
    lv_anim_init(&a);
    lv_anim_set_var(&a, qs);
    lv_anim_set_values(&a, lv_obj_get_y(qs), -LCD_H);
    lv_anim_set_duration(&a, 220);
    lv_anim_set_path_cb(&a, lv_anim_path_ease_in);
    lv_anim_set_exec_cb(&a, qs_y_exec);
    lv_anim_set_completed_cb(&a, qs_hide_done);
    lv_anim_start(&a);
}

static void qs_style_auto(void)
{
    if (!qs_auto_tile) return;
    lv_color_t bg = autoswitch_paused ? P.schh : P.pc, fg = autoswitch_paused ? P.on_sv : P.on_pc;
    lv_obj_set_style_bg_color(qs_auto_tile, bg, 0);
    lv_obj_t *ic = lv_obj_get_child(qs_auto_tile, 0), *st = lv_obj_get_child(qs_auto_tile, 2);
    lv_label_set_text(ic, theme_icon(autoswitch_paused ? "pause" : "sync"));
    lv_obj_set_style_text_color(ic, fg, 0);
    lv_obj_set_style_text_color(lv_obj_get_child(qs_auto_tile, 1), fg, 0);
    lv_label_set_text(st, autoswitch_paused ? "Paused" : "On");
    lv_obj_set_style_text_color(st, fg, 0);
    if (pause_badge) {
        if (autoswitch_paused) lv_obj_remove_flag(pause_badge, LV_OBJ_FLAG_HIDDEN);
        else lv_obj_add_flag(pause_badge, LV_OBJ_FLAG_HIDDEN);
    }
}

static void qs_event(lv_event_t *e)
{
    lv_event_code_t c = lv_event_get_code(e);
    intptr_t what = (intptr_t)lv_event_get_user_data(e);
    if (c == LV_EVENT_GESTURE) {
        if (lv_indev_get_gesture_dir(lv_indev_active()) == LV_DIR_TOP) qs_close();
        return;
    }
    if (c != LV_EVENT_CLICKED) return;
    if (what == 1) { // screen off
        qs_close();
        screen_off();
    } else if (what == 2) { // auto-switch
        autoswitch_paused = !autoswitch_paused;
        store_local_set("autosw_paused", autoswitch_paused);
        qs_style_auto();
    } else if (what >= 100) { // page shortcut
        qs_close();
        goto_index((int)(what - 100), true);
    } else {
        qs_close(); // background tap
    }
}

static lv_obj_t *qs_tile(lv_obj_t *parent, const char *icon, const char *label, int x, intptr_t what)
{
    lv_obj_t *t = box(parent);
    lv_obj_set_size(t, 128, 104);
    lv_obj_align(t, LV_ALIGN_CENTER, x, -4);
    lv_obj_set_style_radius(t, 32, 0);
    lv_obj_set_style_bg_color(t, P.schh, 0);
    lv_obj_set_style_bg_opa(t, LV_OPA_COVER, 0);
    lv_obj_add_flag(t, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_set_style_bg_color(t, lv_color_mix(lv_color_white(), P.schh, 30), LV_STATE_PRESSED);
    squish_on_press(t, 104);
    lv_obj_set_style_transform_pivot_x(t, 64, 0);
    lv_obj_t *ic = text(t, F.icon_m, P.on_surface, theme_icon(icon));
    lv_obj_align(ic, LV_ALIGN_TOP_MID, 0, 14);
    lv_obj_t *l = text(t, F.body, P.on_surface, label);
    lv_obj_align(l, LV_ALIGN_TOP_MID, 0, 52);
    lv_obj_t *st = text(t, F.label, P.on_sv, "");
    lv_obj_align(st, LV_ALIGN_TOP_MID, 0, 74);
    lv_obj_add_event_cb(t, qs_event, LV_EVENT_CLICKED, (void *)what);
    return t;
}

static void qs_build(void)
{
    if (qs) lv_obj_delete(qs);
    qs = box(lv_layer_top());
    lv_obj_set_size(qs, LCD_W, LCD_H);
    lv_obj_set_style_radius(qs, LV_RADIUS_CIRCLE, 0);
    lv_obj_set_style_bg_color(qs, P.sc, 0);
    lv_obj_set_style_bg_opa(qs, LV_OPA_COVER, 0);
    lv_obj_add_flag(qs, LV_OBJ_FLAG_CLICKABLE | LV_OBJ_FLAG_HIDDEN);
    lv_obj_set_y(qs, -LCD_H);
    lv_obj_add_event_cb(qs, qs_event, LV_EVENT_CLICKED, (void *)0);
    lv_obj_add_event_cb(qs, qs_event, LV_EVENT_GESTURE, (void *)0);

    lv_obj_t *title = text(qs, F.body, P.on_sv, "Quick settings");
    lv_obj_align(title, LV_ALIGN_TOP_MID, 0, 84);

    // tiny page shortcuts along the top edge
    for (int i = 0; i < page_count; i++) {
        const float r = 145, step = 36.0f / r; // 36 px apart on the arc
        float a = 3 * (float)M_PI / 2 + (i - (page_count - 1) / 2.0f) * step;
        lv_obj_t *b = box(qs);
        lv_obj_align(b, LV_ALIGN_CENTER, (int32_t)lroundf(r * cosf(a)), (int32_t)lroundf(r * sinf(a)));
        lv_obj_set_size(b, 30, 30);
        lv_obj_set_style_radius(b, 15, 0);
        bool cur = i == cur_page;
        lv_obj_set_style_bg_color(b, cur ? P.primary : P.schh, 0);
        lv_obj_set_style_bg_opa(b, LV_OPA_COVER, 0);
        lv_obj_add_flag(b, LV_OBJ_FLAG_CLICKABLE);
        char num[4];
        snprintf(num, sizeof(num), "%d", i + 1);
        lv_obj_t *l = text(b, F.label, cur ? P.on_primary : P.on_surface, num);
        lv_obj_center(l);
        lv_obj_add_event_cb(b, qs_event, LV_EVENT_CLICKED, (void *)(intptr_t)(100 + i));
    }

    qs_tile(qs, "bedtime", "Screen off", -70, 1);
    qs_auto_tile = qs_tile(qs, "sync", "Auto-switch", 70, 2);
    qs_style_auto();

    lv_obj_t *close = box(qs); // same pill as the notification panel's close button
    lv_obj_set_size(close, 64, 40);
    lv_obj_align(close, LV_ALIGN_BOTTOM_MID, 0, -8);
    lv_obj_set_style_radius(close, 20, 0);
    lv_obj_set_style_bg_color(close, P.schh, 0);
    lv_obj_set_style_bg_opa(close, LV_OPA_COVER, 0);
    lv_obj_add_flag(close, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_t *ci = text(close, F.icon_m, P.on_surface, theme_icon("expand_less"));
    lv_obj_center(ci);
    lv_obj_add_event_cb(close, qs_event, LV_EVENT_CLICKED, (void *)3);
}

static void qs_open(void)
{
    if (power != AWAKE && power != DIM) return;
    qs_build(); // fresh page list and colors every time
    lv_obj_remove_flag(qs, LV_OBJ_FLAG_HIDDEN);
    lv_anim_t a;
    lv_anim_init(&a);
    lv_anim_set_var(&a, qs);
    lv_anim_set_values(&a, -LCD_H, 0);
    lv_anim_set_duration(&a, 280);
    lv_anim_set_path_cb(&a, lv_anim_path_ease_out);
    lv_anim_set_exec_cb(&a, qs_y_exec);
    lv_anim_start(&a);
    if (indev) { // the pull must not also press, drag or scroll whatever was under the finger
        // lv_indev_reset drops the pressed widget without PRESS_LOST: undo our own drag state too
        arc_mode = ARC_IDLE;
        dragging = false;
        tp.mode = TP_IDLE;
        lv_indev_reset(indev, NULL);
        lv_indev_wait_release(indev);
    }
}


// ---- time & weather (pushed by the daemon) -------------------------------------------------

static struct {
    bool valid;
    int64_t base, at_us; // unix seconds at esp_timer time at_us
    int32_t offset;      // local UTC offset
    struct {
        char tz[40];
        int32_t off;
    } zones[12];
    int nzones;
} clk;

static struct {
    bool valid;
    char city[48];
    int temp, hi, lo, code;
    bool day, imperial;
} wx;

static int64_t now_utc(void) { return clk.base + (esp_timer_get_time() - clk.at_us) / 1000000; }

static void broken_down(int32_t offset, struct tm *out)
{
    time_t t = (time_t)(now_utc() + offset);
    gmtime_r(&t, out);
}

static int32_t zone_offset(const char *tz)
{
    for (int i = 0; i < clk.nzones; i++)
        if (!strcmp(clk.zones[i].tz, tz)) return clk.zones[i].off;
    return clk.offset;
}

static const char *weather_icon(int code, bool day)
{
    if (code == 0) return day ? "clear_day" : "clear_night";
    if (code <= 2) return day ? "partly_cloudy_day" : "partly_cloudy_night";
    if (code == 3) return "cloud";
    if (code == 45 || code == 48) return "foggy";
    if (code == 65 || code == 67 || code == 82) return "rainy_heavy";
    if ((code >= 51 && code <= 67) || (code >= 80 && code <= 82)) return "rainy";
    if ((code >= 71 && code <= 77) || code == 85 || code == 86) return "weather_snowy";
    if (code >= 95) return "thunderstorm";
    return "cloud";
}

// ---- clock widgets -----------------------------------

typedef struct {
    bool used, analog;
    lv_obj_t *hour, *min, *sec, *time_lbl, *date_lbl, *wx_icon, *wx_lbl, *zone_lbl[3];
    lv_point_precise_t hp[2], mp[2];
    char tz[3][40], zname[3][17];
    int nz;
} clock_w;
static clock_w clocks[MAX_PAGES];

static void hand(lv_obj_t *line, lv_point_precise_t *pts, int angle10, int len)
{
    // angle in tenths of a degree, 0 = 12 o'clock, clockwise
    int a = (angle10 / 10 + 270) % 360;
    pts[0].x = LCD_W / 2;
    pts[0].y = LCD_H / 2;
    pts[1].x = LCD_W / 2 + len * lv_trigo_cos(a) / LV_TRIGO_SIN_MAX;
    pts[1].y = LCD_H / 2 + len * lv_trigo_sin(a) / LV_TRIGO_SIN_MAX;
    lv_line_set_points(line, pts, 2);
}

static void clock_update(clock_w *c)
{
    if (!c->used) return;
    if (!clk.valid) {
        if (c->time_lbl) lv_label_set_text(c->time_lbl, "--:--");
        return;
    }
    struct tm t;
    broken_down(clk.offset, &t);
    if (c->analog) {
        hand(c->hour, c->hp, (t.tm_hour % 12) * 300 + t.tm_min * 5, 78);
        hand(c->min, c->mp, t.tm_min * 60 + t.tm_sec, 118);
        int a = (t.tm_sec * 6 + 270) % 360;
        lv_obj_set_pos(c->sec, LCD_W / 2 + 132 * lv_trigo_cos(a) / LV_TRIGO_SIN_MAX - 5,
                       LCD_H / 2 + 132 * lv_trigo_sin(a) / LV_TRIGO_SIN_MAX - 5);
    } else {
        lv_label_set_text_fmt(c->time_lbl, "%02d:%02d", t.tm_hour, t.tm_min);
        static const char *days[] = {"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"};
        static const char *months[] = {"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"};
        lv_label_set_text_fmt(c->date_lbl, "%s %d %s", days[t.tm_wday], t.tm_mday, months[t.tm_mon]);
    }
    for (int i = 0; i < c->nz; i++) {
        struct tm z;
        broken_down(zone_offset(c->tz[i]), &z);
        lv_label_set_text_fmt(c->zone_lbl[i], "%s  %02d:%02d", c->zname[i], z.tm_hour, z.tm_min);
    }
    if (c->wx_lbl) {
        if (wx.valid) {
            lv_label_set_text(c->wx_icon, theme_icon(weather_icon(wx.code, wx.day)));
            lv_label_set_text_fmt(c->wx_lbl, "%d°  %d° / %d°", wx.temp, wx.hi, wx.lo);
        } else {
            lv_label_set_text(c->wx_icon, "");
            lv_label_set_text(c->wx_lbl, "");
        }
    }
}

static void build_clock_into(lv_obj_t *parent, cJSON *pg, clock_w *c, bool force_digital, bool weather)
{
    memset(c, 0, sizeof(*c));
    c->used = true;
    const char *style = jstr(pg, "style");
    c->analog = !force_digital && !(style && !strcmp(style, "digital"));
    if (c->analog) {
        lv_obj_t *face = box(parent);
        lv_obj_set_size(face, 300, 300);
        lv_obj_center(face);
        lv_obj_set_style_radius(face, LV_RADIUS_CIRCLE, 0);
        lv_obj_set_style_bg_color(face, P.pc, 0);
        lv_obj_set_style_bg_opa(face, LV_OPA_40, 0);
        lv_obj_remove_flag(face, LV_OBJ_FLAG_CLICKABLE);
        for (int i = 0; i < 12; i++) {
            int a = (i * 30 + 270) % 360;
            lv_obj_t *tk = box(parent);
            int r = i % 3 == 0 ? 5 : 3;
            lv_obj_set_size(tk, 2 * r, 2 * r);
            lv_obj_set_style_radius(tk, r, 0);
            lv_obj_set_style_bg_color(tk, i % 3 == 0 ? P.primary : P.on_sv, 0);
            lv_obj_set_style_bg_opa(tk, LV_OPA_COVER, 0);
            lv_obj_set_pos(tk, LCD_W / 2 + 132 * lv_trigo_cos(a) / LV_TRIGO_SIN_MAX - r,
                           LCD_H / 2 + 132 * lv_trigo_sin(a) / LV_TRIGO_SIN_MAX - r);
            lv_obj_remove_flag(tk, LV_OBJ_FLAG_CLICKABLE);
        }
    } else {
        c->time_lbl = text(parent, theme_font_clock(), P.on_surface, "--:--");
        lv_obj_align(c->time_lbl, LV_ALIGN_CENTER, 0, -24);
        c->date_lbl = text(parent, F.body, P.on_sv, "");
        lv_obj_align(c->date_lbl, LV_ALIGN_CENTER, 0, 26);
    }
    cJSON *wflag = cJSON_GetObjectItem(pg, "weather");
    if (weather && !(cJSON_IsBool(wflag) && cJSON_IsFalse(wflag))) {
        c->wx_icon = text(parent, F.icon_m, P.primary, "");
        c->wx_lbl = text(parent, F.body, P.on_surface, "");
        int y = c->analog ? -66 : -92; // weather sits above the home time
        lv_obj_align(c->wx_icon, LV_ALIGN_CENTER, -54, y);
        lv_obj_align(c->wx_lbl, LV_ALIGN_CENTER, 22, y);
    }
    cJSON *zones = cJSON_GetObjectItem(pg, "zones"), *z;
    cJSON_ArrayForEach(z, zones)
    {
        if (c->nz >= 3) break;
        const char *tz = jstr(z, "tz"), *label = jstr(z, "label");
        if (!tz) continue;
        strlcpy(c->tz[c->nz], tz, sizeof(c->tz[0]));
        if (!label || !label[0]) {
            const char *slash = strrchr(tz, '/');
            label = slash ? slash + 1 : tz;
        }
        strlcpy(c->zname[c->nz], label, sizeof(c->zname[0]));
        for (char *u = c->zname[c->nz]; *u; u++)
            if (*u == '_') *u = ' ';
        c->zone_lbl[c->nz] = text(parent, F.label, P.on_sv, "");
        lv_obj_align(c->zone_lbl[c->nz], LV_ALIGN_CENTER, 0, (c->analog ? 72 : 82) + c->nz * 20); // world clocks below
        c->nz++;
    }
    if (c->analog) { // hands last so they draw on top
        c->hour = lv_line_create(parent);
        c->min = lv_line_create(parent);
        lv_obj_set_style_line_width(c->hour, 10, 0);
        lv_obj_set_style_line_color(c->hour, P.primary, 0);
        lv_obj_set_style_line_rounded(c->hour, true, 0);
        lv_obj_set_style_line_width(c->min, 6, 0);
        lv_obj_set_style_line_color(c->min, P.on_surface, 0);
        lv_obj_set_style_line_rounded(c->min, true, 0);
        c->sec = box(parent);
        lv_obj_set_size(c->sec, 10, 10);
        lv_obj_set_style_radius(c->sec, 5, 0);
        lv_obj_set_style_bg_color(c->sec, P.tertiary, 0);
        lv_obj_set_style_bg_opa(c->sec, LV_OPA_COVER, 0);
        lv_obj_t *pin = box(parent);
        lv_obj_set_size(pin, 14, 14);
        lv_obj_center(pin);
        lv_obj_set_style_radius(pin, 7, 0);
        lv_obj_set_style_bg_color(pin, P.primary, 0);
        lv_obj_set_style_bg_opa(pin, LV_OPA_COVER, 0);
        lv_obj_remove_flag(c->hour, LV_OBJ_FLAG_CLICKABLE);
        lv_obj_remove_flag(c->min, LV_OBJ_FLAG_CLICKABLE);
        lv_obj_remove_flag(c->sec, LV_OBJ_FLAG_CLICKABLE);
        lv_obj_remove_flag(pin, LV_OBJ_FLAG_CLICKABLE);
    }
    clock_update(c);
}

static void build_clock(lv_obj_t *tile, cJSON *pg, int page)
{
    build_clock_into(tile, pg, &clocks[page], false, true);
}

static void tv_settle(void);

static void clock_tick(lv_timer_t *t)
{
    tv_settle();
    if (power == OFF || power == ASLEEP) return;
    if (cur_page >= 0 && cur_page < page_count) clock_update(&clocks[cur_page]);
    game_session_update();
}

void ui_time(cJSON *m)
{
    cJSON *e = cJSON_GetObjectItem(m, "epoch"), *o = cJSON_GetObjectItem(m, "offset");
    if (!cJSON_IsNumber(e)) return;
    clk.base = (int64_t)e->valuedouble;
    clk.at_us = esp_timer_get_time();
    clk.offset = cJSON_IsNumber(o) ? o->valueint : 0;
    clk.nzones = 0;
    cJSON *z;
    cJSON_ArrayForEach(z, cJSON_GetObjectItem(m, "zones"))
    {
        if (clk.nzones >= 12 || !cJSON_IsNumber(z)) continue;
        strlcpy(clk.zones[clk.nzones].tz, z->string, sizeof(clk.zones[0].tz));
        clk.zones[clk.nzones++].off = z->valueint;
    }
    clk.valid = true;
    clock_tick(NULL);
}

void ui_weather(cJSON *m)
{
    cJSON *t = cJSON_GetObjectItem(m, "temp");
    if (!cJSON_IsNumber(t)) return;
    wx.temp = (int)lroundf(t->valuedouble);
    cJSON *hi = cJSON_GetObjectItem(m, "hi"), *lo = cJSON_GetObjectItem(m, "lo"), *c = cJSON_GetObjectItem(m, "code");
    wx.hi = cJSON_IsNumber(hi) ? (int)lroundf(hi->valuedouble) : wx.temp;
    wx.lo = cJSON_IsNumber(lo) ? (int)lroundf(lo->valuedouble) : wx.temp;
    wx.code = cJSON_IsNumber(c) ? c->valueint : 3;
    wx.day = cJSON_IsTrue(cJSON_GetObjectItem(m, "day"));
    strlcpy(wx.city, jstr(m, "city") ?: "", sizeof(wx.city));
    wx.valid = true;
    for (int i = 0; i < MAX_PAGES; i++) clock_update(&clocks[i]);
}

// ---- timer / pomodoro ----------------------------------------------------------------------

static struct {
    int set_s, left_ms, phase, cycle; // phase 0 work, 1 break (pomodoro)
    bool running, pomodoro;
    uint32_t last;
    char label[25];
} tmr = {.set_s = 25 * 60, .left_ms = 25 * 60 * 1000};

static struct {
    lv_obj_t *arc, *time_lbl, *state_icon, *phase_lbl, *presets, *reset;
} tw;

// Idle = stopped at the full duration: only then can the ring and presets change it.
static bool timer_idle(void) { return !tmr.running && tmr.left_ms == tmr.set_s * 1000; }

static void timer_show(void)
{
    if (!tw.arc) return;
    int s = (tmr.left_ms + 999) / 1000;
    lv_label_set_text_fmt(tw.time_lbl, "%d:%02d", s / 60, s % 60);
    lv_label_set_text(tw.state_icon, theme_icon(tmr.running ? "pause" : "play_arrow"));
    bool idle = timer_idle();
    if (!idle) { // counting or paused: the ring only shows what's left, read-only until reset
        lv_arc_set_range(tw.arc, 0, 1000);
        lv_arc_set_value(tw.arc, tmr.set_s ? (int32_t)((int64_t)tmr.left_ms * 1000 / (tmr.set_s * 1000)) : 0);
        lv_obj_remove_flag(tw.arc, LV_OBJ_FLAG_CLICKABLE);
        lv_obj_set_style_opa(tw.arc, LV_OPA_TRANSP, LV_PART_KNOB);
    } else { // stopped at full: the ring sets the minutes
        lv_arc_set_range(tw.arc, 0, 120);
        lv_arc_set_value(tw.arc, tmr.set_s / 60);
        if (!tmr.pomodoro) {
            lv_obj_add_flag(tw.arc, LV_OBJ_FLAG_CLICKABLE);
            lv_obj_set_style_opa(tw.arc, LV_OPA_COVER, LV_PART_KNOB);
        }
    }
    if (tw.presets) {
        if (idle) lv_obj_remove_flag(tw.presets, LV_OBJ_FLAG_HIDDEN);
        else lv_obj_add_flag(tw.presets, LV_OBJ_FLAG_HIDDEN);
    }
    if (tw.reset) { // "disabled" look by color, not opacity (a translucent button looked like a stain)
        lv_obj_set_style_bg_color(tw.reset, idle ? P.sch : P.schh, 0);
        lv_obj_set_style_text_color(lv_obj_get_child(tw.reset, 0), idle ? P.outline : P.on_surface, 0);
    }
    if (tw.phase_lbl)
        lv_label_set_text(tw.phase_lbl, tmr.pomodoro ? (tmr.phase ? "Break" : "Focus") : (tmr.label[0] ? tmr.label : ""));
}

static lv_obj_t *pulse;
static void pulse_exec(void *o, int32_t v) { lv_obj_set_style_bg_opa(o, (lv_opa_t)v, 0); }
static void pulse_done(lv_anim_t *a)
{
    lv_obj_delete(a->var);
    pulse = NULL;
}

static void timer_finished(void)
{
    wake();
    if (!pulse) { // three soft flashes of the primary color
        pulse = box(lv_layer_top());
        lv_obj_set_size(pulse, LCD_W, LCD_H);
        lv_obj_set_style_radius(pulse, LV_RADIUS_CIRCLE, 0);
        lv_obj_set_style_bg_color(pulse, P.primary, 0);
        lv_obj_remove_flag(pulse, LV_OBJ_FLAG_CLICKABLE);
        lv_anim_t a;
        lv_anim_init(&a);
        lv_anim_set_var(&a, pulse);
        lv_anim_set_values(&a, 0, 160);
        lv_anim_set_duration(&a, 350);
        lv_anim_set_playback_duration(&a, 350);
        lv_anim_set_repeat_count(&a, 3);
        lv_anim_set_exec_cb(&a, pulse_exec);
        lv_anim_set_completed_cb(&a, pulse_done);
        lv_anim_start(&a);
    }
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", "timerDone");
    cJSON_AddStringToObject(o, "label", tmr.label);
    if (tmr.pomodoro) cJSON_AddStringToObject(o, "phase", tmr.phase ? "break" : "work");
    char *js = cJSON_PrintUnformatted(o);
    if (js) usb_send_line(js);
    free(js);
    cJSON_Delete(o);
    ui_toast(tmr.pomodoro ? (tmr.phase ? "Back to work" : "Break time") : "Time's up");
    if (tmr.pomodoro) { // next phase starts by itself
        if (tmr.phase == 0) {
            tmr.cycle++;
            tmr.phase = 1;
            tmr.set_s = (tmr.cycle % 4 == 0 ? 15 : 5) * 60;
        } else {
            tmr.phase = 0;
            tmr.set_s = 25 * 60;
        }
        tmr.left_ms = tmr.set_s * 1000;
        tmr.running = true;
        tmr.last = lv_tick_get();
    } else {
        tmr.running = false;
        tmr.left_ms = tmr.set_s * 1000;
    }
}

static void timer_tick(lv_timer_t *t)
{
    if (!tmr.running) return;
    uint32_t now = lv_tick_get();
    tmr.left_ms -= (int)(now - tmr.last);
    tmr.last = now;
    if (tmr.left_ms <= 0) timer_finished();
    timer_show();
}

static void timer_button_event(lv_event_t *e)
{
    if (!(intptr_t)lv_event_get_user_data(e)) { // play / pause
        tmr.running = !tmr.running;
        tmr.last = lv_tick_get();
    } else { // reset
        tmr.running = false;
        if (tmr.pomodoro) {
            tmr.phase = 0;
            tmr.cycle = 0;
            tmr.set_s = 25 * 60;
        }
        tmr.left_ms = tmr.set_s * 1000;
    }
    timer_show();
}

static void timer_arc_event(lv_event_t *e)
{
    if (!ARC_DRAGGING() || tmr.running || tmr.left_ms != tmr.set_s * 1000 || tmr.pomodoro) return;
    int m = LV_MAX(1, lv_arc_get_value(lv_event_get_target_obj(e)));
    tmr.set_s = m * 60;
    tmr.left_ms = tmr.set_s * 1000;
    timer_show();
}

static void timer_preset_event(lv_event_t *e)
{
    if (!timer_idle()) return;
    tmr.set_s = (int)(intptr_t)lv_event_get_user_data(e) * 60;
    tmr.left_ms = tmr.set_s * 1000;
    timer_show();
}

static void build_timer(lv_obj_t *tile, cJSON *pg)
{
    tmr.pomodoro = cJSON_IsTrue(cJSON_GetObjectItem(pg, "pomodoro"));
    strlcpy(tmr.label, jstr(pg, "label") ?: "", sizeof(tmr.label));
    if (tmr.pomodoro && !tmr.running && tmr.left_ms == tmr.set_s * 1000 && tmr.phase == 0) {
        tmr.set_s = 25 * 60;
        tmr.left_ms = tmr.set_s * 1000;
    }
    tw.arc = bezel_arc(tile, P.primary, true);
    lv_arc_set_bg_angles(tw.arc, 0, 360);
    lv_arc_set_rotation(tw.arc, 270);
    lv_obj_add_event_cb(tw.arc, arc_drag_event, LV_EVENT_ALL, NULL);
    lv_obj_add_event_cb(tw.arc, timer_arc_event, LV_EVENT_VALUE_CHANGED, NULL);

    tw.time_lbl = text(tile, F.big, P.on_surface, "");
    lv_obj_align(tw.time_lbl, LV_ALIGN_CENTER, 0, -40);
    tw.phase_lbl = text(tile, F.body, P.on_sv, "");
    lv_obj_align(tw.phase_lbl, LV_ALIGN_CENTER, 0, 2);

    // play/pause and reset, side by side
    lv_obj_t *play = box(tile);
    lv_obj_set_size(play, 64, 64);
    lv_obj_set_style_radius(play, 20, 0);
    lv_obj_set_style_bg_color(play, P.pc, 0);
    lv_obj_set_style_bg_opa(play, LV_OPA_COVER, 0);
    lv_obj_add_flag(play, LV_OBJ_FLAG_CLICKABLE);
    squish_on_press(play, 64);
    lv_obj_align(play, LV_ALIGN_CENTER, -38, 54);
    tw.state_icon = text(play, F.icon_m, P.on_pc, "");
    lv_obj_center(tw.state_icon);
    lv_obj_add_event_cb(play, timer_button_event, LV_EVENT_CLICKED, (void *)0);
    tw.reset = box(tile);
    lv_obj_set_size(tw.reset, 64, 64);
    lv_obj_set_style_radius(tw.reset, 32, 0);
    lv_obj_set_style_bg_color(tw.reset, P.schh, 0);
    lv_obj_set_style_bg_opa(tw.reset, LV_OPA_COVER, 0);
    lv_obj_add_flag(tw.reset, LV_OBJ_FLAG_CLICKABLE);
    squish_on_press(tw.reset, 64);
    lv_obj_align(tw.reset, LV_ALIGN_CENTER, 38, 54);
    lv_obj_t *ri = text(tw.reset, F.icon_m, P.on_surface, theme_icon("restart_alt"));
    lv_obj_center(ri);
    lv_obj_add_event_cb(tw.reset, timer_button_event, LV_EVENT_CLICKED, (void *)1);

    if (!tmr.pomodoro) {
        int presets[3] = {5, 15, 25}, n = 3;
        cJSON *mins = cJSON_GetObjectItem(pg, "minutes");
        if (cJSON_IsArray(mins) && cJSON_GetArraySize(mins) > 0) {
            n = 0;
            cJSON *mm;
            cJSON_ArrayForEach(mm, mins) if (cJSON_IsNumber(mm) && n < 3) presets[n++] = mm->valueint;
        }
        lv_obj_t *row = box(tile);
        tw.presets = row; // hidden while a countdown is running or paused
        lv_obj_set_size(row, LV_SIZE_CONTENT, 34);
        lv_obj_set_flex_flow(row, LV_FLEX_FLOW_ROW);
        lv_obj_set_style_pad_column(row, 8, 0);
        lv_obj_align(row, LV_ALIGN_CENTER, 0, 112);
        for (int i = 0; i < n; i++) {
            lv_obj_t *chip = box(row);
            lv_obj_set_size(chip, 46, 34);
            lv_obj_set_style_radius(chip, 17, 0);
            lv_obj_set_style_bg_color(chip, P.secc, 0);
            lv_obj_set_style_bg_opa(chip, LV_OPA_COVER, 0);
            lv_obj_add_flag(chip, LV_OBJ_FLAG_CLICKABLE);
            lv_obj_set_style_bg_color(chip, lv_color_mix(lv_color_white(), P.secc, 30), LV_STATE_PRESSED);
            char b[6];
            snprintf(b, sizeof(b), "%d", presets[i]);
            lv_obj_t *l = text(chip, F.label, P.on_secc, b);
            lv_obj_center(l);
            lv_obj_add_event_cb(chip, timer_preset_event, LV_EVENT_CLICKED, (void *)(intptr_t)presets[i]);
        }
    }
    timer_show();
}

// ---- app icons pushed by the daemon ---------------------------------------------------------

static struct {
    char name[128]; // icon names can be avatar URLs
    lv_image_dsc_t dsc;
} icon_cache[64];
static int nicons;

static const lv_image_dsc_t *icon_get(const char *name)
{
    for (int i = 0; name && i < nicons; i++)
        if (!strcmp(icon_cache[i].name, name)) return &icon_cache[i].dsc;
    return NULL;
}

void ui_icon(const char *name, int w, int h, const uint8_t *data, size_t len)
{
    if (!name || len != (size_t)w * h * 3 || icon_get(name) || nicons >= 64) return;
    uint8_t *px = heap_caps_malloc(len, MALLOC_CAP_SPIRAM);
    if (!px) return;
    memcpy(px, data, len);
    lv_image_dsc_t *d = &icon_cache[nicons].dsc;
    strlcpy(icon_cache[nicons++].name, name, sizeof(icon_cache[0].name));
    memset(d, 0, sizeof(*d));
    d->header.magic = LV_IMAGE_HEADER_MAGIC;
    d->header.cf = LV_COLOR_FORMAT_RGB565A8;
    d->header.w = w;
    d->header.h = h;
    d->header.stride = w * 2;
    d->data = px;
    d->data_size = len;
}

// An app icon, or a glyph when the PC had none.
static lv_obj_t *app_icon(lv_obj_t *parent, const char *name, const char *fallback, lv_color_t fg, int zoom256)
{
    const lv_image_dsc_t *d = icon_get(name);
    if (d) {
        lv_obj_t *im = lv_image_create(parent);
        lv_image_set_src(im, d);
        if (zoom256 != 256) lv_image_set_scale(im, zoom256);
        lv_obj_remove_flag(im, LV_OBJ_FLAG_CLICKABLE);
        return im;
    }
    return text(parent, F.icon_m, fg, theme_icon(fallback));
}

static void send_simple(const char *t, const char *key, const char *sval, int ival)
{
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", t);
    if (sval) cJSON_AddStringToObject(o, key, sval);
    else if (key) cJSON_AddNumberToObject(o, key, ival);
    char *js = cJSON_PrintUnformatted(o);
    if (js) usb_send_line(js);
    free(js);
    cJSON_Delete(o);
}

// A vertically scrolling column inside the round screen.
static lv_obj_t *scroll_column(lv_obj_t *tile, int w, int h, int y)
{
    lv_obj_t *col = lv_obj_create(tile);
    lv_obj_remove_style_all(col);
    lv_obj_set_size(col, w, h);
    lv_obj_align(col, LV_ALIGN_CENTER, 0, y);
    lv_obj_set_flex_flow(col, LV_FLEX_FLOW_COLUMN);
    lv_obj_set_flex_align(col, LV_FLEX_ALIGN_START, LV_FLEX_ALIGN_CENTER, LV_FLEX_ALIGN_CENTER);
    lv_obj_set_style_pad_row(col, 8, 0);
    lv_obj_set_style_pad_ver(col, 40, 0);
    lv_obj_set_scroll_dir(col, LV_DIR_VER);
    lv_obj_set_scrollbar_mode(col, LV_SCROLLBAR_MODE_OFF);
    lv_obj_add_flag(col, LV_OBJ_FLAG_SCROLL_MOMENTUM);
    return col;
}

static lv_obj_t *card(lv_obj_t *parent, int w)
{
    lv_obj_t *c = box(parent);
    lv_obj_set_size(c, w, LV_SIZE_CONTENT);
    lv_obj_set_style_radius(c, 24, 0);
    lv_obj_set_style_bg_color(c, P.sch, 0);
    lv_obj_set_style_bg_opa(c, LV_OPA_COVER, 0);
    lv_obj_set_style_pad_all(c, 12, 0);
    lv_obj_add_flag(c, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_set_style_bg_color(c, lv_color_mix(lv_color_white(), P.sch, 25), LV_STATE_PRESSED);
    return c;
}

// ---- page: windows --------------------------------------------------------------------------

static struct {
    lv_obj_t *ring;
} win_w;

static void window_event(lv_event_t *e)
{
    const char *id = lv_event_get_user_data(e);
    send_simple("activate", "id", id, 0);
}

static void build_windows(lv_obj_t *tile)
{
    win_w.ring = box(tile);
    lv_obj_set_size(win_w.ring, LCD_W, LCD_H);
    lv_obj_center(win_w.ring);
    lv_obj_remove_flag(win_w.ring, LV_OBJ_FLAG_CLICKABLE);
    if (!usb_host_listening()) {
        lv_obj_t *l = text(win_w.ring, F.body, P.on_sv, "Connect the PC");
        lv_obj_center(l);
    }
}

static char win_ids[9][16];

// The center shape of this profile's shortcut pages, so the window page matches them.
static int shortcut_center_shape(void)
{
    for (int i = 0; i < page_count; i++) {
        if (strcmp(page_kind(pages[i]), "radial")) continue;
        cJSON *c = cJSON_GetObjectItem(pages[i], "center");
        if (c) return shape_id(jstr(c, "shape"), SH_SQUARE);
    }
    return SH_SQUARE;
}

static lv_obj_t *window_button(int k, cJSON *w, int size, bool center)
{
    strlcpy(win_ids[k], jstr(w, "id") ?: "", sizeof(win_ids[0]));
    lv_color_t bg = center ? P.pc : P.schh;
    lv_obj_t *b = mask_button(win_w.ring, center ? shortcut_center_shape() : SH_CIRCLE, size, bg);
    // icon sizes follow the shortcut glyphs (ring 32 px, center 44 px) a bit larger for colour icons
    lv_obj_t *ic = app_icon(b, jstr(w, "icon"), "select_window", center ? P.on_pc : P.on_surface, center ? 300 : 192);
    lv_obj_center(ic);
    lv_obj_add_event_cb(b, window_event, LV_EVENT_CLICKED, win_ids[k]);
    return b;
}

// 1:1 with a full shortcut page: 8 ring slots (72 px at radius 128) and the
// 112 px center. The most recent window you're not in takes the center (a quick
// switch back), the next most recent fill the ring from 12 o'clock clockwise.
void ui_windows(cJSON *list)
{
    if (!win_w.ring) return;
    lv_obj_clean(win_w.ring);
    int total = cJSON_GetArraySize(list);
    int first = total > 1 ? 1 : 0; // list[0] is the focused window: already in use
    cJSON *center = cJSON_GetArrayItem(list, first);
    int n = LV_MAX(0, LV_MIN(total - first - 1, 8));
    for (int i = 0; i < n; i++) {
        int16_t ang = (int16_t)(270 + i * 45) % 360;
        lv_obj_t *b = window_button(i + 1, cJSON_GetArrayItem(list, first + 1 + i), 72, false);
        lv_obj_align(b, LV_ALIGN_CENTER, 128 * lv_trigo_cos(ang) / LV_TRIGO_SIN_MAX, 128 * lv_trigo_sin(ang) / LV_TRIGO_SIN_MAX);
    }
    if (center) lv_obj_center(window_button(0, center, 112, true));
    else lv_obj_center(text(win_w.ring, F.body, P.on_sv, "No windows"));
}

// ---- page: mixer ----------------------------------------------------------------------------

static struct {
    lv_obj_t *col;
    bool sliding;
    char ids[12][16];
    char outs[6][96];
    uint32_t last_send;
} mix_w;

static void mixer_slider_event(lv_event_t *e)
{
    lv_event_code_t c = lv_event_get_code(e);
    const char *id = lv_event_get_user_data(e);
    lv_obj_t *sl = lv_event_get_target_obj(e);
    if (c == LV_EVENT_PRESSED) mix_w.sliding = true;
    if (c == LV_EVENT_VALUE_CHANGED && lv_tick_get() - mix_w.last_send < 80) return; // ≤ ~12 updates/s
    if (c == LV_EVENT_VALUE_CHANGED || c == LV_EVENT_RELEASED) {
        mix_w.last_send = lv_tick_get();
        cJSON *o = cJSON_CreateObject();
        cJSON_AddStringToObject(o, "t", "mixerSet");
        cJSON_AddStringToObject(o, "id", id);
        cJSON_AddNumberToObject(o, "vol", lv_slider_get_value(sl));
        char *js = cJSON_PrintUnformatted(o);
        if (js) usb_send_line(js);
        free(js);
        cJSON_Delete(o);
    }
    if (c == LV_EVENT_RELEASED || c == LV_EVENT_PRESS_LOST) mix_w.sliding = false;
}

static void mixer_mute_event(lv_event_t *e) { send_simple("mixerMute", "id", lv_event_get_user_data(e), 0); }

static void build_mixer(lv_obj_t *tile)
{
    mix_w.col = scroll_column(tile, 280, LCD_H, 0);
    lv_obj_t *l = text(mix_w.col, F.body, P.on_sv, usb_host_listening() ? "" : "Connect the PC");
    (void)l;
}

void ui_mixer(cJSON *m)
{
    if (!mix_w.col || mix_w.sliding) return;
    int32_t scroll = lv_obj_get_scroll_y(mix_w.col);
    lv_obj_clean(mix_w.col);
    lv_obj_t *head = text(mix_w.col, F.body, P.on_sv, "Volume mixer");
    (void)head;
    // apps
    cJSON *apps = cJSON_GetObjectItem(m, "apps"), *a;
    int k = 0;
    cJSON_ArrayForEach(a, apps)
    {
        if (k >= 12) break;
        strlcpy(mix_w.ids[k], jstr(a, "id") ?: "", sizeof(mix_w.ids[0]));
        bool muted = cJSON_IsTrue(cJSON_GetObjectItem(a, "muted"));
        lv_obj_t *c = card(mix_w.col, 260);
        lv_obj_remove_flag(c, LV_OBJ_FLAG_CLICKABLE);
        lv_obj_set_style_pad_all(c, 10, 0);
        lv_obj_t *ib = box(c);
        lv_obj_set_size(ib, 48, 48);
        lv_obj_set_style_radius(ib, 24, 0);
        lv_obj_set_style_bg_color(ib, muted ? P.ec : P.schh, 0);
        lv_obj_set_style_bg_opa(ib, LV_OPA_COVER, 0);
        lv_obj_add_flag(ib, LV_OBJ_FLAG_CLICKABLE);
        lv_obj_t *ic = muted ? text(ib, F.icon_s, P.on_ec, theme_icon("volume_off"))
                             : app_icon(ib, jstr(a, "icon"), "music_note", P.on_surface, 213); // 40 px in a 48 px circle
        lv_obj_center(ic);
        lv_obj_add_event_cb(ib, mixer_mute_event, LV_EVENT_CLICKED, mix_w.ids[k]);
        lv_obj_t *nm = text(c, F.label, P.on_surface, jstr(a, "name") ?: "");
        lv_obj_set_width(nm, 180);
        lv_label_set_long_mode(nm, LV_LABEL_LONG_DOT);
        lv_obj_set_style_text_align(nm, LV_TEXT_ALIGN_LEFT, 0);
        lv_obj_align(nm, LV_ALIGN_TOP_LEFT, 58, 2);
        lv_obj_t *sl = lv_slider_create(c);
        lv_obj_set_size(sl, 176, 10);
        lv_obj_align(sl, LV_ALIGN_TOP_LEFT, 60, 30);
        lv_slider_set_range(sl, 0, 100);
        cJSON *v = cJSON_GetObjectItem(a, "vol");
        lv_slider_set_value(sl, cJSON_IsNumber(v) ? v->valueint : 0, LV_ANIM_OFF);
        lv_obj_set_style_bg_color(sl, P.schh, LV_PART_MAIN);
        lv_obj_set_style_bg_color(sl, P.primary, LV_PART_INDICATOR);
        lv_obj_set_style_bg_color(sl, P.primary, LV_PART_KNOB);
        lv_obj_set_style_pad_all(sl, 4, LV_PART_KNOB);
        lv_obj_set_ext_click_area(sl, 12);
        lv_obj_add_event_cb(sl, mixer_slider_event, LV_EVENT_ALL, mix_w.ids[k]);
        k++;
    }
    if (k == 0) text(mix_w.col, F.body, P.on_sv, "Nothing is playing");
    lv_obj_update_layout(mix_w.col);
    lv_obj_scroll_to_y(mix_w.col, scroll, LV_ANIM_OFF);
}

// ---- page: clipboard ------------------------------------------------------------------------

static lv_obj_t *clip_col;

static void clip_event(lv_event_t *e)
{
    send_simple("paste", "i", NULL, (int)(intptr_t)lv_event_get_user_data(e));
    show_bubble("Pasted");
}

static void build_clipboard(lv_obj_t *tile)
{
    clip_col = scroll_column(tile, 280, LCD_H, 0);
    text(clip_col, F.body, P.on_sv, usb_host_listening() ? "" : "Connect the PC");
}

void ui_clips(cJSON *list)
{
    if (!clip_col) return;
    lv_obj_clean(clip_col);
    text(clip_col, F.body, P.on_sv, "Clipboard");
    int i = 0;
    cJSON *it;
    cJSON_ArrayForEach(it, list)
    {
        if (!cJSON_IsString(it) || i >= 10) continue;
        lv_obj_t *c = card(clip_col, 250);
        lv_obj_t *l = text(c, F.label, P.on_surface, it->valuestring);
        lv_obj_set_width(l, 226);
        lv_obj_set_style_text_align(l, LV_TEXT_ALIGN_LEFT, 0);
        lv_obj_set_style_max_height(l, 40, 0);
        lv_label_set_long_mode(l, LV_LABEL_LONG_DOT);
        lv_obj_add_event_cb(c, clip_event, LV_EVENT_CLICKED, (void *)(intptr_t)i);
        i++;
    }
    if (i == 0) text(clip_col, F.body, P.on_sv, "Nothing copied yet");
}

// ---- notification panel (pull up from the bottom) -------------------------------------------

static cJSON *notifs; // last list from the PC
static lv_obj_t *np, *np_col;
static char np_ids[20][12];

static void np_close(void)
{
    if (!np || lv_obj_has_flag(np, LV_OBJ_FLAG_HIDDEN)) return;
    lv_anim_t a;
    lv_anim_init(&a);
    lv_anim_set_var(&a, np);
    lv_anim_set_values(&a, lv_obj_get_y(np), LCD_H);
    lv_anim_set_duration(&a, 220);
    lv_anim_set_path_cb(&a, lv_anim_path_ease_in);
    lv_anim_set_exec_cb(&a, qs_y_exec);
    lv_anim_set_completed_cb(&a, qs_hide_done);
    lv_anim_start(&a);
}

static void np_fill(void);

static void np_card_event(lv_event_t *e)
{
    const char *id = lv_event_get_user_data(e);
    lv_event_code_t c = lv_event_get_code(e);
    if (c == LV_EVENT_GESTURE) {
        lv_dir_t d = lv_indev_get_gesture_dir(lv_indev_active());
        if (d == LV_DIR_LEFT || d == LV_DIR_RIGHT) {
            send_simple("notifDismiss", "id", id, 0);
            lv_obj_t *card_obj = lv_event_get_current_target_obj(e);
            lv_obj_add_flag(card_obj, LV_OBJ_FLAG_HIDDEN); // the refreshed list follows
            if (indev) lv_indev_wait_release(indev);
        }
    } else if (c == LV_EVENT_SHORT_CLICKED) {
        send_simple("notifOpen", "id", id, 0);
        np_close();
    }
}

static void np_event(lv_event_t *e)
{
    lv_event_code_t c = lv_event_get_code(e);
    if (c == LV_EVENT_GESTURE && lv_indev_get_gesture_dir(lv_indev_active()) == LV_DIR_BOTTOM) np_close();
    else if (c == LV_EVENT_CLICKED && (intptr_t)lv_event_get_user_data(e) == 1) {
        send_simple("notifClear", NULL, NULL, 0);
        np_close();
    } else if (c == LV_EVENT_CLICKED && (intptr_t)lv_event_get_user_data(e) == 2) {
        np_close();
    }
}

static void np_fill(void)
{
    if (!np_col) return;
    lv_obj_clean(np_col);
    text(np_col, F.body, P.on_sv, "Notifications");
    int i = 0;
    cJSON *n;
    cJSON_ArrayForEach(n, notifs)
    {
        if (i >= 20) break;
        strlcpy(np_ids[i], jstr(n, "id") ?: "", sizeof(np_ids[0]));
        lv_obj_t *c = card(np_col, 270);
        lv_obj_remove_flag(c, LV_OBJ_FLAG_GESTURE_BUBBLE);
        // small app icon + app name on one line, then title and body below
        lv_obj_t *ib = box(c);
        lv_obj_set_size(ib, 20, 20);
        lv_obj_align(ib, LV_ALIGN_TOP_LEFT, 0, 0);
        lv_obj_remove_flag(ib, LV_OBJ_FLAG_CLICKABLE);
        lv_obj_t *ic = text(ib, F.icon_xs, P.on_sv, theme_icon("info")); // same small icon on every card
        lv_obj_center(ic);
        lv_obj_t *app = text(c, F.label, P.on_sv, jstr(n, "app") ?: "");
        lv_obj_align(app, LV_ALIGN_TOP_LEFT, 30, 2);
        lv_obj_t *ti = text(c, F.body, P.on_surface, jstr(n, "title") ?: "");
        lv_obj_set_width(ti, 246);
        lv_label_set_long_mode(ti, LV_LABEL_LONG_DOT);
        lv_obj_set_style_text_align(ti, LV_TEXT_ALIGN_LEFT, 0);
        lv_obj_align(ti, LV_ALIGN_TOP_LEFT, 0, 30);
        const char *body = jstr(n, "body");
        if (body && body[0]) {
            lv_obj_t *bo = text(c, F.label, P.on_sv, body);
            lv_obj_set_width(bo, 246);
            lv_obj_set_style_max_height(bo, 38, 0);
            lv_label_set_long_mode(bo, LV_LABEL_LONG_DOT);
            lv_obj_set_style_text_align(bo, LV_TEXT_ALIGN_LEFT, 0);
            lv_obj_align(bo, LV_ALIGN_TOP_LEFT, 0, 54);
        }
        lv_obj_add_event_cb(c, np_card_event, LV_EVENT_SHORT_CLICKED, np_ids[i]);
        lv_obj_add_event_cb(c, np_card_event, LV_EVENT_GESTURE, np_ids[i]);
        i++;
    }
    if (i == 0) {
        text(np_col, F.body, P.on_sv, "No notifications");
    } else {
        lv_obj_t *clear = box(np_col);
        lv_obj_set_size(clear, 140, 40);
        lv_obj_set_style_radius(clear, 20, 0);
        lv_obj_set_style_bg_color(clear, P.secc, 0);
        lv_obj_set_style_bg_opa(clear, LV_OPA_COVER, 0);
        lv_obj_add_flag(clear, LV_OBJ_FLAG_CLICKABLE);
        lv_obj_t *cl = text(clear, F.label, P.on_secc, "Clear all");
        lv_obj_center(cl);
        lv_obj_add_event_cb(clear, np_event, LV_EVENT_CLICKED, (void *)1);
    }
}

static void np_open(void)
{
    if (power != AWAKE && power != DIM) return;
    if (np) lv_obj_delete(np);
    np = box(lv_layer_top());
    lv_obj_set_size(np, LCD_W, LCD_H);
    lv_obj_set_style_radius(np, LV_RADIUS_CIRCLE, 0);
    lv_obj_set_style_bg_color(np, P.sc, 0);
    lv_obj_set_style_bg_opa(np, LV_OPA_COVER, 0);
    lv_obj_add_flag(np, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_add_event_cb(np, np_event, LV_EVENT_GESTURE, NULL);
    np_col = scroll_column(np, 290, LCD_H, 0);
    lv_obj_set_style_pad_top(np_col, 64, 0);
    np_fill();
    lv_obj_t *close = box(np); // always reachable, above the scrolling list
    lv_obj_set_size(close, 64, 40);
    lv_obj_align(close, LV_ALIGN_TOP_MID, 0, 8);
    lv_obj_set_style_radius(close, 20, 0);
    lv_obj_set_style_bg_color(close, P.schh, 0);
    lv_obj_set_style_bg_opa(close, LV_OPA_COVER, 0);
    lv_obj_add_flag(close, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_t *ci = text(close, F.icon_m, P.on_surface, theme_icon("expand_more"));
    lv_obj_center(ci);
    lv_obj_add_event_cb(close, np_event, LV_EVENT_CLICKED, (void *)2);
    lv_obj_set_y(np, LCD_H);
    lv_anim_t a;
    lv_anim_init(&a);
    lv_anim_set_var(&a, np);
    lv_anim_set_values(&a, LCD_H, 0);
    lv_anim_set_duration(&a, 280);
    lv_anim_set_path_cb(&a, lv_anim_path_ease_out);
    lv_anim_set_exec_cb(&a, qs_y_exec);
    lv_anim_start(&a);
    if (indev) {
        arc_mode = ARC_IDLE;
        dragging = false;
        tp.mode = TP_IDLE;
        lv_indev_reset(indev, NULL);
        lv_indev_wait_release(indev);
    }
}

void ui_notifs(cJSON *list)
{
    cJSON_Delete(notifs);
    notifs = cJSON_Duplicate(list, true);
    if (np && !lv_obj_has_flag(np, LV_OBJ_FLAG_HIDDEN)) np_fill();
}

void ui_notif_toast(cJSON *n)
{
    if (power == OFF || power == ASLEEP) return;
    char buf[120];
    const char *app = jstr(n, "app"), *title = jstr(n, "title");
    snprintf(buf, sizeof(buf), "%s%s%s", app ?: "", app && title ? " · " : "", title ?: "");
    ui_toast(buf);
}



// ---- page: OBS ---------------------------------------------------------------------------

static lv_obj_t *obs_root, *obs_time;
static char obs_scenes[8][64];
static uint32_t obs_sig;

static void obs_cmd(const char *request, const char *scene)
{
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", "obsCmd");
    cJSON_AddStringToObject(o, "request", request);
    if (scene) cJSON_AddStringToObject(o, "scene", scene);
    char *js = cJSON_PrintUnformatted(o);
    if (js) usb_send_line(js);
    free(js);
    cJSON_Delete(o);
}

static void obs_record_event(lv_event_t *e) { obs_cmd("ToggleRecord", NULL); }
static void obs_scene_event(lv_event_t *e) { obs_cmd("SetScene", lv_event_get_user_data(e)); }

static void build_obs(lv_obj_t *tile)
{
    obs_root = box(tile);
    lv_obj_set_size(obs_root, LCD_W, LCD_H);
    lv_obj_center(obs_root);
    lv_obj_remove_flag(obs_root, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_t *l = text(obs_root, F.body, P.on_sv, usb_host_listening() ? "OBS…" : "Connect the PC");
    lv_obj_center(l);
}

static void obs_time_set(cJSON *st)
{
    cJSON *t = cJSON_GetObjectItem(st, "recTime");
    int s = cJSON_IsNumber(t) ? t->valueint : 0;
    if (s >= 3600) lv_label_set_text_fmt(obs_time, "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60);
    else lv_label_set_text_fmt(obs_time, "%d:%02d", s / 60, s % 60);
}

void ui_obs(cJSON *st)
{
    if (!obs_root) return;
    // Rebuild only when something other than the recording clock changed:
    // the daemon sends a tick every second while recording.
    cJSON *rt = cJSON_DetachItemFromObject(st, "recTime");
    char *js = cJSON_PrintUnformatted(st);
    uint32_t sig = 2166136261u;
    for (const char *p = js; p && *p; p++) sig = (sig ^ (uint8_t)*p) * 16777619u;
    free(js);
    if (rt) cJSON_AddItemToObject(st, "recTime", rt);
    if (sig == obs_sig && obs_time) {
        obs_time_set(st);
        return;
    }
    obs_sig = sig;
    obs_time = NULL;
    lv_obj_clean(obs_root);
    if (!cJSON_IsTrue(cJSON_GetObjectItem(st, "connected"))) {
        lv_obj_t *ic = text(obs_root, F.icon_xl, P.on_sv, theme_icon("videocam"));
        lv_obj_align(ic, LV_ALIGN_CENTER, 0, -26);
        lv_obj_t *l = text(obs_root, F.body, P.on_sv, "Open OBS (set it up in the configurator)");
        lv_obj_set_width(l, 220);
        lv_label_set_long_mode(l, LV_LABEL_LONG_WRAP);
        lv_obj_align(l, LV_ALIGN_CENTER, 0, 36);
        return;
    }
    bool rec = cJSON_IsTrue(cJSON_GetObjectItem(st, "recording")), live_on = cJSON_IsTrue(cJSON_GetObjectItem(st, "streaming"));
    const char *current = jstr(st, "current");
    // scenes on the ring, like a shortcut page
    cJSON *scenes = cJSON_GetObjectItem(st, "scenes");
    int n = LV_MIN(cJSON_GetArraySize(scenes), 8);
    for (int i = 0; i < n; i++) {
        const char *name = cJSON_GetStringValue(cJSON_GetArrayItem(scenes, i));
        if (!name) continue;
        strlcpy(obs_scenes[i], name, sizeof(obs_scenes[0]));
        bool on = current && !strcmp(current, name);
        // symmetric around the top, leaving the bottom 60° free for the page dots
        int step = n > 1 ? LV_MIN(300 / (n - 1), 360 / n) : 0;
        int16_t ang = (int16_t)((270 - step * (n - 1) / 2 + i * step + 360) % 360);
        lv_obj_t *b = mask_button(obs_root, SH_CIRCLE, 78, on ? P.pc : P.schh);
        lv_obj_align(b, LV_ALIGN_CENTER, 128 * lv_trigo_cos(ang) / LV_TRIGO_SIN_MAX, 128 * lv_trigo_sin(ang) / LV_TRIGO_SIN_MAX);
        lv_obj_t *l = text(b, F.label, on ? P.on_pc : P.on_surface, name);
        lv_obj_set_size(l, 68, 2 * lv_font_get_line_height(F.label));
        lv_label_set_long_mode(l, LV_LABEL_LONG_DOT);
        lv_obj_set_style_text_align(l, LV_TEXT_ALIGN_CENTER, 0);
        lv_obj_center(l);
        lv_obj_add_event_cb(b, obs_scene_event, LV_EVENT_CLICKED, obs_scenes[i]);
    }
    // record button in the center
    lv_obj_t *recb = mask_button(obs_root, shortcut_center_shape(), 104, rec ? P.ec : P.schh);
    lv_obj_center(recb);
    lv_obj_t *ri = text(recb, F.icon_l, rec ? P.on_ec : P.on_surface, theme_icon("videocam"));
    lv_obj_align(ri, LV_ALIGN_CENTER, 0, rec ? -8 : 0);
    if (rec) {
        obs_time = text(recb, F.label, P.on_ec, "");
        obs_time_set(st);
        lv_obj_align(obs_time, LV_ALIGN_CENTER, 0, 24);
    }
    lv_obj_add_event_cb(recb, obs_record_event, LV_EVENT_CLICKED, NULL);
    if (live_on) { // streaming: a small label at the top of the record button
        lv_obj_t *lv = text(recb, F.label, rec ? P.on_ec : P.error, "● LIVE");
        lv_obj_align(lv, LV_ALIGN_CENTER, 0, rec ? -36 : -30);
        if (!rec) lv_obj_align(ri, LV_ALIGN_CENTER, 0, 6);
    }
}

// ---- page: game --------------------------------------------------------------------------

static struct {
    lv_obj_t *art, *name, *session;
    lv_obj_t *close, *close_icon, *close_label;
    bool close_armed;
    int64_t since;
} gw;
static uint8_t *game_blend;
static const uint8_t *game_src;
static int game_w, game_h;
static lv_image_dsc_t game_dsc;

static void game_art_apply(void)
{
    if (!game_src || !gw.art) return;
    if (!game_blend) game_blend = heap_caps_malloc(360 * 360 * 2, MALLOC_CAP_SPIRAM);
    if (!game_blend) return;
    blend_scrim((const uint16_t *)game_src, (uint16_t *)game_blend, (size_t)game_w * game_h, P.surface, cover_scrim_pct());
    game_dsc.header.magic = LV_IMAGE_HEADER_MAGIC;
    game_dsc.header.cf = LV_COLOR_FORMAT_RGB565;
    game_dsc.header.w = game_w;
    game_dsc.header.h = game_h;
    game_dsc.header.stride = game_w * 2;
    game_dsc.data_size = (uint32_t)game_w * game_h * 2;
    game_dsc.data = game_blend;
    lv_image_cache_drop(&game_dsc);
    lv_image_set_src(gw.art, &game_dsc);
    lv_obj_remove_flag(gw.art, LV_OBJ_FLAG_HIDDEN);
}

void ui_game_art(const uint8_t *rgb565, int w, int h)
{
    game_src = rgb565;
    game_w = w;
    game_h = h;
    game_art_apply();
}

static void game_session_update(void)
{
    if (!gw.session || !clk.valid || !gw.since) return;
    int m = (int)((now_utc() - gw.since) / 60);
    if (m < 60) lv_label_set_text_fmt(gw.session, "%d min", m);
    else lv_label_set_text_fmt(gw.session, "%d h %02d min", m / 60, m % 60);
}

void ui_game(cJSON *msg)
{
    if (!gw.name) return;
    const char *name = jstr(msg, "name");
    bool playing = name && name[0];
    lv_label_set_text(gw.name, playing ? name : "No game running");
    cJSON *since = cJSON_GetObjectItem(msg, "since");
    gw.since = playing && cJSON_IsNumber(since) ? (int64_t)since->valuedouble : 0;
    if (!playing) lv_label_set_text(gw.session, "Start a Steam game");
    game_session_update();
    game_place_name();
}

static void game_place_name(void)
{
    // two lines at most, top pinned high; the session time follows below
    lv_label_set_long_mode(gw.name, LV_LABEL_LONG_WRAP);
    lv_obj_set_height(gw.name, LV_SIZE_CONTENT);
    lv_obj_update_layout(gw.name);
    int two = 2 * lv_font_get_line_height(F.value);
    if (lv_obj_get_height(gw.name) > two) {
        lv_obj_set_height(gw.name, two);
        lv_label_set_long_mode(gw.name, LV_LABEL_LONG_DOT);
    }
    lv_obj_align(gw.name, LV_ALIGN_TOP_MID, 0, 74);
    lv_obj_align_to(gw.session, gw.name, LV_ALIGN_OUT_BOTTOM_MID, 0, 4);
}

static void game_key_event(lv_event_t *e)
{
    const char *keys = lv_event_get_user_data(e);
    usb_hid(HID_KEYS, keys, NULL);
    show_bubble(keys[0] == 's' ? "Steam overlay" : "Screenshot");
}

static lv_obj_t *game_button(lv_obj_t *tile, const char *icon, const char *keys, int x)
{
    lv_obj_t *b = mask_button(tile, SH_CIRCLE, 68, P.schh);
    lv_obj_align(b, LV_ALIGN_CENTER, x, 88);
    lv_obj_t *ic = text(b, F.icon_m, P.on_surface, theme_icon(icon));
    lv_obj_center(ic);
    if (keys) lv_obj_add_event_cb(b, game_key_event, LV_EVENT_CLICKED, (void *)keys);
    return b;
}

// Close game: the first tap arms it ("Sure?" in red for 3 s), the second closes.
static lv_timer_t *game_close_timer; // persistent, paused between uses

static void game_close_set(bool armed)
{
    gw.close_armed = armed;
    if (!gw.close) return;
    lv_color_t bg = armed ? P.ec : P.schh;
    lv_obj_set_style_image_recolor(gw.close, bg, 0);
    lv_obj_set_style_image_recolor(gw.close, lv_color_mix(lv_color_white(), bg, 30), LV_STATE_PRESSED);
    if (armed) {
        lv_obj_add_flag(gw.close_icon, LV_OBJ_FLAG_HIDDEN);
        lv_obj_remove_flag(gw.close_label, LV_OBJ_FLAG_HIDDEN);
    } else {
        lv_obj_remove_flag(gw.close_icon, LV_OBJ_FLAG_HIDDEN);
        lv_obj_add_flag(gw.close_label, LV_OBJ_FLAG_HIDDEN);
    }
}

static void game_close_timeout(lv_timer_t *t)
{
    lv_timer_pause(t);
    game_close_set(false);
}

static void game_close_event(lv_event_t *e)
{
    if (!gw.close_armed) {
        game_close_set(true);
        lv_timer_reset(game_close_timer);
        lv_timer_resume(game_close_timer);
        return;
    }
    lv_timer_pause(game_close_timer);
    game_close_set(false);
    if (!usb_host_listening()) {
        show_bubble("PC app not running");
        return;
    }
    send_simple("gameClose", NULL, NULL, 0);
    show_bubble("Closing the game");
}

static void build_game(lv_obj_t *tile)
{
    gw.art = lv_image_create(tile);
    lv_obj_center(gw.art);
    lv_obj_remove_flag(gw.art, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_add_flag(gw.art, LV_OBJ_FLAG_HIDDEN);
    gw.session = text(tile, F.body, P.on_sv, "");
    gw.name = text(tile, F.value, P.on_surface, usb_host_listening() ? "" : "Connect the PC");
    lv_obj_set_width(gw.name, 270);
    lv_obj_set_style_text_align(gw.name, LV_TEXT_ALIGN_CENTER, 0);
    game_place_name();
    // Steam's own shortcuts, typed by the device's keyboard, and close game
    game_button(tile, "layers", "shift+tab", -78);
    game_button(tile, "photo_camera", "f12", 0);
    gw.close = game_button(tile, "close", NULL, 78);
    gw.close_icon = lv_obj_get_child(gw.close, 0);
    gw.close_label = text(gw.close, F.label, P.on_ec, "Sure?");
    lv_obj_center(gw.close_label);
    lv_obj_add_flag(gw.close_label, LV_OBJ_FLAG_HIDDEN);
    lv_obj_add_event_cb(gw.close, game_close_event, LV_EVENT_CLICKED, NULL);
    if (!game_close_timer) {
        game_close_timer = lv_timer_create(game_close_timeout, 3000, NULL);
    }
    lv_timer_pause(game_close_timer);
    if (game_src) game_art_apply();
}

// ---- page: MangoHud ------------------------------------------------------------------------

#define MH_GRAPH 60
static struct {
    lv_obj_t *fps, *fps_cap, *avg, *ft, *top, *hint;
    lv_obj_t *arc[2], *val[2], *temp[2];
    lv_obj_t *chart;
    lv_chart_series_t *ser;
} mh;

static void mh_side(lv_obj_t *tile, int i, const char *caption)
{
    bool right = i == 1;
    lv_obj_t *a = lv_arc_create(tile);
    lv_obj_set_size(a, 344, 344);
    lv_obj_center(a);
    if (right) {
        lv_arc_set_bg_angles(a, 320, 40);
        lv_arc_set_mode(a, LV_ARC_MODE_REVERSE); // fills bottom-up like the left one
    } else {
        lv_arc_set_bg_angles(a, 140, 220);
    }
    lv_arc_set_range(a, 0, 100);
    lv_arc_set_value(a, 0);
    lv_obj_set_style_arc_width(a, 10, LV_PART_MAIN);
    lv_obj_set_style_arc_width(a, 10, LV_PART_INDICATOR);
    lv_obj_set_style_arc_color(a, P.schh, LV_PART_MAIN);
    lv_obj_set_style_arc_color(a, right ? P.tertiary : P.primary, LV_PART_INDICATOR);
    lv_obj_set_style_arc_rounded(a, true, LV_PART_INDICATOR);
    lv_obj_set_style_bg_opa(a, LV_OPA_TRANSP, LV_PART_KNOB);
    lv_obj_remove_flag(a, LV_OBJ_FLAG_CLICKABLE);
    mh.arc[i] = a;
    int x = right ? 124 : -124;
    mh.val[i] = text(tile, F.body, P.on_surface, "–");
    lv_obj_align(mh.val[i], LV_ALIGN_CENTER, x, -18);
    mh.temp[i] = text(tile, F.label, P.on_sv, "");
    lv_obj_align(mh.temp[i], LV_ALIGN_CENTER, x, 2);
    lv_obj_t *cap = text(tile, F.label, P.on_sv, caption);
    lv_obj_align(cap, LV_ALIGN_CENTER, x, 22);
}

static void build_mangohud(lv_obj_t *tile)
{
    memset(&mh, 0, sizeof(mh));
    mh.top = text(tile, F.label, P.on_sv, "");
    lv_obj_align(mh.top, LV_ALIGN_CENTER, 0, -118);
    mh.fps = text(tile, F.big, P.on_surface, "–");
    lv_obj_align(mh.fps, LV_ALIGN_CENTER, 0, -54);
    mh.fps_cap = text(tile, F.label, P.on_sv, "FPS");
    lv_obj_align(mh.fps_cap, LV_ALIGN_CENTER, 0, -16);
    mh.avg = text(tile, F.label, P.on_surface, "");
    lv_obj_align(mh.avg, LV_ALIGN_CENTER, 0, 8);
    mh_side(tile, 0, "CPU");
    mh_side(tile, 1, "GPU");

    mh.chart = lv_chart_create(tile);
    lv_obj_set_size(mh.chart, 176, 52);
    lv_obj_align(mh.chart, LV_ALIGN_CENTER, 0, 62);
    lv_chart_set_type(mh.chart, LV_CHART_TYPE_LINE);
    lv_chart_set_point_count(mh.chart, MH_GRAPH);
    lv_chart_set_div_line_count(mh.chart, 0, 0);
    lv_chart_set_update_mode(mh.chart, LV_CHART_UPDATE_MODE_SHIFT);
    lv_obj_set_style_bg_opa(mh.chart, LV_OPA_TRANSP, 0);
    lv_obj_set_style_border_width(mh.chart, 0, 0);
    lv_obj_set_style_pad_all(mh.chart, 0, 0);
    lv_obj_set_style_line_width(mh.chart, 2, LV_PART_ITEMS);
    lv_obj_set_style_line_rounded(mh.chart, true, LV_PART_ITEMS);
    lv_obj_set_style_size(mh.chart, 0, 0, LV_PART_INDICATOR); // no point dots
    lv_obj_remove_flag(mh.chart, LV_OBJ_FLAG_CLICKABLE);
    mh.ser = lv_chart_add_series(mh.chart, P.primary, LV_CHART_AXIS_PRIMARY_Y);
    lv_chart_set_all_values(mh.chart, mh.ser, LV_CHART_POINT_NONE);
    mh.ft = text(tile, F.label, P.on_sv, "");
    lv_obj_align(mh.ft, LV_ALIGN_CENTER, 0, 104);

    mh.hint = text(tile, F.body, P.on_sv, usb_host_listening() ? "" : "Connect the PC");
    lv_obj_set_width(mh.hint, 200);
    lv_obj_set_style_text_align(mh.hint, LV_TEXT_ALIGN_CENTER, 0);
    lv_label_set_long_mode(mh.hint, LV_LABEL_LONG_WRAP);
    lv_obj_align(mh.hint, LV_ALIGN_CENTER, 0, 62);
}

static double jnum(cJSON *o, const char *k)
{
    cJSON *v = cJSON_GetObjectItem(o, k);
    return cJSON_IsNumber(v) ? v->valuedouble : 0;
}

void ui_mangohud(cJSON *m)
{
    if (!mh.fps) return;
    bool on = cJSON_IsTrue(cJSON_GetObjectItem(m, "active"));
    bool fallback = !on && cJSON_IsTrue(cJSON_GetObjectItem(m, "fallback"));
    lv_obj_t *data[] = {mh.chart, mh.ft, mh.avg, mh.top};
    for (size_t i = 0; i < sizeof(data) / sizeof(data[0]); i++) {
        if (on) lv_obj_remove_flag(data[i], LV_OBJ_FLAG_HIDDEN);
        else lv_obj_add_flag(data[i], LV_OBJ_FLAG_HIDDEN);
    }
    if (on) lv_obj_add_flag(mh.hint, LV_OBJ_FLAG_HIDDEN);
    else lv_obj_remove_flag(mh.hint, LV_OBJ_FLAG_HIDDEN);
    if (fallback) { // MangoHud can't log in this game: Roundbar's own stats, no FPS
        lv_obj_add_flag(mh.chart, LV_OBJ_FLAG_HIDDEN);
        lv_obj_add_flag(mh.ft, LV_OBJ_FLAG_HIDDEN);
        lv_obj_add_flag(mh.avg, LV_OBJ_FLAG_HIDDEN);
        lv_obj_remove_flag(mh.top, LV_OBJ_FLAG_HIDDEN);
        lv_obj_remove_flag(mh.hint, LV_OBJ_FLAG_HIDDEN);
        lv_label_set_text(mh.hint, "FPS isn't available in this game");
        lv_label_set_text(mh.fps, "–");
        lv_label_set_text_fmt(mh.top, "VRAM %d%%  ·  RAM %d%%", (int)jnum(m, "vram"), (int)jnum(m, "ram"));
        const char *load[] = {"cpu", "gpu"}, *temp[] = {"cpuTemp", "gpuTemp"};
        for (int i = 0; i < 2; i++) {
            int v = (int)jnum(m, load[i]);
            lv_arc_set_value(mh.arc[i], v);
            lv_label_set_text_fmt(mh.val[i], "%d%%", v);
            lv_label_set_text_fmt(mh.temp[i], "%d°", (int)jnum(m, temp[i]));
        }
        return;
    }
    if (!on) {
        const char *h = jstr(m, "hint");
        lv_label_set_text(mh.hint, h && !strcmp(h, "waiting")       ? "Waiting for MangoHud…"
                                   : h && !strcmp(h, "unsupported") ? "MangoHud isn't available on this PC"
                                                                    : "Start a game with MangoHud (restart games that were already running)");
        lv_label_set_text(mh.fps, "–");
        for (int i = 0; i < 2; i++) {
            lv_arc_set_value(mh.arc[i], 0);
            lv_label_set_text(mh.val[i], "–");
            lv_label_set_text(mh.temp[i], "");
        }
        return;
    }
    lv_label_set_text_fmt(mh.fps, "%d", (int)(jnum(m, "fps") + 0.5));
    lv_label_set_text_fmt(mh.avg, "avg %d  ·  1%% low %d", (int)(jnum(m, "avg") + 0.5), (int)(jnum(m, "low") + 0.5));
    char top[48];
    snprintf(top, sizeof(top), "%d W  ·  %.1f GB", (int)(jnum(m, "power") + 0.5), jnum(m, "vram"));
    lv_label_set_text(mh.top, top);
    char ft[24];
    snprintf(ft, sizeof(ft), "%.1f ms", jnum(m, "ft"));
    lv_label_set_text(mh.ft, ft);
    const char *load[] = {"cpu", "gpu"}, *temp[] = {"cpuTemp", "gpuTemp"};
    for (int i = 0; i < 2; i++) {
        int v = (int)(jnum(m, load[i]) + 0.5);
        lv_arc_set_value(mh.arc[i], v);
        lv_label_set_text_fmt(mh.val[i], "%d%%", v);
        lv_label_set_text_fmt(mh.temp[i], "%d°", (int)(jnum(m, temp[i]) + 0.5));
    }
    // frametime graph, scaled to its own peak (at least 20 ms so a flat line sits low)
    cJSON *g = cJSON_GetObjectItem(m, "graph");
    int n = LV_MIN(cJSON_GetArraySize(g), MH_GRAPH);
    double peak = 20;
    for (int i = 0; i < n; i++) peak = LV_MAX(peak, cJSON_GetArrayItem(g, i)->valuedouble);
    lv_chart_set_range(mh.chart, LV_CHART_AXIS_PRIMARY_Y, 0, (int32_t)(peak * 10 * 1.1));
    lv_chart_set_all_values(mh.chart, mh.ser, LV_CHART_POINT_NONE);
    for (int i = 0; i < n; i++)
        lv_chart_set_value_by_id(mh.chart, mh.ser, MH_GRAPH - n + i, (int32_t)(cJSON_GetArrayItem(g, i)->valuedouble * 10));
    lv_chart_refresh(mh.chart);
}

// ---- page: Discord voice --------------------------------------------------------------------

static lv_obj_t *dc_root;
static lv_obj_t *dc_av[12]; // avatars of the current layout, for in-place speaking rings
static int dc_avn, dc_av_size;
static uint32_t dc_sig;
static cJSON *dc_pending; // state that arrived mid-slide

static bool pages_moving(void) { return wrap_shot || (tv && lv_obj_is_scrolling(tv)); }

// Hash of the state without the speaking flags (those change many times a second).
static uint32_t dc_signature(cJSON *st)
{
    cJSON *copy = cJSON_Duplicate(st, 1);
    cJSON *m;
    cJSON_ArrayForEach(m, cJSON_GetObjectItem(copy, "members")) cJSON_DeleteItemFromObject(m, "speaking");
    char *js = cJSON_PrintUnformatted(copy);
    cJSON_Delete(copy);
    uint32_t h = 2166136261u;
    for (const char *p = js; p && *p; p++) h = (h ^ (uint8_t)*p) * 16777619u;
    free(js);
    return h;
}

static void dc_toggle_event(lv_event_t *e) { send_simple("discordToggle", "what", lv_event_get_user_data(e), 0); }

static lv_obj_t *dc_button(lv_obj_t *parent, int size, bool on, const char *icon, lv_color_t on_bg, lv_color_t on_fg,
                           const char *what)
{
    lv_obj_t *b = box(parent);
    lv_obj_set_size(b, size, size);
    lv_obj_set_style_radius(b, on ? size * 30 / 100 : size / 2, 0); // active = rounded square (M3E toggle)
    lv_obj_set_style_bg_color(b, on ? on_bg : P.schh, 0);
    lv_obj_set_style_bg_opa(b, LV_OPA_COVER, 0);
    lv_obj_add_flag(b, LV_OBJ_FLAG_CLICKABLE);
    squish_on_press(b, size);
    lv_obj_t *ic = text(b, size > 70 ? F.icon_l : size > 40 ? F.icon_m : F.icon_s, on ? on_fg : P.on_surface, theme_icon(icon));
    lv_obj_center(ic);
    lv_obj_add_event_cb(b, dc_toggle_event, LV_EVENT_CLICKED, (void *)what);
    return b;
}

static void build_discord(lv_obj_t *tile)
{
    dc_root = box(tile);
    lv_obj_set_size(dc_root, LCD_W, LCD_H);
    lv_obj_center(dc_root);
    lv_obj_remove_flag(dc_root, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_t *l = text(dc_root, F.body, P.on_sv, usb_host_listening() ? "Discord…" : "Connect the PC");
    lv_obj_center(l);
}

void ui_discord(cJSON *st)
{
    if (!dc_root) return;
    uint32_t sig = dc_signature(st);
    if (sig == dc_sig && dc_avn > 0) { // only who's talking changed: just the rings
        cJSON *members = cJSON_GetObjectItem(st, "members");
        for (int i = 0; i < dc_avn; i++) {
            bool speaking = cJSON_IsTrue(cJSON_GetObjectItem(cJSON_GetArrayItem(members, i), "speaking"));
            lv_obj_set_style_border_width(dc_av[i], speaking ? LV_MAX(3, dc_av_size / 16) : 0, 0);
        }
        return;
    }
    if (pages_moving()) { // rebuilding mid-slide breaks the snap: apply it when the pages stop
        if (dc_pending) cJSON_Delete(dc_pending);
        dc_pending = cJSON_Duplicate(st, 1);
        return;
    }
    dc_sig = sig;
    dc_avn = 0;
    lv_obj_clean(dc_root);
    if (!cJSON_IsTrue(cJSON_GetObjectItem(st, "connected"))) {
        lv_obj_t *ic = text(dc_root, F.icon_xl, P.on_sv, theme_icon("headset_mic"));
        lv_obj_align(ic, LV_ALIGN_CENTER, 0, -30);
        const char *err = jstr(st, "error");
        lv_obj_t *l = text(dc_root, F.body, P.on_sv, err && err[0] ? err : "Open Vesktop to see your voice channel");
        lv_obj_set_width(l, 220);
        lv_label_set_long_mode(l, LV_LABEL_LONG_WRAP);
        lv_obj_align(l, LV_ALIGN_CENTER, 0, 34);
        return;
    }
    bool mute = cJSON_IsTrue(cJSON_GetObjectItem(st, "mute")), deaf = cJSON_IsTrue(cJSON_GetObjectItem(st, "deaf"));
    const char *channel = jstr(st, "channel");
    cJSON *members = cJSON_GetObjectItem(st, "members");
    int n = LV_MIN(cJSON_GetArraySize(members), 12);
    // Avatars shrink as the channel fills up: big when it's you and a friend.
    int size = n <= 2 ? 88 : n <= 4 ? 80 : n <= 6 ? 72 : n <= 8 ? 64 : 54;
    dc_av_size = size;
    int radius = n <= 2 ? 112 : n <= 4 ? 120 : n <= 6 ? 126 : n <= 8 ? 130 : 136;
    for (int i = 0; i < n; i++) {
        cJSON *m = cJSON_GetArrayItem(members, i);
        bool speaking = cJSON_IsTrue(cJSON_GetObjectItem(m, "speaking")), muted = cJSON_IsTrue(cJSON_GetObjectItem(m, "muted"));
        int16_t ang = (int16_t)(270 + i * 360 / LV_MAX(n, 1)) % 360;
        lv_obj_t *av = box(dc_root);
        lv_obj_set_size(av, size, size);
        lv_obj_set_style_radius(av, size / 2, 0);
        lv_obj_set_style_bg_color(av, P.schh, 0);
        lv_obj_set_style_bg_opa(av, LV_OPA_COVER, 0);
        lv_obj_set_style_border_width(av, speaking ? LV_MAX(3, size / 16) : 0, 0);
        lv_obj_set_style_border_color(av, lv_color_hex(0x23A55A), 0); // Discord's speaking green
        lv_obj_remove_flag(av, LV_OBJ_FLAG_CLICKABLE);
        dc_av[dc_avn++] = av;
        int ax = radius * lv_trigo_cos(ang) / LV_TRIGO_SIN_MAX, ay = radius * lv_trigo_sin(ang) / LV_TRIGO_SIN_MAX;
        lv_obj_align(av, LV_ALIGN_CENTER, ax, ay);
        const lv_image_dsc_t *img = icon_get(jstr(m, "avatar"));
        lv_obj_t *ic = img ? app_icon(av, jstr(m, "avatar"), "person", P.on_surface, (size - 6) * 256 / (int)img->header.w)
                           : text(av, size > 70 ? F.icon_l : F.icon_m, P.on_surface, theme_icon("person"));
        lv_obj_center(ic);
        if (muted) { // red badge on the avatar's edge: headset when deafened, mic when muted
            bool deafened = cJSON_IsTrue(cJSON_GetObjectItem(m, "deaf"));
            int bs = size >= 72 ? 26 : 22;
            lv_obj_t *badge = box(dc_root); // on the root, so the round avatar doesn't clip it
            lv_obj_set_size(badge, bs, bs);
            lv_obj_set_style_radius(badge, bs / 2, 0);
            lv_obj_set_style_bg_color(badge, P.ec, 0);
            lv_obj_set_style_bg_opa(badge, LV_OPA_COVER, 0);
            lv_obj_set_style_border_width(badge, 2, 0);
            lv_obj_set_style_border_color(badge, P.surface, 0);
            lv_obj_remove_flag(badge, LV_OBJ_FLAG_CLICKABLE);
            lv_obj_align(badge, LV_ALIGN_CENTER, ax + size * 35 / 100, ay + size * 35 / 100); // edge, at 45°
            lv_obj_t *bi = text(badge, F.icon_xs, P.on_ec, theme_icon(deafened ? "headset_off" : "mic_off"));
            lv_obj_center(bi);
        }
    }
    // The members stay the focus: channel and server in the middle, and when
    // the client allows it, three small controls under them.
    bool controls = cJSON_IsTrue(cJSON_GetObjectItem(st, "controls"));
    bool in_voice = channel && channel[0];
    const char *guild = jstr(st, "guild");
    bool has_guild = in_voice && guild && guild[0];
    bool small = controls && in_voice;
    int y = small ? -30 : 0; // block shifts up to make room for the buttons
    lv_obj_t *ch = text(dc_root, F.title, P.on_surface, in_voice ? channel : "Not in voice");
    lv_obj_set_width(ch, 170);
    lv_label_set_long_mode(ch, LV_LABEL_LONG_DOT);
    lv_obj_set_style_text_align(ch, LV_TEXT_ALIGN_CENTER, 0);
    lv_obj_align(ch, LV_ALIGN_CENTER, 0, y + (has_guild ? -20 : -12));
    if (has_guild) { // server name, small, under the channel
        lv_obj_t *g = text(dc_root, F.label, P.on_sv, guild);
        lv_obj_set_width(g, 160);
        lv_label_set_long_mode(g, LV_LABEL_LONG_DOT);
        lv_obj_set_style_text_align(g, LV_TEXT_ALIGN_CENTER, 0);
        lv_obj_align(g, LV_ALIGN_CENTER, 0, y + 4);
    }
    if (in_voice) {
        lv_obj_t *cnt = text(dc_root, F.label, P.on_sv, "");
        lv_label_set_text_fmt(cnt, "%d in voice", cJSON_GetArraySize(members));
        lv_obj_align(cnt, LV_ALIGN_CENTER, 0, y + (has_guild ? 26 : 20));
    }
    if (small) {
        lv_obj_t *mb = dc_button(dc_root, 36, mute, mute ? "mic_off" : "mic", P.ec, P.on_ec, "mute");
        lv_obj_align(mb, LV_ALIGN_CENTER, -44, 30);
        lv_obj_t *db = dc_button(dc_root, 36, deaf, deaf ? "headset_off" : "headphones", P.ec, P.on_ec, "deaf");
        lv_obj_align(db, LV_ALIGN_CENTER, 0, 30);
        lv_obj_t *lb = dc_button(dc_root, 36, true, "call_end", P.error, P.on_primary, "leave");
        lv_obj_align(lb, LV_ALIGN_CENTER, 44, 30);
    }
}

static void dc_flush_pending(void)
{
    if (!dc_pending) return;
    cJSON *st = dc_pending;
    dc_pending = NULL;
    ui_discord(st);
    cJSON_Delete(st);
}

// ---- page dots ---------------------------------------------------------------------

static void dot_width_exec(void *o, int32_t v) { lv_obj_set_width(o, v); }

#define DOT_R 169 // the dots sit on this circle, centered at 6 o'clock

// Places the dots along the bottom edge, spaced by arc length for the target
// widths (the active one is a pill), each turned to follow the curve.
static void layout_dots(void)
{
    const float gap = 6, r = DOT_R;
    float total = gap * (page_count - 1);
    for (int i = 0; i < page_count; i++) total += i == cur_page ? 18 : 6;
    float s0 = -total / 2;
    for (int i = 0; i < page_count; i++) {
        float w = i == cur_page ? 18 : 6;
        float a = (float)M_PI / 2 - (s0 + w / 2) / r; // left to right along the bottom
        lv_obj_align(dots[i], LV_ALIGN_CENTER, (int32_t)lroundf(r * cosf(a)), (int32_t)lroundf(r * sinf(a)));
        lv_obj_set_style_transform_rotation(dots[i], (int32_t)lroundf((a - (float)M_PI / 2) * 1800 / (float)M_PI), 0);
        s0 += w + gap;
    }
}

static void update_dots(void)
{
    layout_dots();
    for (int i = 0; i < page_count; i++) {
        bool on = i == cur_page;
        lv_obj_set_style_bg_color(dots[i], on ? P.primary : P.outline_v, 0);
        int32_t target = on ? 18 : 6;
        if (lv_obj_get_width(dots[i]) == target) continue;
        lv_anim_t a;
        lv_anim_init(&a);
        lv_anim_set_var(&a, dots[i]);
        lv_anim_set_values(&a, lv_obj_get_width(dots[i]), target);
        lv_anim_set_duration(&a, 220);
        lv_anim_set_path_cb(&a, lv_anim_path_ease_out);
        lv_anim_set_exec_cb(&a, dot_width_exec);
        lv_anim_start(&a);
    }
}

static void dc_flush_pending(void);

// After any slide: never rest between two pages, and apply updates held back
// while the pages were moving.
static void tv_scroll_end(lv_event_t *e)
{
    static bool busy; // snapping below scrolls too, which fires this again
    if (busy || wrap_shot || lv_obj_is_scrolling(tv)) return;
    busy = true;
    lv_obj_t *t = lv_tileview_get_tile_active(tv);
    if (t && lv_obj_get_scroll_x(tv) != lv_obj_get_x(t)) lv_tileview_set_tile(tv, t, LV_ANIM_OFF); // instant, no loop
    if (lv_obj_get_x(tv) != 0) lv_obj_set_x(tv, 0);
    busy = false;
    dc_flush_pending();
}

// Once a second: never rest between two pages (an interrupted slide can leave
// the pages there), unless a finger or an animation is moving them.
static void tv_settle(void)
{
    if (!tv || wrap_shot || lv_obj_is_scrolling(tv)) return;
    if (indev && lv_indev_get_state(indev) == LV_INDEV_STATE_PRESSED) return;
    if (lv_anim_get(tv, NULL)) return;
    lv_obj_t *t = lv_tileview_get_tile_active(tv);
    if (t && lv_obj_get_scroll_x(tv) != lv_obj_get_x(t)) lv_tileview_set_tile(tv, t, LV_ANIM_OFF);
    if (lv_obj_get_x(tv) != 0) lv_obj_set_x(tv, 0);
}

static void tile_event(lv_event_t *e)
{
    lv_obj_t *t = lv_tileview_get_tile_active(tv);
    int idx = (int)(intptr_t)lv_obj_get_user_data(t);
    if (idx == cur_page) return;
    cur_page = idx;
    remember_page();
    update_dots();
    sync_gifs();
    ui_announce();
}

// ---- build a profile screen ----------------------------------------------------------

static cJSON *find_profile(const char *id)
{
    cJSON *list = cJSON_GetObjectItem(store_config(), "profiles");
    cJSON *p, *def = NULL;
    cJSON_ArrayForEach(p, list)
    {
        const char *pid = jstr(p, "id");
        if (!pid) continue;
        if (id && !strcmp(pid, id)) return p;
        if (!strcmp(pid, "default")) def = p;
    }
    return def ? def : cJSON_GetArrayItem(list, 0);
}

static void build(bool animate)
{
    theme_load(store_config());
    prof = find_profile(cur_profile);
    cJSON *ppal = cJSON_GetObjectItem(prof, "palette"); // profile color
    if (cJSON_IsObject(ppal)) theme_parse_palette(ppal, &P);
    for (int i = 0; i < MAX_PAGES; i++) clocks[i].used = false;
    memset(&tw, 0, sizeof(tw)); // also clears presets/reset
    win_w.ring = NULL;
    mix_w.col = NULL;
    mix_w.sliding = false;
    clip_col = NULL;
    dc_root = NULL;
    dc_avn = 0;
    dc_sig = 0;
    obs_root = obs_time = NULL;
    obs_sig = 0;
    memset(&gw, 0, sizeof(gw));
    memset(&mh, 0, sizeof(mh));
    strlcpy(cur_profile, jstr(prof, "id") ?: "default", sizeof(cur_profile));
    memset(&live, 0, sizeof(live));
    memset(&media_live, 0, sizeof(media_live));
    memset(slides, 0, sizeof(slides));
    ncustom = 0;
    if (wrap_shot) { // a wrap slide was running on the old screen: end it cleanly
        lv_anim_delete(wrap_shot, NULL);
        if (tv) {
            lv_anim_delete(tv, NULL);
            lv_obj_set_x(tv, 0);
        }
        wrap_done(NULL);
    }

    lv_obj_t *s = lv_obj_create(NULL);
    res = heap_caps_calloc(1, sizeof(*res), MALLOC_CAP_SPIRAM);
    if (res) lv_obj_add_event_cb(s, free_screen_res, LV_EVENT_DELETE, res);
    lv_obj_add_event_cb(s, screen_gesture, LV_EVENT_GESTURE, NULL);
    lv_obj_remove_flag(s, LV_OBJ_FLAG_SCROLLABLE);
    lv_obj_set_style_bg_color(s, P.surface, 0);
    lv_obj_set_style_bg_opa(s, LV_OPA_COVER, 0);

    tv = lv_tileview_create(s);
    lv_obj_set_size(tv, LCD_W, LCD_H);
    lv_obj_set_style_bg_opa(tv, LV_OPA_TRANSP, 0);
    lv_obj_set_scrollbar_mode(tv, LV_SCROLLBAR_MODE_OFF);

    cJSON *list = cJSON_GetObjectItem(prof, "pages");
    page_count = 0;
    cur_page = recalled_page();
    cJSON *pg;
    cJSON_ArrayForEach(pg, list)
    {
        if (page_count >= MAX_PAGES) break;
        pages[page_count++] = pg;
    }
    for (int i = 0; i < page_count; i++) {
        lv_dir_t dir = (i > 0 ? LV_DIR_LEFT : 0) | (i < page_count - 1 ? LV_DIR_RIGHT : 0);
        lv_obj_t *tile = lv_tileview_add_tile(tv, i, 0, dir);
        lv_obj_set_user_data(tile, (void *)(intptr_t)i);
        const char *k = page_kind(pages[i]);
        if (!strcmp(k, "media")) build_media(tile, pages[i]);
        else if (!strcmp(k, "stats")) build_stats(tile, pages[i]);
        else if (!strcmp(k, "dial")) build_dial(tile, pages[i]);
        else if (!strcmp(k, "image")) build_image(tile, pages[i], i);
        else if (!strcmp(k, "touchpad")) build_touchpad(tile, pages[i]);
        else if (!strcmp(k, "clock")) build_clock(tile, pages[i], i);
        else if (!strcmp(k, "timer")) build_timer(tile, pages[i]);
        else if (!strcmp(k, "windows")) build_windows(tile);
        else if (!strcmp(k, "mixer")) build_mixer(tile);
        else if (!strcmp(k, "clipboard")) build_clipboard(tile);
        else if (!strcmp(k, "discord")) build_discord(tile);
        else if (!strcmp(k, "obs")) build_obs(tile);
        else if (!strcmp(k, "game")) build_game(tile);
        else if (!strcmp(k, "mangohud")) build_mangohud(tile);
        else build_radial(tile, pages[i]);
    }

    // page dots along the bottom gap of the bezel
    // page dots along the bottom edge of the round screen
    dots_box = box(s);
    lv_obj_set_size(dots_box, LCD_W, LCD_H);
    lv_obj_center(dots_box);
    lv_obj_remove_flag(dots_box, LV_OBJ_FLAG_CLICKABLE);
    for (int i = 0; i < page_count; i++) {
        dots[i] = box(dots_box);
        lv_obj_set_size(dots[i], 6, 6);
        lv_obj_set_style_radius(dots[i], 3, 0);
        lv_obj_set_style_bg_opa(dots[i], LV_OPA_COVER, 0);
        lv_obj_set_style_transform_pivot_x(dots[i], lv_pct(50), 0);
        lv_obj_set_style_transform_pivot_y(dots[i], lv_pct(50), 0);
        lv_obj_remove_flag(dots[i], LV_OBJ_FLAG_CLICKABLE);
    }
    if (page_count < 2) lv_obj_add_flag(dots_box, LV_OBJ_FLAG_HIDDEN);

    bubble = pill(s, P.schh, P.on_surface, F.body);
    lv_obj_align(bubble, LV_ALIGN_CENTER, 0, 0);

    host_dot = box(s);
    lv_obj_set_size(host_dot, 8, 8);
    lv_obj_set_style_radius(host_dot, 4, 0);
    lv_obj_set_style_bg_color(host_dot, P.error, 0);
    lv_obj_set_style_bg_opa(host_dot, LV_OPA_COVER, 0);
    lv_obj_align(host_dot, LV_ALIGN_TOP_MID, 0, 10);
    lv_obj_remove_flag(host_dot, LV_OBJ_FLAG_CLICKABLE);
    if (usb_host_listening()) lv_obj_add_flag(host_dot, LV_OBJ_FLAG_HIDDEN);

    pause_badge = text(s, F.icon_s, P.tertiary, theme_icon("pause"));
    lv_obj_align(pause_badge, LV_ALIGN_TOP_MID, 0, 18);
    lv_obj_remove_flag(pause_badge, LV_OBJ_FLAG_CLICKABLE);
    if (!autoswitch_paused) lv_obj_add_flag(pause_badge, LV_OBJ_FLAG_HIDDEN);

    if (cur_page >= page_count) cur_page = 0;
    lv_tileview_set_tile_by_index(tv, cur_page, 0, LV_ANIM_OFF);
    lv_obj_add_event_cb(tv, tile_event, LV_EVENT_VALUE_CHANGED, NULL);
    lv_obj_add_event_cb(tv, tv_scroll_end, LV_EVENT_SCROLL_END, NULL);
    for (int i = 0; i < page_count; i++) lv_obj_set_width(dots[i], i == cur_page ? 18 : 6);
    update_dots();
    sync_gifs();
    if (qs) {
        lv_obj_delete(qs); // rebuilt with the new pages when opened
        qs = NULL;
        qs_auto_tile = NULL;
    }

    scr = s;
    lv_screen_load_anim(s, animate ? LV_SCR_LOAD_ANIM_FADE_IN : LV_SCR_LOAD_ANIM_NONE, animate ? 220 : 0, 0, true);
    ui_announce();
}

void ui_reload(void)
{
    static const lv_display_rotation_t ROT[] = {LV_DISPLAY_ROTATION_0, LV_DISPLAY_ROTATION_90, LV_DISPLAY_ROTATION_180,
                                                LV_DISPLAY_ROTATION_270};
    int rot = store_get_int("rotation", 0);
    lv_display_rotation_t want = ROT[(rot / 90) & 3];
    // esp_lvgl_port turns this into panel MADCTL swap/mirror; LVGL rotates touch points.
    if (lv_display_get_rotation(disp) != want) lv_display_set_rotation(disp, want);
    if (indev) lv_indev_set_long_press_time(indev, store_get_int("longPressMs", 450));
    if (power == AWAKE) board_backlight(store_brightness());
    build(false);
}

static bool pages_equivalent(cJSON *a, cJSON *b)
{
    if (!a || !b) return false;
    const char *ta = page_kind(a), *tb = page_kind(b);
    if (strcmp(ta, tb)) return false;
    if (!strcmp(ta, "media")) return cJSON_Compare(cJSON_GetObjectItem(a, "shapes"), cJSON_GetObjectItem(b, "shapes"), true) ||
                                      (!cJSON_GetObjectItem(a, "shapes") && !cJSON_GetObjectItem(b, "shapes"));
    // anything else: same content apart from the id
    cJSON *ca = cJSON_Duplicate(a, true), *cb = cJSON_Duplicate(b, true);
    cJSON_DeleteItemFromObject(ca, "id");
    cJSON_DeleteItemFromObject(cb, "id");
    bool same = cJSON_Compare(ca, cb, true);
    cJSON_Delete(ca);
    cJSON_Delete(cb);
    return same;
}

static void switch_profile(const char *id)
{
    if (!id) return;
    cJSON *p = find_profile(id);
    const char *pid = jstr(p, "id");
    if (pid && !strcmp(pid, cur_profile)) return;
    remember_page();
    cJSON *before = page_count ? pages[cur_page] : NULL;
    strlcpy(cur_profile, pid ? pid : "default", sizeof(cur_profile));
    // Landing on an equivalent page (media -> media, or an identical page) swaps
    // instantly: a fade would only flash the same content.
    int idx = recalled_page();
    cJSON *after = cJSON_GetArrayItem(cJSON_GetObjectItem(p, "pages"), idx);
    if (!after) after = cJSON_GetArrayItem(cJSON_GetObjectItem(p, "pages"), 0);
    build(!pages_equivalent(before, after));
}

void ui_set_profile(const char *id)
{
    if (autoswitch_paused) return; // paused from quick settings
    switch_profile(id);
}

void ui_assets_changed(void)
{
    remember_page();
    build(false);
}

static void np_close(void);
static void qs_close(void);
void ui_show(const char *profile, const char *page)
{
    np_close(); // previews from the configurator shouldn't open behind a panel
    qs_close();
    if (profile) switch_profile(profile);
    if (page) goto_page(page);
    if (power != AWAKE) lv_display_trigger_activity(disp);
}

// ---- power: dim & sleep ----------------------------------------------------------------

static void drop_shield(void)
{
    if (shield) {
        lv_obj_delete_async(shield);
        shield = NULL;
    }
}

static void wake(void)
{
    if (power == AWAKE) return;
    bool was_dark = power == ASLEEP || power == OFF;
    if (power == OFF) {
        board_panel_sleep(false);
        lv_obj_invalidate(lv_screen_active());
        drop_shield();
    }
    power = AWAKE;
    lv_display_trigger_activity(disp);
    board_backlight(store_brightness());
    if (was_dark) {
        sync_gifs();
        ui_announce();
    }
}

static uint32_t last_shield_tap;

static void shield_event(lv_event_t *e)
{
    lv_event_code_t c = lv_event_get_code(e);
    if (power == OFF) { // truly off: only a double tap wakes
        if (c == LV_EVENT_RELEASED) {
            uint32_t now = lv_tick_get();
            if (now - last_shield_tap < 450) wake();
            last_shield_tap = now;
        }
        return;
    }
    if (c == LV_EVENT_PRESSED) wake();
    else if (c == LV_EVENT_RELEASED || c == LV_EVENT_PRESS_LOST) drop_shield();
}

static void make_shield(void)
{
    if (shield) return;
    // Swallows the touches that wake the screen so they don't press a button.
    shield = box(lv_layer_top());
    lv_obj_set_size(shield, LCD_W, LCD_H);
    lv_obj_add_flag(shield, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_add_event_cb(shield, shield_event, LV_EVENT_ALL, NULL);
}

static void screen_off(void)
{
    if (power == OFF) return;
    power = OFF;
    board_backlight(0);
    board_panel_sleep(true); // display off + sleep in: no light at all
    make_shield();
    last_shield_tap = 0;
    sync_gifs();
    ui_announce();
}

static void power_tick(lv_timer_t *t)
{
    // Follow the PC: dark ~8 s after the host side goes away, back when it returns.
    static uint32_t host_gone_at;
    static bool host_was;
    bool host = usb_host_listening() && !usb_suspended();
    if (store_get_bool("followPc", true)) {
        if (!host && host_was) host_gone_at = lv_tick_get();
        if (!host && host_gone_at && lv_tick_get() - host_gone_at > 8000) {
            host_gone_at = 0;
            screen_off();
        }
        if (host && !host_was && power == OFF) wake();
    }
    if (host) host_gone_at = 0;
    host_was = host;
    if (power == OFF) return; // stays dark until a double tap or the PC comes back

    uint32_t idle = lv_display_get_inactive_time(disp);
    int dim_s = store_get_int("dimAfter", 30), sleep_s = store_get_int("sleepAfter", 300);

    if (host_dot) {
        bool hide = usb_host_listening();
        if (hide != lv_obj_has_flag(host_dot, LV_OBJ_FLAG_HIDDEN)) {
            if (hide) lv_obj_add_flag(host_dot, LV_OBJ_FLAG_HIDDEN);
            else lv_obj_remove_flag(host_dot, LV_OBJ_FLAG_HIDDEN);
        }
    }

    if (sleep_s > 0 && idle > (uint32_t)sleep_s * 1000) {
        if (power != ASLEEP) {
            power = ASLEEP;
                    board_backlight(0);
            make_shield();
                    sync_gifs();
            ui_announce();
        }
    } else if (dim_s > 0 && idle > (uint32_t)dim_s * 1000) {
        if (power == AWAKE) {
            power = DIM;
            board_backlight(store_brightness() / 4);
        }
    } else if (power == DIM) {
        wake();
    }
}

// ---- performance probe ({"t":"perf","ms":N}) ------------------------------------------------
// Times every refresh for N ms, then reports frames, average and worst draw time.

static struct {
    bool on;
    uint32_t until, frames, t0, sum, worst;
} perf;

static void perf_event(lv_event_t *e)
{
    if (!perf.on) return;
    uint32_t now = lv_tick_get();
    if (lv_event_get_code(e) == LV_EVENT_REFR_START) {
        perf.t0 = now;
        return;
    }
    uint32_t d = now - perf.t0;
    perf.frames++;
    perf.sum += d;
    if (d > perf.worst) perf.worst = d;
}

static void perf_done(lv_timer_t *t)
{
    if (!perf.on || lv_tick_get() < perf.until) return;
    perf.on = false;
    char b[120];
    uint32_t span = perf.until - (perf.until - (uint32_t)(uintptr_t)lv_timer_get_user_data(t));
    snprintf(b, sizeof(b), "perf page=%s frames=%u avg=%ums worst=%ums fps=%.1f", jstr(pages[cur_page], "id") ?: "?",
             (unsigned)perf.frames, (unsigned)(perf.frames ? perf.sum / perf.frames : 0), (unsigned)perf.worst,
             span ? perf.frames * 1000.0f / span : 0);
    proto_log(b);
    lv_timer_delete(t);
}

void ui_perf(int ms)
{
    memset(&perf, 0, sizeof(perf));
    perf.on = true;
    perf.until = lv_tick_get() + ms;
    static bool hooked;
    if (!hooked) {
        lv_display_add_event_cb(disp, perf_event, LV_EVENT_REFR_START, NULL);
        lv_display_add_event_cb(disp, perf_event, LV_EVENT_REFR_READY, NULL);
        hooked = true;
    }
    lv_timer_create(perf_done, 100, (void *)(uintptr_t)ms);
}

// ---- pull-down detector -----------------------------------------------------------------
// Wraps the touch driver's read callback: a press that starts in the top band
// and moves down opens quick settings, whatever widget is under the finger.

static lv_indev_read_cb_t touch_read_orig;
static struct {
    bool down, armed, armed_up;
    lv_point_t p0, last;
    uint32_t t0;
} pull;
static void np_open_async(void *arg) { np_open(); }
static void np_close_async(void *arg) { np_close(); }
static bool pull_close_np;
static struct {
    int side;      // -1 left, 1 right, 0 none
    uint32_t when; // release time of the previous edge tap
} edge;

static void pull_open_async(void *arg) { qs_open(); }

static void edge_action_async(void *arg)
{
    int side = (int)(intptr_t)arg;
    const char *a = jstr(cJSON_GetObjectItem(store_config(), "device"), "edgeDoubleTap");
    if (!a) a = "page";
    arc_mode = ARC_IDLE;
    dragging = false;
    if (np && !lv_obj_has_flag(np, LV_OBJ_FLAG_HIDDEN)) { np_close(); return; }
    if (qs && !lv_obj_has_flag(qs, LV_OBJ_FLAG_HIDDEN)) { qs_close(); return; }
    if (!strcmp(a, "page")) goto_index(cur_page + side, false); // instant, no slide
    else if (!strcmp(a, "quickSettings")) qs_open();
    else if (!strcmp(a, "screenOff")) screen_off();
}

// A short, still tap in the left/right edge band; returns -1/1, or 0.
static int edge_tap(lv_point_t p0, lv_point_t p1, uint32_t dur)
{
    int32_t dx = p1.x - p0.x, dy = p1.y - p0.y;
    if (dur > 300 || dx * dx + dy * dy > 15 * 15) return 0;
    int32_t cx = p0.x - LCD_W / 2, cy = p0.y - LCD_H / 2;
    if (cx * cx + cy * cy < 135 * 135 || LV_ABS(cy) > LV_ABS(cx)) return 0; // outer ring, left/right quarters
    return cx < 0 ? -1 : 1;
}

static void touch_read_wrap(lv_indev_t *in, lv_indev_data_t *d)
{
    touch_read_orig(in, d);
    if (d->state != LV_INDEV_STATE_PRESSED) {
        if (pull.down && power != OFF && power != ASLEEP) {
            int side = edge_tap(pull.p0, pull.last, lv_tick_get() - pull.t0);
            const char *a = jstr(cJSON_GetObjectItem(store_config(), "device"), "edgeDoubleTap");
            if (side && !(a && !strcmp(a, "off"))) {
                if (edge.side == side && lv_tick_get() - edge.when < 400) {
                    edge.side = 0;
                    lv_async_call(edge_action_async, (void *)(intptr_t)side);
                } else {
                    edge.side = side;
                    edge.when = lv_tick_get();
                }
            } else {
                edge.side = 0;
            }
        }
        pull.down = false;
        return;
    }
    lv_point_t p = d->point; // same rotation mapping LVGL applies after this callback
    lv_display_rotation_t r = lv_display_get_rotation(disp);
    if (r == LV_DISPLAY_ROTATION_180 || r == LV_DISPLAY_ROTATION_270) {
        p.x = LCD_W - p.x - 1;
        p.y = LCD_H - p.y - 1;
    }
    if (r == LV_DISPLAY_ROTATION_90 || r == LV_DISPLAY_ROTATION_270) {
        int32_t t = p.y;
        p.y = p.x;
        p.x = LCD_H - t - 1;
    }
    pull.last = p;
    if (!pull.down) {
        pull.down = true;
        pull.p0 = p;
        pull.t0 = lv_tick_get();
        bool panels_closed = (!qs || lv_obj_has_flag(qs, LV_OBJ_FLAG_HIDDEN)) && (!np || lv_obj_has_flag(np, LV_OBJ_FLAG_HIDDEN));
        bool awake = power == AWAKE || power == DIM;
        pull.armed = p.y < 64 && awake && panels_closed;
        pull.armed_up = p.y > LCD_H - 64 && awake && panels_closed && store_get_bool("notifications", true);
        pull_close_np = p.y < 64 && np && !lv_obj_has_flag(np, LV_OBJ_FLAG_HIDDEN);
        return;
    }
    int32_t dy = p.y - pull.p0.y, dx = LV_ABS(p.x - pull.p0.x);
    if (pull.armed && dy > 50 && dx < dy) {
        pull.armed = false;
        lv_async_call(pull_open_async, NULL);
    }
    if (pull_close_np && dy > 50 && dx < dy) {
        pull_close_np = false;
        lv_async_call(np_close_async, NULL);
    }
    if (pull.armed_up && -dy > 50 && dx < -dy) {
        pull.armed_up = false;
        lv_async_call(np_open_async, NULL);
    }
}

// ---- init -------------------------------------------------------------------------------

void ui_init(lv_display_t *d, lv_indev_t *touch)
{
    disp = d;
    indev = touch;
    lv_style_transition_dsc_init(&tr_press, press_props, lv_anim_path_ease_out, 110, 0, NULL);
    lv_style_transition_dsc_init(&tr_release, press_props, lv_anim_path_overshoot, 320, 0, NULL);

    toast = pill(lv_layer_top(), P.schh, P.on_surface, F.body);
    lv_obj_align(toast, LV_ALIGN_TOP_MID, 0, 30);

    bubble_timer = lv_timer_create(bubble_hide, LABEL_SHOW_MS, NULL);
    lv_timer_pause(bubble_timer);
    lv_timer_create(level_flush, 50, NULL);
    lv_timer_create(power_tick, 500, NULL);
    lv_timer_create(slideshow_tick, 250, NULL);
    lv_timer_create(clock_tick, 1000, NULL);
    lv_timer_create(timer_tick, 250, NULL);
    autoswitch_paused = store_local_get("autosw_paused", 0);
    if (indev) {
        touch_read_orig = lv_indev_get_read_cb(indev);
        lv_indev_set_read_cb(indev, touch_read_wrap);
    }
    ui_reload();
    // style the toast now that the palette is loaded
    lv_obj_set_style_bg_color(toast, P.schh, 0);
    lv_obj_set_style_text_color(lv_obj_get_child(toast, 0), P.on_surface, 0);
}
