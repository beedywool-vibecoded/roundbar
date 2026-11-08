#pragma once
#include <stdbool.h>
#include <stdint.h>

#define MOD_CTRL  0x01
#define MOD_SHIFT 0x02
#define MOD_ALT   0x04
#define MOD_SUPER 0x08
#define MOD_ALTGR 0x40

typedef struct {
    uint8_t mods, key;
} keystroke_t;

// Layouts come from keymaps.h, generated from shared/keymaps.json. Unknown ids act as "us".
// Returns the table's own copy of a known layout id (a static string), NULL otherwise.
const char *keymap_canonical(const char *id);
// "ctrl+shift+t" -> modifiers + HID usage. Letters name the character typed on
// the host layout. A lone modifier gives key = 0.
bool keymap_combo(const char *combo, const char *layout, uint8_t *mods, uint8_t *key);
// Consumes one UTF-8 character from *p and returns 0..2 keystrokes for it
// (2 for a dead key followed by a letter or space, such as ê on AZERTY).
int keymap_char(const char **p, const char *layout, keystroke_t out[2]);
// "play_pause" etc -> consumer usage, 0 if unknown.
uint16_t keymap_media(const char *name);
