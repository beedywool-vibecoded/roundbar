#pragma once
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

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
// Blurred album art for media pages (RGB565, w*h), NULL to clear. The buffer must
// stay valid until the next call.
void ui_cover(const uint8_t *rgb565, int w, int h);
// Palette for media pages only ({role: "#RRGGBB"}), NULL restores the theme.
void ui_media_theme(cJSON *palette);
// Asset files changed on flash: rebuild so image pages reload them.
void ui_assets_changed(void);
void ui_time(cJSON *msg);      // {"t":"time"}
void ui_weather(cJSON *msg);   // {"t":"weather"}
void ui_icon(const char *name, int w, int h, const uint8_t *data, size_t len);
void ui_windows(cJSON *list);
void ui_mixer(cJSON *msg);
void ui_clips(cJSON *list);
void ui_notifs(cJSON *list);
void ui_notif_toast(cJSON *n);
void ui_discord(cJSON *state);
void ui_perf(int ms);
void ui_obs(cJSON *state);
void ui_game(cJSON *msg);
void ui_game_art(const uint8_t *rgb565, int w, int h); // measure frame times, report as a log line
