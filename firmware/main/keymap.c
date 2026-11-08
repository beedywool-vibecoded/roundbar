#include "keymap.h"

#include <stdlib.h>
#include <string.h>

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

static uint8_t letter(char c, bool fr)
{
    if (fr) {
        switch (c) { // AZERTY moves these letters
        case 'a': return 0x14; // US q position
        case 'q': return 0x04;
        case 'z': return 0x1A;
        case 'w': return 0x1D;
        case 'm': return 0x33;
        }
    }
    return 0x04 + (c - 'a');
}

static uint8_t digit_us(char c) { return c == '0' ? 0x27 : 0x1E + (c - '1'); }

static uint8_t key_by_name(const char *k, size_t n, bool fr)
{
    if (n == 1 && k[0] >= 'a' && k[0] <= 'z') return letter(k[0], fr);
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
    bool fr = layout && !strcmp(layout, "fr");
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
            *key = key_by_name(p, n, fr);
            if (!*key) return false;
        }
        if (!e) break;
        p = e + 1;
    }
    return *key || *mods;
}

// ---- text -------------------------------------------------------------------

#define S MOD_SHIFT
#define G MOD_ALTGR

typedef struct {
    uint32_t ch;
    uint8_t mods, key;
    bool dead; // follow with space to get the bare character
} chmap_t;

static const chmap_t FR[] = {
    {' ', 0, 0x2C}, {'\n', 0, 0x28}, {'\t', 0, 0x2B},
    {'&', 0, 0x1E}, {0xE9, 0, 0x1F}, {'"', 0, 0x20}, {'\'', 0, 0x21}, {'(', 0, 0x22}, {'-', 0, 0x23},
    {0xE8, 0, 0x24}, {'_', 0, 0x25}, {0xE7, 0, 0x26}, {0xE0, 0, 0x27}, {')', 0, 0x2D}, {'=', 0, 0x2E},
    {'1', S, 0x1E}, {'2', S, 0x1F}, {'3', S, 0x20}, {'4', S, 0x21}, {'5', S, 0x22}, {'6', S, 0x23},
    {'7', S, 0x24}, {'8', S, 0x25}, {'9', S, 0x26}, {'0', S, 0x27}, {'+', S, 0x2E},
    {'~', G, 0x1F, true}, {'#', G, 0x20}, {'{', G, 0x21}, {'[', G, 0x22}, {'|', G, 0x23}, {'`', G, 0x24, true},
    {'\\', G, 0x25}, {'^', 0, 0x2F, true}, {'@', G, 0x27}, {']', G, 0x2D}, {'}', G, 0x2E},
    {'$', 0, 0x30}, {0xF9, 0, 0x34}, {'%', S, 0x34}, {'*', 0, 0x31},
    {',', 0, 0x10}, {'?', S, 0x10}, {';', 0, 0x36}, {'.', S, 0x36}, {':', 0, 0x37}, {'/', S, 0x37},
    {'!', 0, 0x38}, {'<', 0, 0x64}, {'>', S, 0x64},
};

static const chmap_t US[] = {
    {' ', 0, 0x2C}, {'\n', 0, 0x28}, {'\t', 0, 0x2B},
    {'!', S, 0x1E}, {'@', S, 0x1F}, {'#', S, 0x20}, {'$', S, 0x21}, {'%', S, 0x22}, {'^', S, 0x23},
    {'&', S, 0x24}, {'*', S, 0x25}, {'(', S, 0x26}, {')', S, 0x27},
    {'-', 0, 0x2D}, {'_', S, 0x2D}, {'=', 0, 0x2E}, {'+', S, 0x2E}, {'[', 0, 0x2F}, {'{', S, 0x2F},
    {']', 0, 0x30}, {'}', S, 0x30}, {'\\', 0, 0x31}, {'|', S, 0x31}, {';', 0, 0x33}, {':', S, 0x33},
    {'\'', 0, 0x34}, {'"', S, 0x34}, {'`', 0, 0x35}, {'~', S, 0x35}, {',', 0, 0x36}, {'<', S, 0x36},
    {'.', 0, 0x37}, {'>', S, 0x37}, {'/', 0, 0x38}, {'?', S, 0x38},
};

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
    bool fr = layout && !strcmp(layout, "fr");
    uint32_t c = utf8_next(p);
    if (c >= 'a' && c <= 'z') { out[0] = (keystroke_t){0, letter((char)c, fr)}; return 1; }
    if (c >= 'A' && c <= 'Z') { out[0] = (keystroke_t){MOD_SHIFT, letter((char)(c - 'A' + 'a'), fr)}; return 1; }
    if (!fr && c >= '0' && c <= '9') { out[0] = (keystroke_t){0, digit_us((char)c)}; return 1; }
    const chmap_t *t = fr ? FR : US;
    size_t n = fr ? sizeof(FR) / sizeof(FR[0]) : sizeof(US) / sizeof(US[0]);
    for (size_t i = 0; i < n; i++) {
        if (t[i].ch != c) continue;
        out[0] = (keystroke_t){t[i].mods, t[i].key};
        if (t[i].dead) { out[1] = (keystroke_t){0, 0x2C}; return 2; }
        return 1;
    }
    return 0;
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
