#include "theme.h"

#include <stdlib.h>
#include <string.h>

#include "fontpack.h"
#include "icons.h"

palette_t P;
fonts_t F;

extern const uint8_t gsf500_start[] asm("_binary_gsf_500_ttf_start");
extern const uint8_t gsf500_end[] asm("_binary_gsf_500_ttf_end");
extern const uint8_t gsf700_start[] asm("_binary_gsf_700_ttf_start");
extern const uint8_t gsf700_end[] asm("_binary_gsf_700_ttf_end");
extern const uint8_t sym_start[] asm("_binary_symbols_ttf_start");
extern const uint8_t sym_end[] asm("_binary_symbols_ttf_end");

static lv_font_t *ttf(const uint8_t *s, const uint8_t *e, int px)
{
    return lv_tiny_ttf_create_data(s, e - s, px);
}

const lv_font_t *theme_font_clock(void)
{
    static lv_font_t *f;
    if (!f) {
        f = ttf(gsf700_start, gsf700_end, 72);
    }
    return f;
}

void theme_fonts_init(void)
{
    F.label = ttf(gsf500_start, gsf500_end, 14);
    F.body = ttf(gsf500_start, gsf500_end, 16);
    F.title = ttf(gsf700_start, gsf700_end, 22);
    F.value = ttf(gsf700_start, gsf700_end, 34);
    F.big = ttf(gsf700_start, gsf700_end, 56);
    F.icon_s = ttf(sym_start, sym_end, 24);
    F.icon_m = ttf(sym_start, sym_end, 32);
    F.icon_l = ttf(sym_start, sym_end, 44);
    F.icon_xl = ttf(sym_start, sym_end, 64);
    // Japanese / Chinese / Korean text and color emoji come from the glyph pack.
    fontpack_attach((lv_font_t *)F.title, (lv_font_t *)F.body, (lv_font_t *)F.label);
}

static lv_color_t col(cJSON *pal, const char *k, lv_color_t def)
{
    const char *s = cJSON_GetStringValue(cJSON_GetObjectItem(pal, k));
    if (s && s[0] == '#' && strlen(s) == 7) return lv_color_hex(strtoul(s + 1, NULL, 16));
    return def;
}

void theme_parse_palette(cJSON *pal, palette_t *out)
{
    palette_t d = *out;
    out->surface = col(pal, "surface", d.surface);
    out->sc = col(pal, "surfaceContainer", d.sc);
    out->sch = col(pal, "surfaceContainerHigh", d.sch);
    out->schh = col(pal, "surfaceContainerHighest", d.schh);
    out->on_surface = col(pal, "onSurface", d.on_surface);
    out->on_sv = col(pal, "onSurfaceVariant", d.on_sv);
    out->primary = col(pal, "primary", d.primary);
    out->on_primary = col(pal, "onPrimary", d.on_primary);
    out->pc = col(pal, "primaryContainer", d.pc);
    out->on_pc = col(pal, "onPrimaryContainer", d.on_pc);
    out->secc = col(pal, "secondaryContainer", d.secc);
    out->on_secc = col(pal, "onSecondaryContainer", d.on_secc);
    out->tertiary = col(pal, "tertiary", d.tertiary);
    out->tc = col(pal, "tertiaryContainer", d.tc);
    out->on_tc = col(pal, "onTertiaryContainer", d.on_tc);
    out->error = col(pal, "error", d.error);
    out->ec = col(pal, "errorContainer", d.ec);
    out->on_ec = col(pal, "onErrorContainer", d.on_ec);
    out->outline = col(pal, "outline", d.outline);
    out->outline_v = col(pal, "outlineVariant", d.outline_v);
}

void theme_load(cJSON *cfg)
{
    theme_parse_palette(cJSON_GetObjectItem(cJSON_GetObjectItem(cfg, "theme"), "palette"), &P);
}

void theme_role(const char *role, lv_color_t *bg, lv_color_t *fg)
{
    if (!role) role = "surface";
    if (!strcmp(role, "primary")) { *bg = P.pc; *fg = P.on_pc; }
    else if (!strcmp(role, "secondary")) { *bg = P.secc; *fg = P.on_secc; }
    else if (!strcmp(role, "tertiary")) { *bg = P.tc; *fg = P.on_tc; }
    else if (!strcmp(role, "error")) { *bg = P.ec; *fg = P.on_ec; }
    else { *bg = P.schh; *fg = P.on_surface; }
}

static int cmp(const void *k, const void *e) { return strcmp((const char *)k, ((const icon_t *)e)->name); }

const char *theme_icon(const char *name)
{
    static char buf[8][4]; // a few at a time are alive while building a page
    static int slot;
    if (!name) return "";
    const icon_t *ic = bsearch(name, ICONS, ICON_COUNT, sizeof(icon_t), cmp);
    if (!ic) return "";
    char *b = buf[slot++ & 7];
    uint16_t c = ic->cp; // BMP private use: always 3 bytes in UTF-8
    b[0] = 0xE0 | (c >> 12);
    b[1] = 0x80 | ((c >> 6) & 0x3F);
    b[2] = 0x80 | (c & 0x3F);
    b[3] = 0;
    return b;
}
