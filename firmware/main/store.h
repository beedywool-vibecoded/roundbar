#pragma once
#include <stdbool.h>

#include "cJSON.h"

// Persists the config pushed by the daemon in the "cfg" NVS partition.
void store_init(void);
// Parsed config. Never NULL after store_init (falls back to a built-in default).
cJSON *store_config(void);
const char *store_sha(void);
char *store_json_copy(size_t *len); // the stored config bytes (free() it), NULL if none
// Takes ownership of nothing; copies json. Returns false if it doesn't parse.
bool store_save(const char *json, const char *sha);
const char *store_layout(void); // "fr" | "us"
int store_get_int(const char *key, int def); // device.<key>
// Local brightness override from a brightness dial (-1 = use config).
int store_brightness(void);
void store_set_brightness(int pct);
bool store_get_bool(const char *key, bool def); // device.<key>
// Small device-only settings kept in flash (e.g. "autosw").
int store_local_get(const char *key, int def);
void store_local_set(const char *key, int value);
