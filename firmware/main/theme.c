#include "theme.h"

#include <stdlib.h>
#include <string.h>

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
}

static lv_color_t col(cJSON *pal, const char *k, uint32_t def)
{
    const char *s = cJSON_GetStringValue(cJSON_GetObjectItem(pal, k));
    if (s && s[0] == '#' && strlen(s) == 7) return lv_color_hex(strtoul(s + 1, NULL, 16));
    return lv_color_hex(def);
}

void theme_load(cJSON *cfg)
{
    cJSON *pal = cJSON_GetObjectItem(cJSON_GetObjectItem(cfg, "theme"), "palette");
    P.surface = col(pal, "surface", 0x141218);
    P.sc = col(pal, "surfaceContainer", 0x211F26);
    P.sch = col(pal, "surfaceContainerHigh", 0x2B2930);
    P.schh = col(pal, "surfaceContainerHighest", 0x36343B);
    P.on_surface = col(pal, "onSurface", 0xE6E0E9);
    P.on_sv = col(pal, "onSurfaceVariant", 0xCAC4D0);
    P.primary = col(pal, "primary", 0xD0BCFF);
    P.on_primary = col(pal, "onPrimary", 0x381E72);
    P.pc = col(pal, "primaryContainer", 0x4F378B);
    P.on_pc = col(pal, "onPrimaryContainer", 0xEADDFF);
    P.secc = col(pal, "secondaryContainer", 0x4A4458);
    P.on_secc = col(pal, "onSecondaryContainer", 0xE8DEF8);
    P.tertiary = col(pal, "tertiary", 0xEFB8C8);
    P.tc = col(pal, "tertiaryContainer", 0x633B48);
    P.on_tc = col(pal, "onTertiaryContainer", 0xFFD8E4);
    P.error = col(pal, "error", 0xF2B8B5);
    P.ec = col(pal, "errorContainer", 0x8C1D18);
    P.on_ec = col(pal, "onErrorContainer", 0xF9DEDC);
    P.outline = col(pal, "outline", 0x938F99);
    P.outline_v = col(pal, "outlineVariant", 0x49454F);
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
