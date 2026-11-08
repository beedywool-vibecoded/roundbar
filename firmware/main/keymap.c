#include "keymap.h"

#include <stdlib.h>
#include <string.h>

#include "keymaps.h"

typedef struct {
    const char *name;
    uint8_t key;
} named_t;

// Physical keys by their US name (HID usage page 0x07).
static const named_t NAMED[] = {
    {"enter", 0x28}, {"esc", 0x29}, {"backspace", 0x2A}, {"tab", 0x2B}, {"space", 0x2C},
    {"minus", 0x2D}, {"equal", 0x2E}, {"leftbracket", 0x2F}, {"rightbracket", 0x30}, {"backslash", 0x31},
    {"semicolon", 0x33}, {"quote", 0x34}, {"grave", 0x35}, {"comma", 0x36}, {"period", 0x37},
    {"slash", 0x38}, {"capslock", 0x39}, {"printscreen", 0x46}, {"insert", 0x49}, {"home", 0x4A},
    {"pageup", 0x4B}, {"delete", 0x4C}, {"end", 0x4D}, {"pagedown", 0x4E}, {"right", 0x4F},
    {"left", 0x50}, {"down", 0x51}, {"up", 0x52},
};

// Unknown ids fall back to the first layout (us).
static const km_layout_t *find_layout(const char *id)
{
    for (size_t i = 0; id && i < KEYMAP_COUNT; i++)
        if (!strcmp(KEYMAPS[i].id, id)) return &KEYMAPS[i];
    return &KEYMAPS[0];
}

const char *keymap_canonical(const char *id)
{
    for (size_t i = 0; id && i < KEYMAP_COUNT; i++)
        if (!strcmp(KEYMAPS[i].id, id)) return KEYMAPS[i].id;
    return NULL;
}

static int cmp_char(const void *key, const void *elem)
{
    uint32_t c = *(const uint32_t *)key;
    uint16_t e = ((const km_char_t *)elem)->ch;
    return c < e ? -1 : c > e ? 1 : 0;
}

static const km_char_t *lookup(const km_layout_t *l, uint32_t c)
{
    if (c > 0xFFFF) return NULL;
    return bsearch(&c, l->chars, l->n, sizeof(km_char_t), cmp_char);
}

// The physical key that types this letter on the layout (letters are always unshifted).
static uint8_t letter(const km_layout_t *l, char c)
{
    const km_char_t *e = lookup(l, (uint32_t)c);
    return e && !e->m0 && !e->k1 ? e->k0 : 0x04 + (c - 'a');
}

static uint8_t digit_us(char c) { return c == '0' ? 0x27 : 0x1E + (c - '1'); }

static uint8_t key_by_name(const char *k, size_t n, const km_layout_t *l)
{
    if (n == 1 && k[0] >= 'a' && k[0] <= 'z') return letter(l, k[0]);
    if (n == 1 && k[0] >= '0' && k[0] <= '9') return digit_us(k[0]);
    if (n >= 2 && n <= 3 && k[0] == 'f') {
        int f = atoi(k + 1);
        if (f >= 1 && f <= 12) return 0x3A + f - 1;
        if (f >= 13 && f <= 24) return 0x68 + f - 13;
    }
    for (size_t i = 0; i < sizeof(NAMED) / sizeof(NAMED[0]); i++)
        if (strlen(NAMED[i].name) == n && !strncmp(NAMED[i].name, k, n)) return NAMED[i].key;
    return 0;
}

bool keymap_combo(const char *combo, const char *layout, uint8_t *mods, uint8_t *key)
{
    const km_layout_t *l = find_layout(layout);
    *mods = 0;
    *key = 0;
    const char *p = combo;
    while (*p) {
        const char *e = strchr(p, '+');
        size_t n = e ? (size_t)(e - p) : strlen(p);
        if (n == 4 && !strncmp(p, "ctrl", 4)) *mods |= MOD_CTRL;
        else if (n == 5 && !strncmp(p, "shift", 5)) *mods |= MOD_SHIFT;
        else if (n == 3 && !strncmp(p, "alt", 3)) *mods |= MOD_ALT;
        else if (n == 5 && !strncmp(p, "super", 5)) *mods |= MOD_SUPER;
        else {
            *key = key_by_name(p, n, l);
            if (!*key) return false;
        }
        if (!e) break;
        p = e + 1;
    }
    return *key || *mods;
}

// ---- text -------------------------------------------------------------------

static uint32_t utf8_next(const char **p)
{
    const unsigned char *s = (const unsigned char *)*p;
    uint32_t c = s[0];
    int n = 1;
    if (c >= 0xF0) { c = ((c & 0x07) << 18) | ((s[1] & 0x3F) << 12) | ((s[2] & 0x3F) << 6) | (s[3] & 0x3F); n = 4; }
    else if (c >= 0xE0) { c = ((c & 0x0F) << 12) | ((s[1] & 0x3F) << 6) | (s[2] & 0x3F); n = 3; }
    else if (c >= 0xC0) { c = ((c & 0x1F) << 6) | (s[1] & 0x3F); n = 2; }
    for (int i = 1; i < n; i++)
        if (!s[i]) { n = i; break; }
    *p += n;
    return c;
}

int keymap_char(const char **p, const char *layout, keystroke_t out[2])
{
    const km_char_t *e = lookup(find_layout(layout), utf8_next(p));
    if (!e) return 0;
    out[0] = (keystroke_t){e->m0, e->k0};
    if (!e->k1) return 1;
    out[1] = (keystroke_t){e->m1, e->k1};
    return 2;
}

uint16_t keymap_media(const char *name)
{
    if (!strcmp(name, "play_pause")) return 0x00CD;
    if (!strcmp(name, "next")) return 0x00B5;
    if (!strcmp(name, "prev")) return 0x00B6;
    if (!strcmp(name, "vol_up")) return 0x00E9;
    if (!strcmp(name, "vol_down")) return 0x00EA;
    if (!strcmp(name, "mute")) return 0x00E2;
    return 0;
}
