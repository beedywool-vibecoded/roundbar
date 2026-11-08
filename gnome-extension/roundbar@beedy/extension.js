// Roundbar's window into GNOME Shell, on the session bus as dev.roundbar.Focus.
//
// - focus: which app is focused (Wayland gives other programs no way to know)
// - windows: list / activate open windows
// - notifications and clipboard history: content is only handed out through
//   method calls; the signals carry no data, so nothing private is broadcast
//   on the bus. Clipboard entries marked secret by password managers are skipped.
import Gio from 'gi://Gio';
import GLib from 'gi://GLib';
import Meta from 'gi://Meta';
import Shell from 'gi://Shell';
import St from 'gi://St';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';

const IFACE = `<node>
  <interface name="dev.roundbar.Focus">
    <method name="Get">
      <arg type="s" direction="out" name="id"/>
      <arg type="s" direction="out" name="name"/>
      <arg type="s" direction="out" name="class"/>
    </method>
    <signal name="Changed">
      <arg type="s" name="id"/>
      <arg type="s" name="name"/>
      <arg type="s" name="class"/>
    </signal>
    <method name="Windows">
      <arg type="a(ssss)" direction="out" name="windows"/>
    </method>
    <method name="Activate">
      <arg type="s" direction="in" name="id"/>
    </method>
    <signal name="WindowsChanged"/>
    <method name="Notifications">
      <arg type="a(sssss)" direction="out" name="list"/>
    </method>
    <method name="NotificationAction">
      <arg type="s" direction="in" name="id"/>
      <arg type="s" direction="in" name="action"/>
    </method>
    <signal name="NotificationsChanged">
      <arg type="s" name="newId"/>
    </signal>
    <method name="ClipboardHistory">
      <arg type="as" direction="out" name="texts"/>
    </method>
    <method name="SetClipboard">
      <arg type="s" direction="in" name="text"/>
    </method>
    <signal name="ClipboardChanged"/>
  </interface>
</node>`;

const MAX_NOTIFS = 20;
const MAX_CLIPS = 10;
const SECRET_HINT = 'x-kde-passwordManagerHint';

function iconName(app, gicon) {
    const icon = gicon ?? app?.get_icon?.();
    if (!icon)
        return '';
    if (icon instanceof Gio.ThemedIcon)
        return icon.get_names()[0] ?? '';
    if (icon instanceof Gio.FileIcon)
        return icon.get_file().get_path() ?? '';
    return icon.to_string?.() ?? '';
}

export default class RoundbarFocus extends Extension {
    enable() {
        this._last = null;
        this._notifs = new Map(); // id -> {n, app, icon}
        this._notifSeq = 0;
        this._clips = [];
        this._sigs = [];
        this._winTimer = 0;

        this._dbus = Gio.DBusExportedObject.wrapJSObject(IFACE, this);
        this._dbus.export(Gio.DBus.session, '/dev/roundbar/Focus');
        this._name = Gio.bus_own_name_on_connection(Gio.DBus.session, 'dev.roundbar.Focus',
            Gio.BusNameOwnerFlags.NONE, null, null);

        const on = (obj, sig, fn) => this._sigs.push([obj, obj.connect(sig, fn)]);
        on(global.display, 'notify::focus-window', () => {
            this._emitFocus();
            this._windowsChanged();
        });
        on(global.display, 'window-created', () => this._windowsChanged());
        on(global.window_manager, 'destroy', () => this._windowsChanged());

        // notifications
        on(Main.messageTray, 'source-added', (_tray, source) => this._watchSource(source));
        for (const source of Main.messageTray.getSources())
            this._watchSource(source);

        // clipboard
        const selection = global.display.get_selection();
        on(selection, 'owner-changed', (sel, type) => {
            if (type === Meta.SelectionType.SELECTION_CLIPBOARD)
                this._readClipboard(sel);
        });
    }

    disable() {
        for (const [obj, id] of this._sigs)
            obj.disconnect(id);
        this._sigs = [];
        for (const {sourceSig, source} of this._sourceSigs ?? [])
            source.disconnect(sourceSig);
        this._sourceSigs = [];
        if (this._winTimer)
            GLib.source_remove(this._winTimer);
        this._winTimer = 0;
        this._dbus.unexport();
        this._dbus = null;
        Gio.bus_unown_name(this._name);
        this._notifs = null;
        this._clips = null;
    }

    // ---- focus ----

    _info() {
        const w = global.display.focus_window;
        if (!w)
            return ['', '', ''];
        const app = Shell.WindowTracker.get_default().get_window_app(w);
        let id = app?.get_id() ?? '';
        id = id.startsWith('window:') ? '' : id.replace(/\.desktop$/, '').toLowerCase();
        const name = app?.get_name() ?? w.get_title() ?? '';
        const cls = (w.get_wm_class() ?? '').toLowerCase();
        return [id, name, cls];
    }

    Get() {
        return this._info();
    }

    _emitFocus() {
        const info = this._info();
        const key = info.join('\n');
        if (key === this._last)
            return;
        this._last = key;
        this._dbus.emit_signal('Changed', new GLib.Variant('(sss)', info));
    }

    // ---- windows ----

    _windowsChanged() {
        if (this._winTimer)
            return;
        this._winTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 250, () => {
            this._winTimer = 0;
            this._dbus?.emit_signal('WindowsChanged', null);
            return GLib.SOURCE_REMOVE;
        });
    }

    _normalWindows() {
        return global.get_window_actors()
            .map(a => a.meta_window)
            .filter(w => w && w.get_window_type() === Meta.WindowType.NORMAL && !w.is_skip_taskbar())
            .sort((a, b) => b.get_user_time() - a.get_user_time());
    }

    Windows() {
        const tracker = Shell.WindowTracker.get_default();
        return this._normalWindows().map(w => {
            const app = tracker.get_window_app(w);
            const id = (app?.get_id() ?? '').replace(/\.desktop$/, '');
            return [String(w.get_id()), id, w.get_title() ?? '', iconName(app)];
        });
    }

    Activate(id) {
        const w = this._normalWindows().find(x => String(x.get_id()) === id);
        if (w)
            Main.activateWindow(w);
    }

    // ---- notifications ----

    _watchSource(source) {
        this._sourceSigs ??= [];
        const sig = source.connect('notification-added', (src, n) => this._addNotification(src, n));
        this._sourceSigs.push({source, sourceSig: sig});
    }

    _addNotification(source, n) {
        const id = String(++this._notifSeq);
        this._notifs.set(id, {n, app: source.title ?? '', icon: iconName(source.app ?? null, n.gicon ?? source.icon)});
        n.connect('destroy', () => {
            if (this._notifs?.delete(id))
                this._dbus?.emit_signal('NotificationsChanged', new GLib.Variant('(s)', ['']));
        });
        while (this._notifs.size > MAX_NOTIFS)
            this._notifs.delete(this._notifs.keys().next().value);
        this._dbus.emit_signal('NotificationsChanged', new GLib.Variant('(s)', [id]));
    }

    Notifications() {
        return [...this._notifs.entries()].reverse().map(([id, {n, app, icon}]) =>
            [id, app, n.title ?? '', n.body ?? n.bannerBodyText ?? '', icon]);
    }

    NotificationAction(id, action) {
        const entry = this._notifs.get(id);
        if (!entry)
            return;
        if (action === 'open')
            entry.n.activate();
        else
            entry.n.destroy();
    }

    // ---- clipboard ----

    _readClipboard(selection) {
        const types = selection.get_mimetypes(Meta.SelectionType.SELECTION_CLIPBOARD) ?? [];
        if (types.includes(SECRET_HINT))
            return; // password managers mark secrets: never keep them
        St.Clipboard.get_default().get_text(St.ClipboardType.CLIPBOARD, (_cb, text) => {
            if (!text || !this._clips)
                return;
            text = text.length > 2000 ? text.slice(0, 2000) : text;
            this._clips = [text, ...this._clips.filter(t => t !== text)].slice(0, MAX_CLIPS);
            this._dbus?.emit_signal('ClipboardChanged', null);
        });
    }

    ClipboardHistory() {
        return this._clips;
    }

    SetClipboard(text) {
        St.Clipboard.get_default().set_text(St.ClipboardType.CLIPBOARD, text);
    }
}
