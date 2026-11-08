#include "store.h"

#include <stdlib.h>
#include <string.h>

#include "esp_heap_caps.h"
#include "esp_log.h"
#include "nvs.h"
#include "nvs_flash.h"

static const char *TAG = "store";
static cJSON *cfg;
static char sha[48];
static char layout[4] = "fr";
static int brightness = -1;

// Used until the daemon pushes the real one: media, a volume dial and stats.
static const char *FALLBACK =
    "{\"version\":1,\"theme\":{\"seed\":\"#6750A4\",\"mode\":\"dark\",\"palette\":{"
    "\"surface\":\"#141218\",\"surfaceContainer\":\"#211F26\",\"surfaceContainerHigh\":\"#2B2930\","
    "\"surfaceContainerHighest\":\"#36343B\",\"onSurface\":\"#E6E0E9\",\"onSurfaceVariant\":\"#CAC4D0\","
    "\"primary\":\"#D0BCFF\",\"onPrimary\":\"#381E72\",\"primaryContainer\":\"#4F378B\",\"onPrimaryContainer\":\"#EADDFF\","
    "\"secondaryContainer\":\"#4A4458\",\"onSecondaryContainer\":\"#E8DEF8\",\"tertiary\":\"#EFB8C8\","
    "\"tertiaryContainer\":\"#633B48\",\"onTertiaryContainer\":\"#FFD8E4\",\"error\":\"#F2B8B5\","
    "\"errorContainer\":\"#8C1D18\",\"onErrorContainer\":\"#F9DEDC\",\"outline\":\"#938F99\",\"outlineVariant\":\"#49454F\"}},"
    "\"device\":{\"brightness\":80,\"dimAfter\":30,\"sleepAfter\":300,\"longPressMs\":450,\"keyboardLayout\":\"fr\"},"
    "\"profiles\":[{\"id\":\"default\",\"name\":\"Default\",\"match\":[],\"pages\":["
    "{\"id\":\"media\",\"type\":\"media\",\"title\":\"Now playing\"},"
    "{\"id\":\"keys\",\"type\":\"radial\",\"title\":\"Desktop\",\"center\":{\"id\":\"ov\",\"icon\":\"apps\",\"label\":\"Overview\",\"color\":\"primary\",\"tap\":{\"type\":\"keys\",\"keys\":\"super\"}},"
    "\"buttons\":[{\"id\":\"c\",\"icon\":\"content_copy\",\"label\":\"Copy\",\"color\":\"secondary\",\"tap\":{\"type\":\"keys\",\"keys\":\"ctrl+c\"}},"
    "{\"id\":\"v\",\"icon\":\"content_paste\",\"label\":\"Paste\",\"color\":\"secondary\",\"tap\":{\"type\":\"keys\",\"keys\":\"ctrl+v\"}},"
    "{\"id\":\"z\",\"icon\":\"undo\",\"label\":\"Undo\",\"color\":\"surface\",\"tap\":{\"type\":\"keys\",\"keys\":\"ctrl+z\"}},"
    "{\"id\":\"s\",\"icon\":\"screenshot\",\"label\":\"Screenshot\",\"color\":\"tertiary\",\"tap\":{\"type\":\"keys\",\"keys\":\"printscreen\"}}]},"
    "{\"id\":\"volume\",\"type\":\"dial\",\"icon\":\"volume_up\",\"label\":\"Volume\",\"source\":\"volume\"}]}]}";

static void *spiram_malloc(size_t n) { return heap_caps_malloc(n, MALLOC_CAP_SPIRAM | MALLOC_CAP_8BIT); }

static void refresh_cache(void)
{
    cJSON *dev = cJSON_GetObjectItem(cfg, "device");
    const char *l = cJSON_GetStringValue(cJSON_GetObjectItem(dev, "keyboardLayout"));
    strlcpy(layout, l && !strcmp(l, "us") ? "us" : "fr", sizeof(layout));
}

void store_init(void)
{
    cJSON_Hooks hooks = {.malloc_fn = spiram_malloc, .free_fn = free};
    cJSON_InitHooks(&hooks);

    esp_err_t err = nvs_flash_init_partition("cfg");
    if (err == ESP_ERR_NVS_NO_FREE_PAGES || err == ESP_ERR_NVS_NEW_VERSION_FOUND) {
        nvs_flash_erase_partition("cfg");
        err = nvs_flash_init_partition("cfg");
    }
    nvs_handle_t h;
    if (err == ESP_OK && nvs_open_from_partition("cfg", "rb", NVS_READONLY, &h) == ESP_OK) {
        size_t len = 0;
        if (nvs_get_blob(h, "json", NULL, &len) == ESP_OK && len > 0) {
            char *buf = spiram_malloc(len + 1);
            if (buf && nvs_get_blob(h, "json", buf, &len) == ESP_OK) {
                buf[len] = 0;
                cfg = cJSON_Parse(buf);
            }
            free(buf);
        }
        size_t sl = sizeof(sha);
        if (nvs_get_str(h, "sha", sha, &sl) != ESP_OK) sha[0] = 0;
        int32_t b;
        if (nvs_get_i32(h, "bright", &b) == ESP_OK) brightness = b;
        nvs_close(h);
    }
    if (!cfg) {
        ESP_LOGW(TAG, "no stored config, using fallback");
        cfg = cJSON_Parse(FALLBACK);
        sha[0] = 0;
    }
    refresh_cache();
}

cJSON *store_config(void) { return cfg; }

char *store_json_copy(size_t *len)
{
    nvs_handle_t h;
    char *buf = NULL;
    *len = 0;
    if (!sha[0] || nvs_open_from_partition("cfg", "rb", NVS_READONLY, &h) != ESP_OK) return NULL;
    size_t n = 0;
    if (nvs_get_blob(h, "json", NULL, &n) == ESP_OK && n > 0 && (buf = spiram_malloc(n + 1))) {
        if (nvs_get_blob(h, "json", buf, &n) == ESP_OK) {
            buf[n] = 0;
            *len = n;
        } else {
            free(buf);
            buf = NULL;
        }
    }
    nvs_close(h);
    return buf;
}
const char *store_sha(void) { return sha; }
const char *store_layout(void) { return layout; }

int store_get_int(const char *key, int def)
{
    cJSON *v = cJSON_GetObjectItem(cJSON_GetObjectItem(cfg, "device"), key);
    return cJSON_IsNumber(v) ? v->valueint : def;
}

bool store_save(const char *json, const char *new_sha)
{
    cJSON *next = cJSON_Parse(json);
    if (!next || !cJSON_IsArray(cJSON_GetObjectItem(next, "profiles"))) {
        cJSON_Delete(next);
        return false;
    }
    nvs_handle_t h;
    if (nvs_open_from_partition("cfg", "rb", NVS_READWRITE, &h) == ESP_OK) {
        nvs_set_blob(h, "json", json, strlen(json));
        nvs_set_str(h, "sha", new_sha);
        nvs_erase_key(h, "bright"); // a new config brings its own brightness
        nvs_commit(h);
        nvs_close(h);
    }
    cJSON_Delete(cfg);
    cfg = next;
    strlcpy(sha, new_sha, sizeof(sha));
    brightness = -1;
    refresh_cache();
    return true;
}

bool store_get_bool(const char *key, bool def)
{
    cJSON *v = cJSON_GetObjectItem(cJSON_GetObjectItem(cfg, "device"), key);
    return cJSON_IsBool(v) ? cJSON_IsTrue(v) : def;
}

int store_local_get(const char *key, int def)
{
    nvs_handle_t h;
    int32_t v = def;
    if (nvs_open_from_partition("cfg", "local", NVS_READONLY, &h) == ESP_OK) {
        nvs_get_i32(h, key, &v);
        nvs_close(h);
    }
    return v;
}

void store_local_set(const char *key, int value)
{
    nvs_handle_t h;
    if (nvs_open_from_partition("cfg", "local", NVS_READWRITE, &h) == ESP_OK) {
        nvs_set_i32(h, key, value);
        nvs_commit(h);
        nvs_close(h);
    }
}

int store_brightness(void) { return brightness >= 0 ? brightness : store_get_int("brightness", 80); }

void store_set_brightness(int pct)
{
    brightness = pct;
    nvs_handle_t h;
    if (nvs_open_from_partition("cfg", "rb", NVS_READWRITE, &h) == ESP_OK) {
        nvs_set_i32(h, "bright", pct);
        nvs_commit(h);
        nvs_close(h);
    }
}
