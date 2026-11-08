#pragma once
#include "cJSON.h"
#include "lvgl.h"

typedef struct {
    lv_color_t surface, sc, sch, schh, on_surface, on_sv;
    lv_color_t primary, on_primary, pc, on_pc;
    lv_color_t secc, on_secc;
    lv_color_t tertiary, tc, on_tc;
    lv_color_t error, ec, on_ec;
    lv_color_t outline, outline_v;
} palette_t;

typedef struct {
    const lv_font_t *body;   // 16 medium
    const lv_font_t *label;  // 14 medium
    const lv_font_t *title;  // 22 bold
    const lv_font_t *value;  // 34 bold
    const lv_font_t *big;    // 56 bold
    const lv_font_t *icon_s; // 24
    const lv_font_t *icon_m; // 32
    const lv_font_t *icon_l; // 44
    const lv_font_t *icon_xl;// 64
} fonts_t;

extern palette_t P;
extern fonts_t F;

void theme_fonts_init(void);
void theme_load(cJSON *cfg);
// Fill `out` from a {role: "#RRGGBB"} object, falling back to the current P.
void theme_parse_palette(cJSON *pal, palette_t *out);
// container / on-container pair for a button color role
void theme_role(const char *role, lv_color_t *bg, lv_color_t *fg);
// UTF-8 for an icon name ("" if unknown)
const char *theme_icon(const char *name);
