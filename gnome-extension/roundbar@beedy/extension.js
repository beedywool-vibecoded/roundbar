// Publishes the focused app on the session bus as dev.roundbar.Focus so the
// roundbar daemon can switch profiles. Wayland gives other programs no way to
// see the active window, and org.gnome.Shell.Introspect is locked down.
import Gio from 'gi://Gio';
import GLib from 'gi://GLib';
import Shell from 'gi://Shell';
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
  </interface>
</node>`;

export default class RoundbarFocus extends Extension {
    enable() {
        this._last = null;
        this._dbus = Gio.DBusExportedObject.wrapJSObject(IFACE, this);
        this._dbus.export(Gio.DBus.session, '/dev/roundbar/Focus');
        this._name = Gio.bus_own_name_on_connection(Gio.DBus.session, 'dev.roundbar.Focus',
            Gio.BusNameOwnerFlags.NONE, null, null);
        this._sig = global.display.connect('notify::focus-window', () => this._emit());
    }

    disable() {
        global.display.disconnect(this._sig);
        this._dbus.unexport();
        this._dbus = null;
        Gio.bus_unown_name(this._name);
    }

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

    _emit() {
        const info = this._info();
        const key = info.join('\n');
        if (key === this._last)
            return;
        this._last = key;
        this._dbus.emit_signal('Changed', new GLib.Variant('(sss)', info));
    }
}
