#pragma once
#include "lvgl.h"

#define LCD_W 360
#define LCD_H 360

// Brings up the QSPI panel, the touch controller and LVGL (esp_lvgl_port).
lv_display_t *board_init(lv_indev_t **touch);
// 0..100 %, 0 turns the backlight off.
void board_backlight(int pct);

// CST816 register readback after init, reported in the hello message.
extern char touch_info[32];
