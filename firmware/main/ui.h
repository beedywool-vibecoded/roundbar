#pragma once
#include <stdbool.h>

#include "cJSON.h"
#include "lvgl.h"

// All functions except ui_init must be called with the LVGL port lock held.
void ui_init(lv_display_t *disp, lv_indev_t *touch);
void ui_reload(void);                 // config changed: rebuild everything
void ui_set_profile(const char *id);  // focus changed on the PC
void ui_show(const char *profile, const char *page);
void ui_announce(void);               // resend the current view to the PC
void ui_toast(const char *text);
void ui_media(const char *title, const char *artist, const char *player, bool playing);
void ui_level(const char *kind, int value, bool muted); // "volume" | "mic"
void ui_stats(cJSON *msg);
