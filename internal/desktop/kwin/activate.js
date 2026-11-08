// One-shot: bring the window with the given internal id forward.
(function () {
    const id = "ROUNDBAR_ID";
    const P6 = typeof workspace.windowList === "function";
    for (const w of P6 ? workspace.windowList() : workspace.clientList()) {
        if (String(w.internalId || w.windowId) !== id)
            continue;
        w.minimized = false;
        if (P6)
            workspace.activeWindow = w;
        else
            workspace.activeClient = w;
    }
})();
