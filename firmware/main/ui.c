// Material 3 Expressive UI for the 360x360 round screen.
//
// One LVGL screen per profile. Its pages live in a horizontal tileview (swipe
// to change page). Buttons morph circle -> rounded square while pressed and
// spring back with an overshoot; value pages use a 270° arc along the bezel.
#include "ui.h"

#include <math.h>
#include <stdio.h>
#include <string.h>

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
        lv_obj_t *title, *artist, *player, *play, *play_icon, *cover, *scrim;
    } media[MAX_LIVE];
    int nmedia;
    struct {
        lv_obj_t *arc, *val;
        char key[10];
    } stat[MAX_PAGES * 4];
    int nstat;
} live;

static int vol = 50, mic = 50;
static bool vol_muted, mic_muted;
static bool dragging;
static char m_title[128], m_artist[128], m_player[48];
static palette_t PM;        // media-page palette from the album art
static bool pm_active;
static lv_image_dsc_t cover_dsc;
static bool cover_set;
static bool m_playing;

typedef enum { AWAKE, DIM, ASLEEP } power_t;
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

static lv_obj_t *shape_button(lv_obj_t *parent, int size, int radius, lv_color_t bg)
{
    lv_obj_t *o = box(parent);
    lv_obj_add_flag(o, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_set_size(o, size, size);
    lv_obj_set_style_bg_color(o, bg, 0);
    lv_obj_set_style_bg_opa(o, LV_OPA_COVER, 0);
    lv_obj_set_style_radius(o, radius, 0);
    lv_obj_set_style_bg_color(o, lv_color_mix(lv_color_white(), bg, 30), LV_STATE_PRESSED);
    squish_on_press(o, size);
    return o;
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

static lv_obj_t *config_button(lv_obj_t *parent, cJSON *b, int size, bool center, const lv_font_t *ifont)
{
    lv_color_t bg, fg;
    theme_role(jstr(b, "color"), &bg, &fg);
    int shape = shape_id(jstr(b, "shape"), center ? SH_SQUARE : SH_CIRCLE);
    lv_obj_t *o = lv_image_create(parent);
    lv_image_set_src(o, shape_mask(shape, size));
    lv_obj_set_size(o, size, size);
    lv_obj_add_flag(o, LV_OBJ_FLAG_CLICKABLE);
    lv_obj_remove_flag(o, LV_OBJ_FLAG_SCROLLABLE);
    lv_obj_set_style_image_recolor(o, bg, 0);
    lv_obj_set_style_image_recolor_opa(o, LV_OPA_COVER, 0);
    lv_obj_set_style_image_recolor(o, lv_color_mix(lv_color_white(), bg, 30), LV_STATE_PRESSED);
    squish_on_press(o, size);
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

typedef enum { ARC_IDLE, ARC_UNDECIDED, ARC_DRAG, ARC_SWIPE } arc_mode_t;
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
            arc_mode = across && page_count > 1 ? ARC_SWIPE : ARC_DRAG;
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
            int target = cur_page;
            if (dx < -40 && cur_page < page_count - 1) target++;
            else if (dx > 40 && cur_page > 0) target--;
            lv_tileview_set_tile_by_index(tv, target, 0, LV_ANIM_ON);
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

static void play_morph(lv_obj_t *btn, bool playing)
{
    // Playing: rounded square. Paused: circle. Same as M3 Expressive media controls.
    int32_t size = lv_obj_get_style_width(btn, 0);
    int32_t r = playing ? size * 30 / 100 : size / 2;
    lv_obj_set_style_radius(btn, r, 0);
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

static lv_obj_t *media_btn(lv_obj_t *tile, int size, lv_color_t bg, lv_color_t fg, const char *icon, const lv_font_t *f, const char *cmd)
{
    lv_obj_t *b = shape_button(tile, size, size / 2, bg);
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
    lv_obj_t *scrim = box(tile); // keeps text readable over any artwork
    lv_obj_set_size(scrim, LCD_W, LCD_H);
    lv_obj_center(scrim);
    lv_obj_set_style_bg_color(scrim, P.surface, 0);
    lv_obj_set_style_bg_opa(scrim, LV_OPA_50, 0);
    lv_obj_remove_flag(scrim, LV_OBJ_FLAG_CLICKABLE);
    if (cover_set) lv_image_set_src(cover, &cover_dsc);
    else {
        lv_obj_add_flag(cover, LV_OBJ_FLAG_HIDDEN);
        lv_obj_add_flag(scrim, LV_OBJ_FLAG_HIDDEN);
    }

    lv_obj_t *arc = bezel_arc(tile, P.primary, true);
    lv_obj_t *vl = text(tile, F.label, P.on_sv, "");
    lv_obj_align(vl, LV_ALIGN_CENTER, 0, 132);
    add_level(false, arc, vl);

    if (live.nmedia >= MAX_LIVE) return;
    typeof(live.media[0]) *m = &live.media[live.nmedia++];
    m->cover = cover;
    m->scrim = scrim;
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

    lv_obj_t *prev = media_btn(tile, 64, P.secc, P.on_secc, "skip_previous", F.icon_m, "prev");
    lv_obj_align(prev, LV_ALIGN_CENTER, -94, 44);
    lv_obj_t *next = media_btn(tile, 64, P.secc, P.on_secc, "skip_next", F.icon_m, "next");
    lv_obj_align(next, LV_ALIGN_CENTER, 94, 44);
    m->play = media_btn(tile, 100, P.pc, P.on_pc, m_playing ? "pause" : "play_arrow", F.icon_l, "play_pause");
    lv_obj_align(m->play, LV_ALIGN_CENTER, 0, 44);
    m->play_icon = lv_obj_get_child(m->play, 0);
    play_morph(m->play, m_playing);
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
        play_morph(live.media[i].play, playing);
    }
}

void ui_cover(const uint8_t *rgb565, int w, int h)
{
    cover_set = rgb565 != NULL;
    if (cover_set) {
        cover_dsc.header.magic = LV_IMAGE_HEADER_MAGIC;
        cover_dsc.header.cf = LV_COLOR_FORMAT_RGB565;
        cover_dsc.header.w = w;
        cover_dsc.header.h = h;
        cover_dsc.header.stride = w * 2;
        cover_dsc.data_size = (uint32_t)w * h * 2;
        cover_dsc.data = rgb565;
        lv_image_cache_drop(&cover_dsc); // same descriptor, new pixels
    }
    for (int i = 0; i < live.nmedia; i++) {
        if (cover_set) {
            lv_image_set_src(live.media[i].cover, &cover_dsc);
            lv_obj_remove_flag(live.media[i].cover, LV_OBJ_FLAG_HIDDEN);
            lv_obj_remove_flag(live.media[i].scrim, LV_OBJ_FLAG_HIDDEN);
            lv_obj_invalidate(live.media[i].cover);
        } else {
            lv_obj_add_flag(live.media[i].cover, LV_OBJ_FLAG_HIDDEN);
            lv_obj_add_flag(live.media[i].scrim, LV_OBJ_FLAG_HIDDEN);
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

// ---- page dots ---------------------------------------------------------------------

static void dot_width_exec(void *o, int32_t v) { lv_obj_set_width(o, v); }

static void update_dots(void)
{
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

static void tile_event(lv_event_t *e)
{
    lv_obj_t *t = lv_tileview_get_tile_active(tv);
    int idx = (int)(intptr_t)lv_obj_get_user_data(t);
    if (idx == cur_page) return;
    cur_page = idx;
    remember_page();
    update_dots();
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
    strlcpy(cur_profile, jstr(prof, "id") ?: "default", sizeof(cur_profile));
    memset(&live, 0, sizeof(live));
    ncustom = 0;

    lv_obj_t *s = lv_obj_create(NULL);
    lv_obj_remove_flag(s, LV_OBJ_FLAG_SCROLLABLE);
    lv_obj_set_style_bg_color(s, P.surface, 0);
    lv_obj_set_style_bg_opa(s, LV_OPA_COVER, 0);

    tv = lv_tileview_create(s);
    lv_obj_set_size(tv, LCD_W, LCD_H);
    lv_obj_set_style_bg_opa(tv, LV_OPA_TRANSP, 0);
    lv_obj_set_scrollbar_mode(tv, LV_SCROLLBAR_MODE_OFF);

    cJSON *list = cJSON_GetObjectItem(prof, "pages");
    page_count = 0;
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
        else build_radial(tile, pages[i]);
    }

    // page dots along the bottom gap of the bezel
    dots_box = box(s);
    lv_obj_set_size(dots_box, LV_SIZE_CONTENT, 6);
    lv_obj_set_flex_flow(dots_box, LV_FLEX_FLOW_ROW);
    lv_obj_set_style_pad_column(dots_box, 6, 0);
    lv_obj_align(dots_box, LV_ALIGN_BOTTOM_MID, 0, -8);
    lv_obj_remove_flag(dots_box, LV_OBJ_FLAG_CLICKABLE);
    for (int i = 0; i < page_count; i++) {
        dots[i] = box(dots_box);
        lv_obj_set_size(dots[i], 6, 6);
        lv_obj_set_style_radius(dots[i], 3, 0);
        lv_obj_set_style_bg_opa(dots[i], LV_OPA_COVER, 0);
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

    cur_page = recalled_page();
    if (cur_page >= page_count) cur_page = 0;
    lv_tileview_set_tile_by_index(tv, cur_page, 0, LV_ANIM_OFF);
    lv_obj_add_event_cb(tv, tile_event, LV_EVENT_VALUE_CHANGED, NULL);
    for (int i = 0; i < page_count; i++) lv_obj_set_width(dots[i], i == cur_page ? 18 : 6);
    update_dots();

    scr = s;
    lv_screen_load_anim(s, animate ? LV_SCR_LOAD_ANIM_FADE_IN : LV_SCR_LOAD_ANIM_NONE, animate ? 220 : 0, 0, true);
    ui_announce();
}

void ui_reload(void)
{
    if (indev) lv_indev_set_long_press_time(indev, store_get_int("longPressMs", 450));
    if (power == AWAKE) board_backlight(store_brightness());
    build(false);
}

void ui_set_profile(const char *id)
{
    if (!id) return;
    cJSON *p = find_profile(id);
    const char *pid = jstr(p, "id");
    if (pid && !strcmp(pid, cur_profile)) return;
    remember_page();
    strlcpy(cur_profile, pid ? pid : "default", sizeof(cur_profile));
    build(true);
}

void ui_show(const char *profile, const char *page)
{
    if (profile) ui_set_profile(profile);
    if (page) goto_page(page);
    if (power != AWAKE) lv_display_trigger_activity(disp);
}

// ---- power: dim & sleep ----------------------------------------------------------------

static void wake(void)
{
    if (power == AWAKE) return;
    bool was_asleep = power == ASLEEP;
    power = AWAKE;
    board_backlight(store_brightness());
    if (was_asleep) ui_announce();
}

static void shield_event(lv_event_t *e)
{
    if (lv_event_get_code(e) == LV_EVENT_PRESSED) wake();
    else if (lv_event_get_code(e) == LV_EVENT_RELEASED || lv_event_get_code(e) == LV_EVENT_PRESS_LOST) {
        lv_obj_delete_async(shield);
        shield = NULL;
    }
}

static void power_tick(lv_timer_t *t)
{
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
            if (!shield) {
                // Swallow the touch that wakes the screen so it doesn't press a button.
                shield = box(lv_layer_top());
                lv_obj_set_size(shield, LCD_W, LCD_H);
                lv_obj_add_flag(shield, LV_OBJ_FLAG_CLICKABLE);
                lv_obj_add_event_cb(shield, shield_event, LV_EVENT_ALL, NULL);
            }
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
    ui_reload();
    // style the toast now that the palette is loaded
    lv_obj_set_style_bg_color(toast, P.schh, 0);
    lv_obj_set_style_text_color(lv_obj_get_child(toast, 0), P.on_surface, 0);
}
