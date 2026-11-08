// Glyph pack reader. Layout (little endian), see tools/fontpack:
//   "RBF1", u32 nfonts, u32 font_offset[nfonts]
//   font: u16 px, u16 line_height, u16 base_line, u16 flags (bit0: color),
//         u32 count, u32 cmap_off, u32 dsc_off, u32 bmp_off   (from font start)
//   cmap: u32 codepoint[count], sorted
//   dsc:  {u32 bmp_off, u16 adv, u8 w, u8 h, i8 ofs_x, i8 ofs_y, u16 pad}[count]
//   bitmaps: A4 rows ((w+1)/2 bytes) or RGB565 plane + A8 plane for color glyphs
#include "fontpack.h"

#include <string.h>

#include "esp_heap_caps.h"
#include "esp_log.h"
#include "esp_partition.h"

static const char *TAG = "fontpack";

typedef struct __attribute__((packed)) {
    uint32_t bmp_off;
    uint16_t adv;
    uint8_t w, h;
    int8_t ofs_x, ofs_y;
    uint16_t pad;
} pdsc_t;

typedef struct {
    const uint8_t *base;
    const uint32_t *cmap;
    const pdsc_t *dsc;
    const uint8_t *bmp;
    uint32_t count;
    bool color;
    lv_image_dsc_t *imgs; // color fonts: one image descriptor per glyph, filled lazily
} pfont_t;

static int find(const pfont_t *pf, uint32_t cp)
{
    uint32_t lo = 0, hi = pf->count;
    while (lo < hi) {
        uint32_t mid = (lo + hi) / 2;
        if (pf->cmap[mid] < cp) lo = mid + 1;
        else hi = mid;
    }
    return lo < pf->count && pf->cmap[lo] == cp ? (int)lo : -1;
}

static bool get_dsc(const lv_font_t *font, lv_font_glyph_dsc_t *g, uint32_t letter, uint32_t next)
{
    pfont_t *pf = (pfont_t *)font->dsc;
    int i = find(pf, letter);
    if (i < 0) return false;
    const pdsc_t *d = &pf->dsc[i];
    g->adv_w = d->adv;
    g->box_w = d->w;
    g->box_h = d->h;
    g->ofs_x = d->ofs_x;
    g->ofs_y = d->ofs_y;
    g->stride = 0;
    g->is_placeholder = 0;
    g->resolved_font = font;
    if (pf->color) {
        lv_image_dsc_t *img = &pf->imgs[i];
        if (!img->data) {
            img->header.magic = LV_IMAGE_HEADER_MAGIC;
            img->header.cf = LV_COLOR_FORMAT_RGB565A8;
            img->header.w = d->w;
            img->header.h = d->h;
            img->header.stride = d->w * 2;
            img->data_size = (uint32_t)d->w * d->h * 3;
            img->data = pf->bmp + d->bmp_off;
        }
        g->format = LV_FONT_GLYPH_FORMAT_IMAGE;
        g->gid.src = img;
    } else {
        g->format = LV_FONT_GLYPH_FORMAT_A4;
        g->gid.index = (uint32_t)i;
    }
    return true;
}

static const uint8_t opa4[16] = {0, 17, 34, 51, 68, 85, 102, 119, 136, 153, 170, 187, 204, 221, 238, 255};

static const void *get_bitmap(lv_font_glyph_dsc_t *g, lv_draw_buf_t *buf)
{
    pfont_t *pf = (pfont_t *)g->resolved_font->dsc;
    if (pf->color) return g->gid.src;
    const pdsc_t *d = &pf->dsc[g->gid.index];
    const uint8_t *in = pf->bmp + d->bmp_off;
    if (g->req_raw_bitmap) return in;
    uint32_t stride_out = lv_draw_buf_width_to_stride(d->w, LV_COLOR_FORMAT_A8);
    uint32_t stride_in = (d->w + 1) / 2;
    for (int y = 0; y < d->h; y++) {
        uint8_t *out = buf->data + y * stride_out;
        const uint8_t *row = in + y * stride_in;
        for (int x = 0; x < d->w; x++) out[x] = opa4[(x & 1) ? row[x / 2] & 0xF : row[x / 2] >> 4];
    }
    lv_draw_buf_flush_cache(buf, NULL);
    return buf;
}

static lv_font_t *load(const uint8_t *blob, uint32_t off)
{
    const uint8_t *f = blob + off;
    pfont_t *pf = heap_caps_calloc(1, sizeof(*pf), MALLOC_CAP_SPIRAM);
    lv_font_t *font = heap_caps_calloc(1, sizeof(*font), MALLOC_CAP_SPIRAM);
    if (!pf || !font) return NULL;
    const uint16_t *h16 = (const uint16_t *)f;
    const uint32_t *h32 = (const uint32_t *)(f + 8);
    pf->base = f;
    pf->color = h16[3] & 1;
    pf->count = h32[0];
    pf->cmap = (const uint32_t *)(f + h32[1]);
    pf->dsc = (const pdsc_t *)(f + h32[2]);
    pf->bmp = f + h32[3];
    if (pf->color) pf->imgs = heap_caps_calloc(pf->count, sizeof(lv_image_dsc_t), MALLOC_CAP_SPIRAM);
    font->get_glyph_dsc = get_dsc;
    font->get_glyph_bitmap = get_bitmap;
    font->line_height = h16[1];
    font->base_line = h16[2];
    font->dsc = pf;
    ESP_LOGI(TAG, "font %upx %s: %u glyphs", h16[0], pf->color ? "color" : "A4", (unsigned)pf->count);
    return font;
}

void fontpack_attach(lv_font_t *title22, lv_font_t *body16, lv_font_t *label14)
{
    const esp_partition_t *part = esp_partition_find_first(ESP_PARTITION_TYPE_DATA, 0x40, "fonts");
    if (!part) {
        ESP_LOGW(TAG, "no fonts partition");
        return;
    }
    const void *map;
    esp_partition_mmap_handle_t h;
    if (esp_partition_mmap(part, 0, part->size, ESP_PARTITION_MMAP_DATA, &map, &h) != ESP_OK) {
        ESP_LOGW(TAG, "mmap failed");
        return;
    }
    const uint8_t *blob = map;
    if (memcmp(blob, "RBF1", 4)) {
        ESP_LOGW(TAG, "fonts partition is empty (run tools/flash.sh)");
        esp_partition_munmap(h);
        return;
    }
    uint32_t n = *(const uint32_t *)(blob + 4);
    const uint32_t *offs = (const uint32_t *)(blob + 8);
    if (n < 5) return;
    // Order written by tools/fontpack: text 22, 16, 14, then emoji 22, 16.
    lv_font_t *t22 = load(blob, offs[0]), *t16 = load(blob, offs[1]), *t14 = load(blob, offs[2]);
    lv_font_t *e22 = load(blob, offs[3]), *e16 = load(blob, offs[4]);
    if (t22) t22->fallback = e22;
    if (t16) t16->fallback = e16;
    if (t14) t14->fallback = e16;
    if (title22) title22->fallback = t22;
    if (body16) body16->fallback = t16;
    if (label14) label14->fallback = t14;
}
