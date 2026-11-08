// Guition JC3636W518: ESP32-S3, ST77916 360x360 over QSPI, CST816 touch on I2C.
#include "board.h"

#include <stdio.h>

#include "driver/gpio.h"
#include "driver/i2c_master.h"
#include "driver/ledc.h"
#include "driver/spi_master.h"
#include "esp_lcd_panel_io.h"
#include "esp_lcd_panel_ops.h"
#include "esp_lcd_st77916.h"
#include "esp_lcd_touch_cst816s.h"
#include "esp_log.h"
#include "esp_lvgl_port.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "lcd_init.h"

#define PIN_LCD_CS   10
#define PIN_LCD_SCK  9
#define PIN_LCD_D0   11
#define PIN_LCD_D1   12
#define PIN_LCD_D2   13
#define PIN_LCD_D3   14
#define PIN_LCD_RST  47
#define PIN_LCD_BL   15
#define PIN_TP_SDA   7
#define PIN_TP_SCL   8
#define PIN_TP_INT   41
#define PIN_TP_RST   40
#define LCD_HOST     SPI2_HOST
#define BUF_LINES    60

static const char *TAG = "board";
char touch_info[32] = "none";

static void backlight_init(void)
{
    const ledc_timer_config_t t = {
        .speed_mode = LEDC_LOW_SPEED_MODE,
        .duty_resolution = LEDC_TIMER_10_BIT,
        .timer_num = LEDC_TIMER_0,
        .freq_hz = 20000, // above hearing, no coil whine
        .clk_cfg = LEDC_AUTO_CLK,
    };
    ESP_ERROR_CHECK(ledc_timer_config(&t));
    const ledc_channel_config_t c = {
        .gpio_num = PIN_LCD_BL,
        .speed_mode = LEDC_LOW_SPEED_MODE,
        .channel = LEDC_CHANNEL_0,
        .timer_sel = LEDC_TIMER_0,
        .duty = 0,
    };
    ESP_ERROR_CHECK(ledc_channel_config(&c));
    ledc_fade_func_install(0);
}

void board_backlight(int pct)
{
    if (pct < 0) pct = 0;
    if (pct > 100) pct = 100;
    // Perceived brightness is roughly quadratic in duty.
    uint32_t duty = (uint32_t)(1023 * pct * pct / 10000);
    if (pct > 0 && duty < 4) duty = 4;
    ledc_set_fade_time_and_start(LEDC_LOW_SPEED_MODE, LEDC_CHANNEL_0, duty, 250, LEDC_FADE_NO_WAIT);
}

static esp_lcd_panel_handle_t s_panel;

void board_panel_sleep(bool sleep)
{
    if (!s_panel) return;
    if (sleep) {
        esp_lcd_panel_disp_on_off(s_panel, false);
        esp_lcd_panel_disp_sleep(s_panel, true);
    } else {
        esp_lcd_panel_disp_sleep(s_panel, false); // sleep-out needs ~120 ms before display-on
        vTaskDelay(pdMS_TO_TICKS(120));
        esp_lcd_panel_disp_on_off(s_panel, true);
    }
}

lv_display_t *board_init(lv_indev_t **touch)
{
    backlight_init();

    const spi_bus_config_t bus = ST77916_PANEL_BUS_QSPI_CONFIG(PIN_LCD_SCK, PIN_LCD_D0, PIN_LCD_D1, PIN_LCD_D2, PIN_LCD_D3,
                                                               LCD_W * BUF_LINES * sizeof(uint16_t));
    ESP_ERROR_CHECK(spi_bus_initialize(LCD_HOST, &bus, SPI_DMA_CH_AUTO));

    esp_lcd_panel_io_handle_t io = NULL;
    const esp_lcd_panel_io_spi_config_t io_cfg = ST77916_PANEL_IO_QSPI_CONFIG(PIN_LCD_CS, NULL, NULL);
    ESP_ERROR_CHECK(esp_lcd_new_panel_io_spi((esp_lcd_spi_bus_handle_t)LCD_HOST, &io_cfg, &io));

    st77916_vendor_config_t vendor = {
        .init_cmds = lcd_init_cmds,
        .init_cmds_size = sizeof(lcd_init_cmds) / sizeof(lcd_init_cmds[0]),
        .flags = {.use_qspi_interface = 1},
    };
    const esp_lcd_panel_dev_config_t panel_cfg = {
        .reset_gpio_num = PIN_LCD_RST,
        .rgb_ele_order = LCD_RGB_ELEMENT_ORDER_RGB,
        .bits_per_pixel = 16,
        .vendor_config = &vendor,
    };
    esp_lcd_panel_handle_t panel = NULL;
    ESP_ERROR_CHECK(esp_lcd_new_panel_st77916(io, &panel_cfg, &panel));
    ESP_ERROR_CHECK(esp_lcd_panel_reset(panel));
    ESP_ERROR_CHECK(esp_lcd_panel_init(panel));
    ESP_ERROR_CHECK(esp_lcd_panel_disp_on_off(panel, true));
    s_panel = panel;

    // Touch
    i2c_master_bus_handle_t i2c = NULL;
    const i2c_master_bus_config_t i2c_cfg = {
        .i2c_port = I2C_NUM_0,
        .sda_io_num = PIN_TP_SDA,
        .scl_io_num = PIN_TP_SCL,
        .clk_source = I2C_CLK_SRC_DEFAULT,
        .glitch_ignore_cnt = 7,
        .flags.enable_internal_pullup = true,
    };
    ESP_ERROR_CHECK(i2c_new_master_bus(&i2c_cfg, &i2c));
    esp_lcd_panel_io_handle_t tp_io = NULL;
    esp_lcd_panel_io_i2c_config_t tp_io_cfg = ESP_LCD_TOUCH_IO_I2C_CST816S_CONFIG();
    tp_io_cfg.scl_speed_hz = 400000;
    ESP_ERROR_CHECK(esp_lcd_new_panel_io_i2c(i2c, &tp_io_cfg, &tp_io));
    const esp_lcd_touch_config_t tp_cfg = {
        .x_max = LCD_W,
        .y_max = LCD_H,
        .rst_gpio_num = PIN_TP_RST,
        .int_gpio_num = PIN_TP_INT,
        .levels = {.reset = 0, .interrupt = 0},
        .flags = {.swap_xy = 0, .mirror_x = 0, .mirror_y = 0},
    };
    esp_lcd_touch_handle_t tp = NULL;
    esp_err_t err = esp_lcd_touch_new_i2c_cst816s(tp_io, &tp_cfg, &tp);
    if (err != ESP_OK) ESP_LOGE(TAG, "touch init failed: %s", esp_err_to_name(err));
    if (tp) {
        // esp_lvgl_port only reads the panel when INT fires. Out of the box the
        // CST816 pulses INT on press/release only, so LVGL never sees a drag.
        // IrqCtl (0xFA): EnTouch | EnChange -> pulse continuously while touched.
        // MotionMask (0xEC): EnConLR | EnConUD -> keep streaming during straight swipes.
        // DisAutoSleep (0xFE): don't drop the first touch after idling.
        const uint8_t regs[][2] = {{0xFA, 0x60}, {0xEC, 0x06}, {0xFE, 0x01}};
        for (int i = 0; i < 3; i++) esp_lcd_panel_io_tx_param(tp_io, regs[i][0], &regs[i][1], 1);
        uint8_t fa = 0, ec = 0, fe = 0;
        esp_lcd_panel_io_rx_param(tp_io, 0xFA, &fa, 1);
        esp_lcd_panel_io_rx_param(tp_io, 0xEC, &ec, 1);
        esp_lcd_panel_io_rx_param(tp_io, 0xFE, &fe, 1);
        snprintf(touch_info, sizeof(touch_info), "fa=%02x ec=%02x fe=%02x", fa, ec, fe);
    }

    // LVGL
    lvgl_port_cfg_t port_cfg = ESP_LVGL_PORT_INIT_CONFIG();
    port_cfg.task_stack = 12 * 1024;
    port_cfg.task_affinity = 1;
    port_cfg.task_max_sleep_ms = 500;
    ESP_ERROR_CHECK(lvgl_port_init(&port_cfg));

    const lvgl_port_display_cfg_t disp_cfg = {
        .io_handle = io,
        .panel_handle = panel,
        .buffer_size = LCD_W * BUF_LINES,
        .double_buffer = true,
        .hres = LCD_W,
        .vres = LCD_H,
        .color_format = LV_COLOR_FORMAT_RGB565,
        .flags = {.buff_dma = true, .swap_bytes = true},
    };
    lv_display_t *disp = lvgl_port_add_disp(&disp_cfg);

    *touch = NULL;
    if (tp) {
        const lvgl_port_touch_cfg_t touch_cfg = {.disp = disp, .handle = tp};
        *touch = lvgl_port_add_touch(&touch_cfg);
    }
    return disp;
}
