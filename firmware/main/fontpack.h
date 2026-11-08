#pragma once
#include "lvgl.h"

// Maps the "fonts" partition (tools/fontpack output) and chains its CJK and
// emoji fonts behind the given Latin fonts as LVGL fallbacks. Safe to call
// when the partition is empty: nothing is chained then.
void fontpack_attach(lv_font_t *title22, lv_font_t *body16, lv_font_t *label14);
