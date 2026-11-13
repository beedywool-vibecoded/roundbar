#!/bin/sh
# Install roundbar (daemon, configurator, GNOME extension) for the current user,
# without any developer tools:
#
#   curl -fsSL https://raw.githubusercontent.com/beedywool-vibecoded/roundbar/master/install.sh | sh
#
# It downloads the latest release binary to ~/.local/bin, checks its SHA-256,
# offers to install the udev rule (needs your password once) and runs
# `roundbar install` (systemd user service, app launcher, GNOME extension).
#
# Options (after `sh -s --` when piped):
#   --uninstall        remove roundbar again (settings in ~/.config/roundbar stay)
#   --version vX.Y.Z   install that release instead of the latest
#   --no-udev          don't offer the udev rule
#   --dry-run          download and verify, then only print what would be done
set -eu

REPO="beedywool-vibecoded/roundbar"
RELEASES="${ROUNDBAR_RELEASES:-https://github.com/$REPO/releases}"
BIN_DIR="${ROUNDBAR_BIN_DIR:-$HOME/.local/bin}"
WEB_INSTALLER="https://beedywool-vibecoded.github.io/roundbar/"
VERSION="latest"
DRY_RUN=0
UNINSTALL=0
UDEV=1

say() { printf '%s\n' "$*"; }
die() {
	printf 'roundbar install: %s\n' "$*" >&2
	exit 1
}
run() {
	if [ "$DRY_RUN" = 1 ]; then
		say "would run: $*"
	else
		"$@"
	fi
}

while [ $# -gt 0 ]; do
	case "$1" in
	--uninstall) UNINSTALL=1 ;;
	--version)
		[ $# -ge 2 ] || die "--version needs a value, e.g. --version v0.2.0"
		VERSION="$2"
		shift
		;;
	--no-udev) UDEV=0 ;;
	--dry-run) DRY_RUN=1 ;;
	-h | --help)
		cat <<'EOF'
usage: install.sh [--uninstall] [--version vX.Y.Z] [--no-udev] [--dry-run]
  (piped: curl -fsSL .../install.sh | sh -s -- --uninstall)
EOF
		exit 0
		;;
	*) die "unknown option $1 (try --help)" ;;
	esac
	shift
done

# ask QUESTION: yes/no on the terminal, even when this script comes from a pipe.
# Returns 1 for "no" and 2 when there is no terminal to ask on.
ask() {
	(: <>/dev/tty) 2>/dev/null || return 2
	printf '%s [Y/n] ' "$1" >/dev/tty
	answer=""
	read -r answer </dev/tty || answer=n
	case "$answer" in
	"" | [Yy] | [Yy][Ee][Ss]) return 0 ;;
	*) return 1 ;;
	esac
}

udev_rule_installed() {
	for f in /etc/udev/rules.d/70-roundbar.rules /usr/lib/udev/rules.d/70-roundbar.rules /lib/udev/rules.d/70-roundbar.rules; do
		[ -e "$f" ] && return 0
	done
	return 1
}

[ "$(uname -s)" = Linux ] || die "roundbar runs on Linux only"

# ---- uninstall -------------------------------------------------------------
if [ "$UNINSTALL" = 1 ]; then
	bin="$BIN_DIR/roundbar"
	[ -x "$bin" ] || bin="$(command -v roundbar 2>/dev/null || true)"
	if [ -n "$bin" ] && [ -x "$bin" ]; then
		run "$bin" uninstall
	else
		say "roundbar binary not found; removing the user service and launcher files"
		run systemctl --user disable --now roundbar.service || true
		run rm -rf "$HOME/.config/systemd/user/roundbar.service" \
			"$HOME/.local/share/applications/roundbar.desktop" \
			"$HOME/.local/share/gnome-shell/extensions/roundbar@beedy"
		run systemctl --user daemon-reload || true
	fi
	exit 0
fi

# ---- download and verify ---------------------------------------------------
case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) die "no release binary for $(uname -m) (amd64 and arm64 only); build from source instead" ;;
esac

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL --retry 3 -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -q -O "$2" "$1"; }
else
	die "needs curl or wget"
fi
if command -v sha256sum >/dev/null 2>&1; then
	sha256() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
	sha256() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
	die "needs sha256sum to verify the download"
fi

if [ "$VERSION" = latest ]; then
	base="$RELEASES/latest/download"
else
	base="$RELEASES/download/$VERSION"
fi
name="roundbar-linux-$arch"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

say "downloading $name ($VERSION release)…"
fetch "$base/$name" "$tmp/$name" || die "download failed: $base/$name"
fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS" || die "download failed: $base/SHA256SUMS"
want="$(awk -v f="$name" '$2 == f || $2 == "*" f { print $1 }' "$tmp/SHA256SUMS")"
[ -n "$want" ] || die "$name is not listed in SHA256SUMS"
got="$(sha256 "$tmp/$name")"
[ "$want" = "$got" ] || die "checksum mismatch for $name (expected $want, got $got); not installing"
say "checksum OK"
chmod 755 "$tmp/$name"

# ---- install ---------------------------------------------------------------
run mkdir -p "$BIN_DIR"
# A running daemon keeps its old binary; the move replaces the file safely.
run mv -f "$tmp/$name" "$BIN_DIR/roundbar"
say "installed $BIN_DIR/roundbar"

if [ "$UDEV" = 1 ] && ! udev_rule_installed; then
	cat <<'EOF'

roundbar talks to the board over a USB serial port (/dev/ttyACM*). Linux only
lets root open it until a udev rule grants access to whoever is logged in at
the desktop. Installing that rule (/etc/udev/rules.d/70-roundbar.rules) needs
your password once (sudo).
EOF
	set +e
	ask "Install the udev rule now?"
	rc=$?
	set -e
	case $rc in
	0) run "$BIN_DIR/roundbar" udev --yes || say "the udev rule wasn't installed; run 'roundbar udev' later" ;;
	1) say "skipped. Run 'roundbar udev' later if the board shows 'No access to the device'." ;;
	*) say "no terminal to ask on: run 'roundbar udev' afterwards to install the udev rule." ;;
	esac
fi

say ""
run "$BIN_DIR/roundbar" install --no-udev

# ---- does this PC need a log out / restart? --------------------------------
# Decided from what the running daemon reports, not guessed from the distro:
# GNOME only loads a newly installed extension at the next login (Wayland and
# X11 alike); KDE, wlroots compositors and X11 desktops need nothing. The udev
# rule applies immediately (uaccess + udevadm trigger), so it never needs one.
relogin=""
cmd="roundbar"
if [ "$DRY_RUN" = 0 ]; then
	status=""
	i=0
	while [ $i -lt 10 ]; do # the daemon just (re)started: give it a few seconds
		status="$("$BIN_DIR/roundbar" status 2>/dev/null | tr -d ' \n' || true)"
		case "$status" in *'"extension":true'*) break ;; esac
		sleep 0.5
		i=$((i + 1))
	done
	case "$status" in
	*'"backend":"gnome"'*'"extension":false'* | *'"extension":false'*'"backend":"gnome"'*)
		relogin="GNOME loads Roundbar's extension (it tells Roundbar which app you're using) only at your next login." ;;
	esac
fi
case ":$PATH:" in
*":$BIN_DIR:"*) ;;
*)
	if grep -qs '\.local/bin' "$HOME/.profile" "$HOME/.bash_profile" "$HOME/.zprofile"; then
		# Debian, Ubuntu, Fedora…: ~/.profile adds ~/.local/bin at login if it exists.
		cmd="$BIN_DIR/roundbar"
		relogin="${relogin:+$relogin
  }The short 'roundbar' command works in terminals after that login."
	else
		cmd="$BIN_DIR/roundbar"
		say ""
		say "note: $BIN_DIR is not in your PATH. To type just 'roundbar', add this line to ~/.profile:"
		say "  export PATH=\"$BIN_DIR:\$PATH\""
	fi
	;;
esac

cat <<EOF

Next steps
  1. Plug the board in over USB.
  2. Board still on its factory firmware? Flash roundbar from your browser
     (Chrome or Edge): $WEB_INSTALLER
     Later updates install from the configurator (Device page, Firmware card).
  3. Open the configurator: "Roundbar" in your app menu, or run: $cmd config

Uninstall: $cmd uninstall   (or this script with --uninstall)
EOF

if [ -n "$relogin" ]; then
	cat <<EOF

┌──────────────────────────────────────────────────────────────────────┐
│  ⚠  Log out and back in (or restart) to finish installing.           │
└──────────────────────────────────────────────────────────────────────┘
  $relogin
  Until then the board, its shortcuts and the configurator already work.
EOF
elif [ "$DRY_RUN" = 0 ]; then
	say ""
	say "✓ No restart needed."
fi
case "${status:-}" in
*'"error":"permission"'*)
	say ""
	say "⚠  The board is plugged in but Roundbar can't open it yet: run '$cmd udev' (asks your password once)."
	;;
esac
