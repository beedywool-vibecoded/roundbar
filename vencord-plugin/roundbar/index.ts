/*
 * Roundbar: shows your Discord voice channel on the Roundbar controller.
 * Sends the channel, its members, who is talking and your mute/deafen state to
 * the local roundbar daemon (unix socket). Nothing leaves your machine.
 */

import definePlugin, { PluginNative } from "@utils/types";
import { findStoreLazy } from "@webpack";
import { ChannelStore, GuildMemberStore, SelectedChannelStore, UserStore } from "@webpack/common";

const VoiceStateStore = findStoreLazy("VoiceStateStore");
const SpeakingStore = findStoreLazy("SpeakingStore");
const MediaEngineStore = findStoreLazy("MediaEngineStore");

const Native = VencordNative.pluginHelpers.Roundbar as PluginNative<typeof import("./native")>;

const speaking = new Set<string>();
let timer: ReturnType<typeof setTimeout> | null = null;
let heartbeat: ReturnType<typeof setInterval> | null = null;

function state() {
    const me = UserStore.getCurrentUser();
    const channelId = SelectedChannelStore.getVoiceChannelId();
    const channel = channelId ? ChannelStore.getChannel(channelId) : null;
    const members = channel
        ? Object.values(VoiceStateStore.getVoiceStatesForChannel(channelId) ?? {}).map((vs: any) => {
            const user = UserStore.getUser(vs.userId);
            const nick = channel.guild_id ? GuildMemberStore.getNick(channel.guild_id, vs.userId) : null;
            return {
                id: vs.userId,
                name: nick ?? user?.globalName ?? user?.username ?? "",
                avatar: user?.getAvatarURL?.(undefined, 64, false) ?? "",
                muted: !!(vs.selfMute || vs.mute || vs.selfDeaf || vs.deaf),
                speaking: speaking.has(vs.userId) || SpeakingStore.isSpeaking?.(vs.userId) || false,
            };
        })
        : [];
    return {
        connected: true,
        source: "vencord",
        user: me?.globalName ?? me?.username ?? "",
        channel: channel ? (channel.name || "Voice") : "",
        members,
        mute: !!MediaEngineStore.isSelfMute?.(),
        deaf: !!MediaEngineStore.isSelfDeaf?.(),
    };
}

// Coalesce bursts (speaking toggles fast) into one update per ~120 ms.
function push() {
    if (timer) return;
    timer = setTimeout(() => {
        timer = null;
        Native.send(JSON.stringify(state())).catch(() => { });
    }, 120);
}

export default definePlugin({
    name: "Roundbar",
    description: "Shows your voice channel (members and who is talking) on a Roundbar controller",
    authors: [{ name: "beedy", id: 0n }],

    flux: {
        VOICE_STATE_UPDATES: push,
        VOICE_CHANNEL_SELECT: push,
        RTC_CONNECTION_STATE: push,
        AUDIO_TOGGLE_SELF_MUTE: push,
        AUDIO_TOGGLE_SELF_DEAF: push,
        SPEAKING({ userId, speakingFlags }: { userId: string; speakingFlags: number; }) {
            if (speakingFlags) speaking.add(userId);
            else speaking.delete(userId);
            push();
        },
    },

    start() {
        push();
        heartbeat = setInterval(push, 30_000); // lets the daemon know Discord is still there
    },

    stop() {
        if (heartbeat) clearInterval(heartbeat);
        heartbeat = null;
        Native.send(JSON.stringify({ connected: false, source: "vencord" })).catch(() => { });
    },
});
