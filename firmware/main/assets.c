#include "assets.h"

#include <dirent.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

#include "esp_heap_caps.h"
#include "esp_log.h"
#include "esp_vfs_fat.h"
#include "wear_levelling.h"

static const char *TAG = "assets";
#define ROOT "/assets"
static bool mounted;
static wl_handle_t wl = WL_INVALID_HANDLE;

void assets_init(void)
{
    const esp_vfs_fat_mount_config_t cfg = {
        .format_if_mount_failed = true,
        .max_files = 4,
        .allocation_unit_size = 4096,
    };
    esp_err_t err = esp_vfs_fat_spiflash_mount_rw_wl(ROOT, "assets", &cfg, &wl);
    mounted = err == ESP_OK;
    if (!mounted) ESP_LOGE(TAG, "mount failed: %s", esp_err_to_name(err));
}

static bool safe(const char *name)
{
    return name && name[0] && name[0] != '.' && !strchr(name, '/') && strlen(name) < 48;
}

cJSON *assets_list(void)
{
    cJSON *arr = cJSON_CreateArray();
    DIR *d = mounted ? opendir(ROOT) : NULL;
    if (!d) return arr;
    struct dirent *e;
    while ((e = readdir(d))) {
        if (e->d_name[0] == '.' || strstr(e->d_name, ".part")) continue;
        cJSON_AddItemToArray(arr, cJSON_CreateString(e->d_name));
    }
    closedir(d);
    return arr;
}

uint64_t assets_free(void)
{
    uint64_t total = 0, free_b = 0;
    if (mounted) esp_vfs_fat_info(ROOT, &total, &free_b);
    return free_b;
}

bool assets_write(const char *name, int i, int n, const uint8_t *data, size_t len)
{
    if (!mounted || !safe(name) || i < 0 || i >= n) return false;
    char tmp[80], path[80];
    snprintf(tmp, sizeof(tmp), ROOT "/%s.part", name);
    snprintf(path, sizeof(path), ROOT "/%s", name);
    FILE *f = fopen(tmp, i == 0 ? "wb" : "ab");
    if (!f) return false;
    bool ok = fwrite(data, 1, len, f) == len;
    ok = fclose(f) == 0 && ok;
    if (!ok) {
        unlink(tmp); // most likely full
        return false;
    }
    if (i == n - 1) {
        unlink(path);
        ok = rename(tmp, path) == 0;
    }
    return ok;
}

void assets_prune(cJSON *keep)
{
    DIR *d = mounted ? opendir(ROOT) : NULL;
    if (!d) return;
    char victims[64][48];
    int nv = 0;
    struct dirent *e;
    while ((e = readdir(d)) && nv < 64) {
        if (e->d_name[0] == '.') continue;
        bool kept = false;
        cJSON *k;
        cJSON_ArrayForEach(k, keep)
        {
            if (cJSON_IsString(k) && !strcmp(k->valuestring, e->d_name)) kept = true;
        }
        if (!kept) strlcpy(victims[nv++], e->d_name, sizeof(victims[0]));
    }
    closedir(d);
    for (int i = 0; i < nv; i++) {
        char path[80];
        snprintf(path, sizeof(path), ROOT "/%s", victims[i]);
        unlink(path);
    }
}

uint8_t *assets_load(const char *name, size_t *len)
{
    if (!mounted || !safe(name)) return NULL;
    char path[80];
    snprintf(path, sizeof(path), ROOT "/%s", name);
    struct stat st;
    if (stat(path, &st) != 0 || st.st_size <= 0) return NULL;
    uint8_t *buf = heap_caps_malloc(st.st_size, MALLOC_CAP_SPIRAM);
    FILE *f = buf ? fopen(path, "rb") : NULL;
    if (!f) {
        free(buf);
        return NULL;
    }
    size_t got = fread(buf, 1, st.st_size, f);
    fclose(f);
    if (got != (size_t)st.st_size) {
        free(buf);
        return NULL;
    }
    *len = got;
    return buf;
}
