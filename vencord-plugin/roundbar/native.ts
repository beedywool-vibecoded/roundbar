/*
 * Main-process half of the Roundbar plugin: forwards the voice state to the
 * roundbar daemon over its unix socket ($XDG_RUNTIME_DIR/roundbar.sock).
 */

import { IpcMainInvokeEvent } from "electron";
import { createConnection } from "net";
import { join } from "path";

const socket = join(process.env.XDG_RUNTIME_DIR ?? "/tmp", "roundbar.sock");

export function send(_: IpcMainInvokeEvent, json: string) {
    return new Promise<void>(resolve => {
        const c = createConnection(socket, () => {
            c.write(JSON.stringify({ op: "discordState", action: JSON.parse(json) }) + "\n");
        });
        c.setTimeout(2000, () => c.destroy());
        c.on("data", () => c.end()); // daemon replied
        c.on("error", () => resolve()); // daemon not running: fine
        c.on("close", () => resolve());
    });
}
