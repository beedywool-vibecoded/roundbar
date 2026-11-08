# Roundbar: step-by-step guide

This guide takes you from a bare board to a working Roundbar, without developer tools. It takes about
10 minutes. If something doesn't work, jump to [Troubleshooting](#troubleshooting).

## What you need

- A **Guition JC3636W518C** board: ESP32-S3, 1.8" round 360×360 screen. Get the version **with touch**
  (it's usually sold as "JC3636W518C" or "JC3636W518C_I_Y"; other sizes or chips won't work).
- A **USB-C cable that carries data**. Many cheap cables only charge. If the board lights up but your
  computer never sees it, try another cable first.
- A PC running **Linux** (any common desktop) or **Windows 10/11**.
- **Chrome or Edge** for the first install (Firefox and Safari can't talk to USB serial ports). You only
  need it once, and it can be on another computer.

## 1. Put Roundbar on the board

1. Open the **[web installer](https://beedywool-vibecoded.github.io/roundbar/)** in Chrome or Edge.
2. Plug the board in.
3. Click **Install Roundbar**, pick the board in the list (it may be called "USB JTAG/serial debug unit"
   or "ESP32-S3") and confirm. The first install erases the board, then writes Roundbar: it takes a
   couple of minutes because it includes the fonts for Japanese, Chinese, Korean and emoji.
4. When it's done, the board restarts and shows Roundbar.

**The board isn't in the list?** Unplug it, **hold the BOOT button**, plug it back in, release the button,
and click *Install Roundbar* again.

You only do this once. Later firmware updates come from the PC app, in one click.

## 2. Install the PC app

### Linux

Open a terminal and run:

```sh
curl -fsSL https://raw.githubusercontent.com/beedywool-vibecoded/roundbar/master/install.sh | sh
```

The script:

- downloads the app for your PC to `~/.local/bin` and checks it against the release checksums,
- asks once for your password to install a small permission rule, so your user can talk to the board over
  USB (without it you'd get "permission denied"),
- starts Roundbar's background service, which also starts with your session from now on.

**On Arch** you can use the AUR package `roundbar-bin` instead. Every release also has `.deb` (Debian,
Ubuntu, Mint…) and `.rpm` (Fedora, openSUSE…) packages on the
[releases page](https://github.com/beedywool-vibecoded/roundbar/releases/latest).

**On GNOME**, Roundbar comes with a small GNOME Shell extension (it's the only way to know which app is
active on GNOME). GNOME only loads new extensions at login: **log out and back in once**.

### Windows

1. Download **`roundbar-setup-windows-amd64.exe`** from the
   [latest release](https://github.com/beedywool-vibecoded/roundbar/releases/latest).
2. Run it. It installs for your user only, so no admin rights are needed. Windows SmartScreen may warn
   about an unknown publisher: click *More info*, then *Run anyway*.
3. Roundbar starts and adds a tray icon (bottom right; it may hide behind the `^` arrow). It also starts
   with Windows from now on. Click the icon to open the configurator.

## 3. Follow the setup wizard

The first time the configurator opens (from your app menu, the tray icon, or `roundbar config`), a short
wizard walks you through the rest:

1. **Board**: it waits for the board and fixes the usual problems (USB permission, cable).
2. **Firmware**: it updates the board if a newer release exists.
3. **Desktop**: it tells you what works on your desktop, and whether you need to log out once (GNOME).
4. **Keyboard**: Roundbar types with your PC's keyboard layout, detected automatically. Check it's right.
5. **Apps**: it finds the apps you have installed and offers ready-made pages for them (browser, code
   editor, music, Discord, Steam games, OBS…). Keep the ones you want.
6. **Extras**: only for what it found on your PC: Steam games, OBS recording, Discord voice, MangoHud.
7. **Done**: the board shows your new pages.

You can skip any step and run the wizard again from the configurator's **Device** page.

## 4. Use it

- **Swipe** left and right to change page; pages wrap around.
- **Tap** a button for its action, **long press** for its second action (if it has one).
- **Pull down** from the top edge for quick settings: jump to a page, turn the screen off, pause the
  automatic switching.
- **Pull up** from the bottom edge for your notifications.
- Pages **change on their own** with the app you're using. In the configurator, each profile lists the
  apps it follows.
- **Your setup lives on the board.** Plug it into another PC with Roundbar installed and that PC takes your
  setup from the board. On a PC without the app, the shortcut buttons still work (the board is a USB
  keyboard), only live content like now playing or stats needs the app.

## Troubleshooting

**The board doesn't light up.** Try another USB port and another cable. The board needs a port that gives
enough power; avoid unpowered hubs.

**The PC doesn't see the board** (the configurator says it's not connected).
- Try another cable: charge-only cables are the most common cause.
- Linux: run `roundbar status`. If it mentions **permission**, run `roundbar udev` (asks for your password),
  then unplug and replug the board.
- Make sure the board runs Roundbar: if its screen is black or shows something else, redo
  [step 1](#1-put-roundbar-on-the-board).

**A firmware update fails or the board stays black after an update.** Run the update again: Roundbar only
rewrites what's missing, and the board waits in download mode until it succeeds. If the board doesn't come
back, unplug it, hold **BOOT** while plugging it in, and run the update again from the configurator (or use
the web installer).

**The pages don't follow the active app.**
- GNOME: log out and back in once after installing (GNOME loads the extension at login). Then check
  `gnome-extensions info roundbar@beedy` says it's enabled.
- Other desktops: the configurator's **Device** page shows which desktop backend Roundbar uses and what it
  can do. Some Wayland compositors don't share the active window with other programs; there Roundbar shows
  your default profile.
- The app you're using needs a profile that matches it: open the profile in the configurator and check its
  matched apps (the configurator suggests the app currently in focus).

**The board types the wrong characters** (for example `q` instead of `a`). The board types like a keyboard,
so it must know your layout. In the configurator, **Device → Keyboard layout**, check that *Automatic* shows
the right layout, or pick yours.

**Discord voice doesn't show up.**
- Official Discord app: set up the Discord connection in the configurator (**Device → Discord**).
- Vesktop on Linux: turn on live voice in the configurator, then restart Vesktop. Note that while it's on,
  programs running as you on this PC could read your Discord session through a local-only port.
- Vesktop on Windows: right-click the Vesktop shortcut → *Properties*, and add ` --remote-debugging-port=9337`
  at the end of *Target* (after the closing quote), then start Vesktop from that shortcut. Same note as above.

**OBS page says it isn't connected.** In OBS, open **Tools → WebSocket Server Settings**, enable the server,
copy the password, and paste it in the configurator (**Device → OBS**). OBS must be running.

**MangoHud page says "Start a game with MangoHud".** The game must run with MangoHud (for example
`mangohud %command%` in its Steam launch options). Restart games that were already running when you added
the page.

**Notifications don't appear on Windows.** Windows doesn't let regular apps read other apps' notifications,
so the notification panel isn't available on Windows.

**Something else.** Run `roundbar status` (Linux, or from the tray icon's menu on Windows) and open an issue
on [GitHub](https://github.com/beedywool-vibecoded/roundbar/issues) with what it says.

## Uninstall

- **Linux:** `roundbar uninstall` removes the app, the service and the desktop integration, and offers to
  remove the USB permission rule. Your settings stay in `~/.config/roundbar`.
- **Windows:** uninstall *Roundbar* from **Settings → Apps**. Your settings stay in `%APPDATA%\roundbar`.

The board keeps its firmware and your setup; it still works as a shortcut keyboard on any computer.
