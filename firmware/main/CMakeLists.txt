idf_component_register(
    SRCS "main.c" "board.c" "usb.c" "keymap.c" "proto.c" "store.c" "ui.c" "theme.c" "fontpack.c" "assets.c"
    INCLUDE_DIRS "."
    EMBED_FILES "fonts/gsf-500.ttf" "fonts/gsf-700.ttf" "fonts/symbols.ttf"
    PRIV_REQUIRES esp_lcd esp_driver_spi esp_driver_i2c esp_driver_ledc esp_driver_gpio nvs_flash esp_timer esp_hw_support esp_system esp_partition fatfs wear_levelling vfs
)
# Lookup tables rely on zero-initialised trailing fields.
target_compile_options(${COMPONENT_LIB} PRIVATE -Wno-missing-field-initializers -Wno-format-truncation)
