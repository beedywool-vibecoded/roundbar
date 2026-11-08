#include "proto.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "esp_log.h"
#include "esp_lvgl_port.h"
#include "freertos/task.h"
#include "store.h"
#include "ui.h"
#include "usb.h"

static const char *TAG = "proto";

static void send_json(cJSON *o)
{
    char *s = cJSON_PrintUnformatted(o);
    if (s) {
        usb_send_line(s);
        free(s);
    }
    cJSON_Delete(o);
}

void proto_hello(void)
{
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", "hello");
    cJSON_AddStringToObject(o, "fw", FW_VERSION);
    cJSON_AddNumberToObject(o, "proto", 1);
    cJSON_AddStringToObject(o, "cfg", store_sha());
    cJSON_AddNumberToObject(o, "w", 360);
    cJSON_AddNumberToObject(o, "h", 360);
    send_json(o);
}

void proto_view(const char *profile, const char *page, const char *kind, bool awake)
{
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", "view");
    cJSON_AddStringToObject(o, "profile", profile);
    cJSON_AddStringToObject(o, "page", page);
    cJSON_AddStringToObject(o, "kind", kind);
    cJSON_AddBoolToObject(o, "awake", awake);
    send_json(o);
}

void proto_action(const char *profile, const char *page, const char *btn, bool hold, cJSON *action)
{
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", "action");
    cJSON_AddStringToObject(o, "profile", profile);
    cJSON_AddStringToObject(o, "page", page);
    cJSON_AddStringToObject(o, "btn", btn ? btn : "");
    cJSON_AddBoolToObject(o, "hold", hold);
    cJSON_AddItemToObject(o, "action", cJSON_Duplicate(action, true));
    send_json(o);
}

void proto_dial(const char *source, int value)
{
    char buf[80];
    snprintf(buf, sizeof(buf), "{\"t\":\"dial\",\"source\":\"%s\",\"value\":%d}", source, value);
    usb_send_line(buf);
}

void proto_log(const char *msg)
{
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", "log");
    cJSON_AddStringToObject(o, "msg", msg);
    send_json(o);
}

static void ack(const char *id, bool ok, const char *err)
{
    if (!id) return;
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "t", "ack");
    cJSON_AddStringToObject(o, "id", id);
    cJSON_AddBoolToObject(o, "ok", ok);
    if (err) cJSON_AddStringToObject(o, "err", err);
    send_json(o);
}

static const char *str(cJSON *o, const char *k) { return cJSON_GetStringValue(cJSON_GetObjectItem(o, k)); }
static int num(cJSON *o, const char *k, int def)
{
    cJSON *v = cJSON_GetObjectItem(o, k);
    return cJSON_IsNumber(v) ? v->valueint : def;
}
static bool flag(cJSON *o, const char *k) { return cJSON_IsTrue(cJSON_GetObjectItem(o, k)); }

static void handle(cJSON *m)
{
    const char *t = str(m, "t");
    const char *id = str(m, "id");
    if (!t) return;

    if (!strcmp(t, "hello")) {
        proto_hello();
        lvgl_port_lock(0);
        ui_announce();
        lvgl_port_unlock();
    } else if (!strcmp(t, "config")) {
        cJSON *cfg = cJSON_GetObjectItem(m, "cfg");
        const char *sha = str(m, "sha");
        char *raw = cfg ? cJSON_PrintUnformatted(cfg) : NULL;
        bool ok = raw && sha && store_save(raw, sha);
        free(raw);
        ack(id, ok, ok ? NULL : "invalid config");
        if (ok) {
            lvgl_port_lock(0);
            ui_reload();
            lvgl_port_unlock();
        }
    } else if (!strcmp(t, "profile")) {
        lvgl_port_lock(0);
        ui_set_profile(str(m, "id"));
        lvgl_port_unlock();
    } else if (!strcmp(t, "show")) {
        lvgl_port_lock(0);
        ui_show(str(m, "profile"), str(m, "page"));
        lvgl_port_unlock();
    } else if (!strcmp(t, "app")) {
        const char *name = str(m, "name");
        if (name) {
            lvgl_port_lock(0);
            ui_toast(name);
            lvgl_port_unlock();
        }
    } else if (!strcmp(t, "media")) {
        lvgl_port_lock(0);
        ui_media(str(m, "title"), str(m, "artist"), str(m, "player"), flag(m, "playing"));
        lvgl_port_unlock();
    } else if (!strcmp(t, "volume") || !strcmp(t, "mic")) {
        lvgl_port_lock(0);
        ui_level(t, num(m, "value", 0), flag(m, "muted"));
        lvgl_port_unlock();
    } else if (!strcmp(t, "stats")) {
        lvgl_port_lock(0);
        ui_stats(m);
        lvgl_port_unlock();
    } else if (!strcmp(t, "hid")) {
        if (str(m, "keys")) usb_hid(HID_KEYS, str(m, "keys"), id);
        else if (str(m, "text")) usb_hid(HID_TEXT, str(m, "text"), id);
        else if (str(m, "media")) usb_hid(HID_MEDIA, str(m, "media"), id);
        else ack(id, false, "nothing to type");
    } else if (!strcmp(t, "bootloader")) {
        vTaskDelay(pdMS_TO_TICKS(100));
        usb_reboot_to_bootloader();
    }
}

static void proto_task(void *arg)
{
    QueueHandle_t q = arg;
    char *line;
    for (;;) {
        if (xQueueReceive(q, &line, portMAX_DELAY) != pdTRUE) continue;
        cJSON *m = cJSON_Parse(line);
        free(line);
        if (!m) {
            ESP_LOGW(TAG, "bad line");
            continue;
        }
        handle(m);
        cJSON_Delete(m);
    }
}

void proto_start(QueueHandle_t lines)
{
    xTaskCreatePinnedToCore(proto_task, "proto", 8192, lines, 4, NULL, 0);
}
