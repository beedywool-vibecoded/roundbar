// roundbar firmware entry point.
#include "board.h"
#include "esp_log.h"
#include "esp_lvgl_port.h"
#include "nvs_flash.h"
#include "proto.h"
#include "store.h"
#include "theme.h"
#include "ui.h"
#include "usb.h"

void app_main(void)
{
    esp_err_t err = nvs_flash_init();
    if (err == ESP_ERR_NVS_NO_FREE_PAGES || err == ESP_ERR_NVS_NEW_VERSION_FOUND) {
        nvs_flash_erase();
        nvs_flash_init();
    }
    store_init();

    lv_indev_t *touch = NULL;
    lv_display_t *disp = board_init(&touch);

    lvgl_port_lock(0);
    theme_fonts_init();
    ui_init(disp, touch);
    lvgl_port_unlock();

    QueueHandle_t lines = xQueueCreate(8, sizeof(char *));
    usb_init(lines);
    proto_start(lines);
    ESP_LOGI("main", "roundbar %s up", FW_VERSION);
}
