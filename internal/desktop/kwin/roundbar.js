// Loaded into KWin by the roundbar daemon: reports the active window and the
// window list to dev.roundbar.Daemon on the session bus, on change only.
// Works with the Plasma 6 (windows) and Plasma 5 (clients) APIs.
const S = "dev.roundbar.Daemon", P = "/dev/roundbar/Daemon", I = "dev.roundbar.Daemon";
const P6 = typeof workspace.windowList === "function";

function str(v) {
    if (v instanceof ArrayBuffer) // Plasma 5 hands QByteArray properties over as ArrayBuffer
        return String.fromCharCode.apply(null, new Uint8Array(v));
    return v ? String(v) : "";
}

function info(w) {
    return {
        id: str(w.internalId || w.windowId),
        app: str(w.desktopFileName),
        class: str(w.resourceClass),
        name: str(w.resourceName),
        title: str(w.caption),
    };
}

// gone: a window being removed, which the list may still hold
function windows(gone) {
    const list = P6 ? workspace.windowList() : workspace.clientList();
    const out = [];
    for (const w of list)
        if (w !== gone && w.normalWindow && !w.skipTaskbar)
            out.push(info(w));
    callDBus(S, P, I, "Windows", JSON.stringify(out));
}

function focus(w) {
    if (!w)
        return;
    const i = info(w);
    callDBus(S, P, I, "Focus", i.id, i.app, i.class, i.name, i.title);
}

if (P6) {
    workspace.windowActivated.connect(focus);
    workspace.windowAdded.connect(() => windows());
    workspace.windowRemoved.connect(windows);
    focus(workspace.activeWindow);
} else {
    workspace.clientActivated.connect(focus);
    workspace.clientAdded.connect(() => windows());
    workspace.clientRemoved.connect(windows);
    focus(workspace.activeClient);
}
windows();
