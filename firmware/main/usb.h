#pragma once
#include <stdbool.h>
#include <stdint.h>

#include "freertos/FreeRTOS.h"
#include "freertos/queue.h"

// Composite USB device: CDC (JSON lines to the daemon) + HID keyboard/consumer.
// Complete incoming lines (heap strings, free() them) are posted to `lines`.
void usb_init(QueueHandle_t lines);
bool usb_host_listening(void); // DTR is up: the daemon has the port open
bool usb_suspended(void);      // host suspended the bus (PC asleep) or not mounted
void usb_send_line(const char *json); // appends '\n'

typedef enum { HID_KEYS, HID_TEXT, HID_MEDIA, HID_MOUSE } hid_kind_t;
// Queue a keyboard job. `ack_id` (may be NULL) gets an {"t":"ack"} once typed.
void usb_hid(hid_kind_t kind, const char *arg, const char *ack_id);

// Mouse (touchpad pages). Movement and wheel accumulate and are sent in batches;
// buttons: bit0 left, bit1 right.
void usb_mouse_move(int dx, int dy);
void usb_mouse_wheel(int ticks);
void usb_mouse_buttons(uint8_t buttons); // set the held buttons
void usb_mouse_click(uint8_t button);    // press + release

// Reboot into the ROM USB download mode so esptool can flash.
void usb_reboot_to_bootloader(void);
