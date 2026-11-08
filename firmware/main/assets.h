#pragma once
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#include "cJSON.h"

// Image files for "image" pages, on the FAT "assets" partition at /assets.
void assets_init(void);
cJSON *assets_list(void);      // ["name", ...]
uint64_t assets_free(void);    // bytes
// Chunk i of n of a file; written to a temp file and renamed after the last.
bool assets_write(const char *name, int i, int n, const uint8_t *data, size_t len);
void assets_prune(cJSON *keep); // delete files not in the array
// Reads a whole file into PSRAM (caller frees). NULL if missing.
uint8_t *assets_load(const char *name, size_t *len);
