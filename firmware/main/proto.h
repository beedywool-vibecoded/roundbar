#pragma once
#include <stdbool.h>

#include "cJSON.h"
#include "freertos/FreeRTOS.h"
#include "freertos/queue.h"

#define FW_VERSION "0.1.0"

// Consumes lines from the USB queue and applies them (spec §2, PC -> device).
void proto_start(QueueHandle_t lines);

// Device -> PC helpers (safe to call from the LVGL task).
void proto_hello(void);
void proto_view(const char *profile, const char *page, const char *kind, bool awake);
void proto_action(const char *profile, const char *page, const char *btn, bool hold, cJSON *action);
void proto_dial(const char *source, int value);
void proto_log(const char *msg);
