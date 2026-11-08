#include "usb.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "esp_attr.h"
#include "esp_log.h"
#include "esp_mac.h"
#include "esp_system.h"
#include "freertos/semphr.h"
#include "freertos/task.h"
#include "keymap.h"
#include "esp_rom_sys.h"
#include "rom/ets_sys.h"
#include "soc/rtc_cntl_reg.h"
#include "store.h"
#include "tinyusb.h"
#include "tusb_cdc_acm.h"

static const char *TAG = "usb";

#define RX_LINE_MAX (64 * 1024 + 256)

enum { ITF_CDC = 0, ITF_CDC_DATA, ITF_HID, ITF_TOTAL };
enum { RID_KBD = 1, RID_CONSUMER = 2, RID_MOUSE = 3 };
#define EP_CDC_NOTIF 0x81
#define EP_CDC_OUT   0x02
#define EP_CDC_IN    0x82
#define EP_HID       0x83
#define CFG_LEN (TUD_CONFIG_DESC_LEN + TUD_CDC_DESC_LEN + TUD_HID_DESC_LEN)

static const uint8_t hid_report[] = {
    TUD_HID_REPORT_DESC_KEYBOARD(HID_REPORT_ID(RID_KBD)),
    TUD_HID_REPORT_DESC_CONSUMER(HID_REPORT_ID(RID_CONSUMER)),
    TUD_HID_REPORT_DESC_MOUSE(HID_REPORT_ID(RID_MOUSE)),
};

static const uint8_t cfg_desc[] = {
    TUD_CONFIG_DESCRIPTOR(1, ITF_TOTAL, 0, CFG_LEN, TUSB_DESC_CONFIG_ATT_REMOTE_WAKEUP, 100),
    TUD_CDC_DESCRIPTOR(ITF_CDC, 4, EP_CDC_NOTIF, 8, EP_CDC_OUT, EP_CDC_IN, 64),
    TUD_HID_DESCRIPTOR(ITF_HID, 5, HID_ITF_PROTOCOL_NONE, sizeof(hid_report), EP_HID, 16, 5),
};

static const tusb_desc_device_t dev_desc = {
    .bLength = sizeof(tusb_desc_device_t),
    .bDescriptorType = TUSB_DESC_DEVICE,
    .bcdUSB = 0x0200,
    .bDeviceClass = TUSB_CLASS_MISC,
    .bDeviceSubClass = MISC_SUBCLASS_COMMON,
    .bDeviceProtocol = MISC_PROTOCOL_IAD,
    .bMaxPacketSize0 = CFG_TUD_ENDPOINT0_SIZE,
    .idVendor = 0x303A,
    .idProduct = 0x4011,
    .bcdDevice = 0x0100,
    .iManufacturer = 1,
    .iProduct = 2,
    .iSerialNumber = 3,
    .bNumConfigurations = 1,
};

static char serial[13];
static const char *strings[] = {
    "\x09\x04", "roundbar", "roundbar", serial, "roundbar link", "roundbar keys",
};

static QueueHandle_t line_q;
static char *line_buf;
static size_t line_len;
static bool line_overflow;
static volatile bool dtr;
static SemaphoreHandle_t tx_mu;

typedef struct {
    hid_kind_t kind;
    char arg[72];
    char ack[16];
    uint16_t hold; // ms a key combo stays down (0 = a normal tap)
} hid_job_t;
static QueueHandle_t hid_q;

static bool wait_ready(void);

static portMUX_TYPE mouse_mux = portMUX_INITIALIZER_UNLOCKED;
static int32_t mouse_dx, mouse_dy, mouse_wheel;
static uint8_t mouse_btn;

static void mouse_kick(void)
{
    hid_job_t job = {.kind = HID_MOUSE};
    xQueueSend(hid_q, &job, 0); // if the queue is full, the accumulator keeps the motion
}

void usb_mouse_move(int dx, int dy)
{
    portENTER_CRITICAL(&mouse_mux);
    mouse_dx += dx;
    mouse_dy += dy;
    portEXIT_CRITICAL(&mouse_mux);
    mouse_kick();
}

void usb_mouse_wheel(int ticks)
{
    portENTER_CRITICAL(&mouse_mux);
    mouse_wheel += ticks;
    portEXIT_CRITICAL(&mouse_mux);
    mouse_kick();
}

void usb_mouse_buttons(uint8_t buttons)
{
    mouse_btn = buttons;
    mouse_kick();
}

static int8_t take(int32_t *acc)
{
    int32_t v = *acc;
    if (v > 127) v = 127;
    if (v < -127) v = -127;
    *acc -= v;
    return (int8_t)v;
}

// Sends everything accumulated, in ±127 steps.
static void mouse_flush(void)
{
    for (int guard = 0; guard < 16; guard++) {
        portENTER_CRITICAL(&mouse_mux);
        int8_t x = take(&mouse_dx), y = take(&mouse_dy), w = take(&mouse_wheel);
        portEXIT_CRITICAL(&mouse_mux);
        static uint8_t sent_btn;
        if (!x && !y && !w && sent_btn == mouse_btn) return;
        if (!wait_ready()) return;
        sent_btn = mouse_btn;
        tud_hid_mouse_report(RID_MOUSE, sent_btn, x, y, w, 0);
    }
}

void usb_mouse_click(uint8_t button)
{
    hid_job_t job = {.kind = HID_MOUSE, .arg = {(char)button}};
    xQueueSend(hid_q, &job, pdMS_TO_TICKS(20));
}

// ---- HID callbacks required by TinyUSB ------------------------------------

uint8_t const *tud_hid_descriptor_report_cb(uint8_t instance)
{
    (void)instance;
    return hid_report;
}

uint16_t tud_hid_get_report_cb(uint8_t instance, uint8_t report_id, hid_report_type_t type, uint8_t *buf, uint16_t len)
{
    return 0;
}

void tud_hid_set_report_cb(uint8_t instance, uint8_t report_id, hid_report_type_t type, uint8_t const *buf, uint16_t len)
{
}

// ---- CDC -------------------------------------------------------------------

static void cdc_rx(int itf, cdcacm_event_t *event)
{
    uint8_t chunk[CONFIG_TINYUSB_CDC_RX_BUFSIZE];
    size_t n = 0;
    if (tinyusb_cdcacm_read(itf, chunk, sizeof(chunk), &n) != ESP_OK) return;
    for (size_t i = 0; i < n; i++) {
        char c = (char)chunk[i];
        if (c == '\n') {
            if (!line_overflow && line_len > 0) {
                char *s = malloc(line_len + 1);
                if (s) {
                    memcpy(s, line_buf, line_len);
                    s[line_len] = 0;
                    if (xQueueSend(line_q, &s, 0) != pdTRUE) free(s);
                }
            }
            line_len = 0;
            line_overflow = false;
        } else if (c != '\r') {
            if (line_len < RX_LINE_MAX - 1) line_buf[line_len++] = c;
            else line_overflow = true;
        }
    }
}

static void cdc_line_state(int itf, cdcacm_event_t *event)
{
    bool now = event->line_state_changed_data.dtr;
    if (now && !dtr) {
        // A host just opened the port: introduce ourselves.
        dtr = true;
        char *s = strdup("{\"t\":\"hello\"}");
        if (s && xQueueSend(line_q, &s, 0) != pdTRUE) free(s);
    }
    dtr = now;
}

static void cdc_line_coding(int itf, cdcacm_event_t *event)
{
    // Arduino-style "1200 baud touch": `stty -F <port> 1200` reboots into the bootloader.
    if (event->line_coding_changed_data.p_line_coding->bit_rate == 1200) usb_reboot_to_bootloader();
}

bool usb_host_listening(void) { return dtr; }

static volatile bool suspended;
void tud_suspend_cb(bool remote_wakeup_en) { suspended = true; }
void tud_resume_cb(void) { suspended = false; }
bool usb_suspended(void) { return suspended || !tud_mounted(); }

void usb_send_line(const char *json)
{
    if (!dtr || !tud_mounted()) return;
    xSemaphoreTake(tx_mu, portMAX_DELAY);
    size_t len = strlen(json), off = 0;
    int spins = 0;
    while (off < len && spins < 200) {
        size_t q = tinyusb_cdcacm_write_queue(TINYUSB_CDC_ACM_0, (const uint8_t *)json + off, len - off);
        off += q;
        if (off < len) {
            tinyusb_cdcacm_write_flush(TINYUSB_CDC_ACM_0, pdMS_TO_TICKS(20));
            spins++;
        }
    }
    tinyusb_cdcacm_write_queue_char(TINYUSB_CDC_ACM_0, '\n');
    tinyusb_cdcacm_write_flush(TINYUSB_CDC_ACM_0, pdMS_TO_TICKS(20));
    xSemaphoreGive(tx_mu);
}

// ---- HID typing --------------------------------------------------------------

static bool wait_ready(void)
{
    for (int i = 0; i < 100; i++) {
        if (!tud_mounted()) return false;
        if (tud_suspended()) tud_remote_wakeup();
        if (tud_hid_ready()) return true;
        vTaskDelay(pdMS_TO_TICKS(2));
    }
    return false;
}

static void tap_hold(uint8_t mods, uint8_t key, int hold_ms)
{
    uint8_t keys[6] = {key, 0, 0, 0, 0, 0};
    if (wait_ready()) tud_hid_keyboard_report(RID_KBD, mods, key ? keys : NULL);
    vTaskDelay(pdMS_TO_TICKS(hold_ms));
    if (wait_ready()) tud_hid_keyboard_report(RID_KBD, 0, NULL);
    vTaskDelay(pdMS_TO_TICKS(12));
}

static void tap(uint8_t mods, uint8_t key)
{
    uint8_t keys[6] = {key, 0, 0, 0, 0, 0};
    if (wait_ready()) tud_hid_keyboard_report(RID_KBD, mods, key ? keys : NULL);
    vTaskDelay(pdMS_TO_TICKS(12));
    if (wait_ready()) tud_hid_keyboard_report(RID_KBD, 0, NULL);
    vTaskDelay(pdMS_TO_TICKS(12));
}

static void consumer(uint16_t usage)
{
    if (wait_ready()) tud_hid_report(RID_CONSUMER, &usage, 2);
    vTaskDelay(pdMS_TO_TICKS(15));
    uint16_t zero = 0;
    if (wait_ready()) tud_hid_report(RID_CONSUMER, &zero, 2);
    vTaskDelay(pdMS_TO_TICKS(10));
}

static void hid_task(void *arg)
{
    hid_job_t job;
    for (;;) {
        if (xQueueReceive(hid_q, &job, portMAX_DELAY) != pdTRUE) continue;
        bool ok = true;
        const char *layout = store_layout();
        switch (job.kind) {
        case HID_KEYS: {
            uint8_t mods, key;
            ok = keymap_combo(job.arg, layout, &mods, &key);
            if (ok) {
                if (key == 0 && mods) {
                    // lone modifier (e.g. super): press and release it alone
                    if (wait_ready()) tud_hid_keyboard_report(RID_KBD, mods, NULL);
                    vTaskDelay(pdMS_TO_TICKS(30));
                    if (wait_ready()) tud_hid_keyboard_report(RID_KBD, 0, NULL);
                } else if (job.hold) {
                    tap_hold(mods, key, job.hold);
                } else {
                    tap(mods, key);
                }
            }
            break;
        }
        case HID_TEXT: {
            const char *p = job.arg;
            while (*p) {
                keystroke_t ks[2];
                int n = keymap_char(&p, layout, ks);
                for (int i = 0; i < n; i++) tap(ks[i].mods, ks[i].key);
                if (n == 0) ok = false;
            }
            break;
        }
        case HID_MOUSE:
            if (job.arg[0]) { // click
                uint8_t held = mouse_btn;
                mouse_btn = held | (uint8_t)job.arg[0];
                mouse_flush();
                vTaskDelay(pdMS_TO_TICKS(25));
                mouse_btn = held;
            }
            mouse_flush();
            break;
        case HID_MEDIA: {
            uint16_t u = keymap_media(job.arg);
            if (u) consumer(u);
            else ok = false;
            break;
        }
        }
        if (job.ack[0]) {
            char buf[64];
            snprintf(buf, sizeof(buf), "{\"t\":\"ack\",\"id\":\"%s\",\"ok\":%s}", job.ack, ok ? "true" : "false");
            usb_send_line(buf);
        }
    }
}

void usb_hid_hold(const char *keys, int hold_ms, const char *ack_id)
{
    hid_job_t job = {.kind = HID_KEYS, .hold = (uint16_t)(hold_ms < 1 ? 1 : hold_ms > 2000 ? 2000 : hold_ms)};
    strlcpy(job.arg, keys ? keys : "", sizeof(job.arg));
    strlcpy(job.ack, ack_id ? ack_id : "", sizeof(job.ack));
    xQueueSend(hid_q, &job, pdMS_TO_TICKS(50));
}

void usb_hid(hid_kind_t kind, const char *arg, const char *ack_id)
{
    hid_job_t job = {.kind = kind};
    strlcpy(job.arg, arg ? arg : "", sizeof(job.arg));
    strlcpy(job.ack, ack_id ? ack_id : "", sizeof(job.ack));
    xQueueSend(hid_q, &job, pdMS_TO_TICKS(50));
}

// ---- bootloader ---------------------------------------------------------------

void usb_reboot_to_bootloader(void)
{
    ESP_LOGI(TAG, "rebooting into download mode");
    // Drop off the bus first and give the host time to notice, otherwise it
    // never sees the ROM's download port enumerate after the reset.
    tud_disconnect();
    vTaskDelay(pdMS_TO_TICKS(400));
    REG_WRITE(RTC_CNTL_OPTION1_REG, RTC_CNTL_FORCE_DOWNLOAD_BOOT);
    esp_rom_software_reset_system();
}

// ---- init -----------------------------------------------------------------------

void usb_init(QueueHandle_t lines)
{
    line_q = lines;
    line_buf = heap_caps_malloc(RX_LINE_MAX, MALLOC_CAP_SPIRAM);
    tx_mu = xSemaphoreCreateMutex();
    hid_q = xQueueCreate(16, sizeof(hid_job_t));

    uint8_t mac[6];
    esp_read_mac(mac, ESP_MAC_WIFI_STA);
    snprintf(serial, sizeof(serial), "%02X%02X%02X%02X%02X%02X", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5]);

    const tinyusb_config_t cfg = {
        .device_descriptor = &dev_desc,
        .string_descriptor = strings,
        .string_descriptor_count = sizeof(strings) / sizeof(strings[0]),
        .external_phy = false,
        .configuration_descriptor = cfg_desc,
    };
    ESP_ERROR_CHECK(tinyusb_driver_install(&cfg));

    const tinyusb_config_cdcacm_t acm = {
        .usb_dev = TINYUSB_USBDEV_0,
        .cdc_port = TINYUSB_CDC_ACM_0,
        .rx_unread_buf_sz = 64,
        .callback_rx = cdc_rx,
        .callback_line_state_changed = cdc_line_state,
        .callback_line_coding_changed = cdc_line_coding,
    };
    ESP_ERROR_CHECK(tusb_cdc_acm_init(&acm));

    xTaskCreatePinnedToCore(hid_task, "hid", 4096, NULL, 5, NULL, 0);
}
