package audio

import (
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/beedywool-vibecoded/roundbar/internal/icons"
	"github.com/beedywool-vibecoded/roundbar/internal/win"
	"golang.org/x/sys/windows"
)

// Core Audio: the default render and capture endpoints (IAudioEndpointVolume
// with a change callback), the default device (IMMNotificationClient), and
// the render endpoint's sessions for the mixer (IAudioSessionManager2 with
// IAudioSessionNotification, IAudioSessionEvents per session). Windows calls
// us on its own threads; the callbacks only kick channels.

var (
	clsidMMDeviceEnumerator = win.IID("bcde0395-e52f-467c-8e3d-c4579291692e")
	iidIMMDeviceEnumerator  = win.IID("a95664d2-9614-4f35-a746-de8db63617e6")
	iidIAudioEndpointVolume = win.IID("5cdf2c82-841e-4546-9722-0cf74078229a")
	iidEndpointVolumeCB     = win.IID("657804fa-d6ad-4496-8a60-352752af4f89")
	iidIMMNotificationCli   = win.IID("7991eec9-7e89-4d85-8390-6c703cec60c0")
	iidIAudioSessionMgr2    = win.IID("77aa99a0-1bd6-484f-8bc7-2c654c9a9b6f")
	iidIAudioSessionCtl2    = win.IID("bfb7ff88-7239-4fc9-8fa2-07c950be9c6d")
	iidISimpleAudioVolume   = win.IID("87ce5498-68d6-44e5-9215-6da47ef883d8")
	iidSessionNotification  = win.IID("641dd20b-4d41-49cc-aba3-174b9477bb08")
	iidSessionEvents        = win.IID("24918acc-64b3-37c1-8ca9-74a66e9957a8")
	// IPolicyConfig: undocumented, but how every volume mixer switches the default output.
	clsidPolicyConfig = win.IID("870af99c-171d-4f9e-af0d-e63df40c2bc9")
	iidIPolicyConfig  = win.IID("f8679f50-850a-41cf-9c72-430f290290c8")
	pkeyFriendlyName  = struct {
		fmtid windows.GUID
		pid   uint32
	}{*win.IID("a45c254e-df1c-4efd-8020-67d146a850e0"), 14}
)

const (
	eRender, eCapture = 0, 1
	eConsole          = 0
	clsctxAll         = 0x17
	deviceStateActive = 1
	sessionExpired    = 2
)

// Float arguments travel in FP registers. Go's syscall path mirrors integer
// arguments into XMM0-3 on amd64, so a float32's bits passed as an integer
// arrive intact there; arm64 has no such mirroring, so volumes are set in
// steps there and per-app volume is read-only.
var floatArgs = runtime.GOARCH == "amd64" || runtime.GOARCH == "386"

type session struct {
	id  string // short id for the device
	ctl win.Obj
	vol win.Obj
	pid uint32
}

type Audio struct {
	mu      sync.Mutex
	enum    win.Obj
	out, in win.Obj // IAudioEndpointVolume of the default render / capture device
	outID   string
	mgr     win.Obj // IAudioSessionManager2 of the default render device
	sess    map[string]*session
	nextID  int

	volCB, notifCB, sessCB, eventsCB win.Obj
	rebind, rescan                   chan struct{}
	closed                           chan struct{}
	// Changed receives a value (coalesced) whenever the default sink/source changes.
	Changed chan struct{}
	// Streams: playback streams or outputs changed (mixer pages).
	Streams chan struct{}
}

func kick(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func Connect() (*Audio, error) {
	enum, err := win.CreateInstance(clsidMMDeviceEnumerator, iidIMMDeviceEnumerator)
	if err != nil {
		return nil, fmt.Errorf("core audio: %w", err)
	}
	a := &Audio{enum: enum, sess: map[string]*session{}, rebind: make(chan struct{}, 1), rescan: make(chan struct{}, 1),
		closed: make(chan struct{}), Changed: make(chan struct{}, 1), Streams: make(chan struct{}, 1)}
	ok := func([]uintptr) uintptr { return 0 }
	a.volCB = win.NewObject([]*windows.GUID{iidEndpointVolumeCB}, win.Method{Args: 1, Fn: func([]uintptr) uintptr { kick(a.Changed); return 0 }})
	a.notifCB = win.NewObject([]*windows.GUID{iidIMMNotificationCli},
		win.Method{Args: 2, Fn: func([]uintptr) uintptr { kick(a.rebind); return 0 }}, // OnDeviceStateChanged
		win.Method{Args: 1, Fn: ok}, // OnDeviceAdded
		win.Method{Args: 1, Fn: ok}, // OnDeviceRemoved
		win.Method{Args: 3, Fn: func(args []uintptr) uintptr { // OnDefaultDeviceChanged(flow, role, id)
			if args[1] == eConsole {
				kick(a.rebind)
			}
			return 0
		}},
		win.Method{Args: 2, Fn: ok}, // OnPropertyValueChanged
	)
	a.sessCB = win.NewObject([]*windows.GUID{iidSessionNotification}, win.Method{Args: 1, Fn: func([]uintptr) uintptr { kick(a.rescan); return 0 }})
	streams := func([]uintptr) uintptr { kick(a.Streams); return 0 }
	gone := func([]uintptr) uintptr { kick(a.rescan); return 0 }
	a.eventsCB = win.NewObject([]*windows.GUID{iidSessionEvents},
		win.Method{Args: 2, Fn: streams}, // OnDisplayNameChanged
		win.Method{Args: 2, Fn: ok},      // OnIconPathChanged
		win.Method{Args: 3, Fn: streams}, // OnSimpleVolumeChanged
		win.Method{Args: 4, Fn: ok},      // OnChannelVolumeChanged
		win.Method{Args: 2, Fn: ok},      // OnGroupingParamChanged
		win.Method{Args: 1, Fn: gone},    // OnStateChanged
		win.Method{Args: 1, Fn: gone},    // OnSessionDisconnected
	)
	if err := enum.CallErr(6, uintptr(a.notifCB)); err != nil { // RegisterEndpointNotificationCallback
		enum.Release()
		return nil, err
	}
	a.mu.Lock()
	a.bindLocked()
	a.mu.Unlock()
	go a.loop()
	return a, nil
}

// loop does the work callbacks can't: re-binding to a new default device and
// registering for events on new sessions.
func (a *Audio) loop() {
	for {
		select {
		case <-a.rebind:
			a.mu.Lock()
			a.bindLocked()
			a.mu.Unlock()
			kick(a.Changed)
			kick(a.Streams)
		case <-a.rescan:
			a.mu.Lock()
			a.scanLocked()
			a.mu.Unlock()
			kick(a.Streams)
		}
	}
}

// Closed is closed when the audio service goes away for good (it doesn't on Windows).
func (a *Audio) Closed() <-chan struct{} { return a.closed }

func (a *Audio) defaultDevice(flow uintptr) (win.Obj, error) {
	var dev win.Obj
	if err := a.enum.CallErr(4, flow, eConsole, uintptr(unsafe.Pointer(&dev))); err != nil {
		return 0, err
	}
	return dev, nil
}

func activate(dev win.Obj, iid *windows.GUID) (win.Obj, error) {
	var out win.Obj
	if err := dev.CallErr(3, uintptr(unsafe.Pointer(iid)), clsctxAll, 0, uintptr(unsafe.Pointer(&out))); err != nil {
		return 0, err
	}
	return out, nil
}

func deviceID(dev win.Obj) string {
	var p uintptr
	if dev.CallErr(5, uintptr(unsafe.Pointer(&p))) != nil {
		return ""
	}
	defer win.TaskMemFree(p)
	return win.UTF16At(p)
}

// bindLocked (re)attaches to the current default devices.
func (a *Audio) bindLocked() {
	for _, ep := range []*win.Obj{&a.out, &a.in} {
		if *ep != 0 {
			_ = ep.CallErr(4, uintptr(a.volCB)) // UnregisterControlChangeNotify
			ep.Release()
			*ep = 0
		}
	}
	a.dropSessionsLocked()
	if a.mgr != 0 {
		_ = a.mgr.CallErr(7, uintptr(a.sessCB)) // UnregisterSessionNotification
		a.mgr.Release()
		a.mgr = 0
	}
	a.outID = ""
	if dev, err := a.defaultDevice(eRender); err == nil {
		a.outID = deviceID(dev)
		if ep, err := activate(dev, iidIAudioEndpointVolume); err == nil {
			a.out = ep
			_ = ep.CallErr(3, uintptr(a.volCB)) // RegisterControlChangeNotify
		}
		if m, err := activate(dev, iidIAudioSessionMgr2); err == nil {
			a.mgr = m
			a.scanLocked()                      // notifications only start after a first enumeration
			_ = m.CallErr(6, uintptr(a.sessCB)) // RegisterSessionNotification
		}
		dev.Release()
	}
	if dev, err := a.defaultDevice(eCapture); err == nil {
		if ep, err := activate(dev, iidIAudioEndpointVolume); err == nil {
			a.in = ep
			_ = ep.CallErr(3, uintptr(a.volCB))
		}
		dev.Release()
	}
}

func (a *Audio) dropSessionsLocked() {
	for k, s := range a.sess {
		_ = s.ctl.CallErr(11, uintptr(a.eventsCB)) // UnregisterAudioSessionNotification
		s.vol.Release()
		s.ctl.Release()
		delete(a.sess, k)
	}
}

// scanLocked syncs a.sess with the device's live sessions (other apps', not
// the system sounds), registering for their events.
func (a *Audio) scanLocked() {
	if a.mgr == 0 {
		return
	}
	enum, err := a.mgr.Get(5) // GetSessionEnumerator
	if err != nil {
		return
	}
	defer enum.Release()
	var n int32
	_ = enum.CallErr(3, uintptr(unsafe.Pointer(&n)))
	self := uint32(os.Getpid())
	seen := map[string]bool{}
	for i := int32(0); i < n; i++ {
		var c win.Obj
		if enum.CallErr(4, uintptr(i), uintptr(unsafe.Pointer(&c))) != nil || c == 0 {
			continue
		}
		c2, err := c.QueryInterface(iidIAudioSessionCtl2)
		c.Release()
		if err != nil {
			continue
		}
		var state, pid uint32
		_ = c2.CallErr(3, uintptr(unsafe.Pointer(&state)))
		_ = c2.CallErr(14, uintptr(unsafe.Pointer(&pid)))
		system := c2.Call(15) == 0 // IsSystemSoundsSession: S_OK means yes
		key := sessionKey(c2)
		if state == sessionExpired || system || pid == 0 || pid == self || key == "" {
			c2.Release()
			continue
		}
		seen[key] = true
		if _, ok := a.sess[key]; ok {
			c2.Release()
			continue
		}
		vol, err := c2.QueryInterface(iidISimpleAudioVolume)
		if err != nil {
			c2.Release()
			continue
		}
		_ = c2.CallErr(10, uintptr(a.eventsCB)) // RegisterAudioSessionNotification
		a.nextID++
		a.sess[key] = &session{id: fmt.Sprint(a.nextID), ctl: c2, vol: vol, pid: pid}
	}
	for k, s := range a.sess {
		if !seen[k] {
			_ = s.ctl.CallErr(11, uintptr(a.eventsCB))
			s.vol.Release()
			s.ctl.Release()
			delete(a.sess, k)
		}
	}
}

func sessionKey(c2 win.Obj) string {
	var p uintptr
	if c2.CallErr(13, uintptr(unsafe.Pointer(&p))) != nil { // GetSessionInstanceIdentifier
		return ""
	}
	defer win.TaskMemFree(p)
	return win.UTF16At(p)
}

func scalar(ep win.Obj) (int, bool) {
	var v float32
	var mute int32
	if ep == 0 || ep.CallErr(9, uintptr(unsafe.Pointer(&v))) != nil { // GetMasterVolumeLevelScalar
		return 0, false
	}
	_ = ep.CallErr(15, uintptr(unsafe.Pointer(&mute)))
	return int(math.Round(float64(v) * 100)), mute != 0
}

func (a *Audio) State() (State, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.out == 0 && a.in == 0 {
		return State{}, errors.New("no audio device")
	}
	var s State
	s.Volume, s.Muted = scalar(a.out)
	s.Mic, s.MicMuted = scalar(a.in)
	return s, nil
}

func float32Arg(v float32) uintptr { return uintptr(math.Float32bits(v)) }

// setScalar sets an endpoint to p percent: directly where float arguments
// work, else with VolumeStepUp/Down from the current level.
func setScalar(ep win.Obj, p int) error {
	if ep == 0 {
		return errors.New("no audio device")
	}
	p = max(0, min(100, p))
	if floatArgs {
		return ep.CallErr(7, float32Arg(float32(p)/100), 0) // SetMasterVolumeLevelScalar
	}
	var step, steps uint32
	if err := ep.CallErr(16, uintptr(unsafe.Pointer(&step)), uintptr(unsafe.Pointer(&steps))); err != nil || steps < 2 { // GetVolumeStepInfo
		return errors.New("volume steps unavailable")
	}
	want := uint32(math.Round(float64(p) / 100 * float64(steps-1)))
	for i := 0; i < int(steps) && step != want; i++ {
		slot := 17 // VolumeStepUp
		if step > want {
			slot = 18 // VolumeStepDown
		}
		if err := ep.CallErr(slot, 0); err != nil {
			return err
		}
		_ = ep.CallErr(16, uintptr(unsafe.Pointer(&step)), uintptr(unsafe.Pointer(&steps)))
	}
	return nil
}

func setMute(ep win.Obj, mute bool) error {
	if ep == 0 {
		return errors.New("no audio device")
	}
	return ep.CallErr(14, boolArg(mute), 0)
}

func boolArg(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

// SetVolume sets the default output to p percent and unmutes it.
func (a *Audio) SetVolume(p int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, muted := scalar(a.out); muted && p > 0 {
		_ = setMute(a.out, false)
	}
	return setScalar(a.out, p)
}

func (a *Audio) SetMicVolume(p int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return setScalar(a.in, p)
}

// Mic applies "toggle" | "mute" | "unmute" to the default microphone.
func (a *Audio) Mic(cmd string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	mute := cmd == "mute"
	if cmd == "toggle" {
		_, m := scalar(a.in)
		mute = !m
	}
	return setMute(a.in, mute)
}

// Mixer lists the apps with an audio session on the default output, and the outputs.
func (a *Audio) Mixer() ([]App, []Output, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var apps []App
	byPID := map[uint32]bool{}
	for _, s := range a.sess {
		if byPID[s.pid] { // one row per app (browsers open several sessions)
			continue
		}
		byPID[s.pid] = true
		var v float32
		var mute int32
		_ = s.vol.CallErr(4, uintptr(unsafe.Pointer(&v)))
		_ = s.vol.CallErr(6, uintptr(unsafe.Pointer(&mute)))
		exe := win.ExePath(s.pid)
		name := sessionName(s.ctl)
		if name == "" {
			name = win.FileDescription(exe)
		}
		if name == "" {
			name = win.ExeID(exe)
		}
		icon := ""
		if exe != "" {
			icon = "exe:" + win.ExeID(exe)
			icons.Alias(icon, exe)
		}
		apps = append(apps, App{ID: s.id, Name: strings.TrimSpace(name), Icon: icon, Vol: min(100, int(math.Round(float64(v)*100))), Muted: mute != 0})
	}
	sort.Slice(apps, func(i, j int) bool { // oldest session first, stable rows
		x, _ := strconv.Atoi(apps[i].ID)
		y, _ := strconv.Atoi(apps[j].ID)
		return x < y
	})
	return apps, a.outputsLocked(), nil
}

func sessionName(c2 win.Obj) string {
	var p uintptr
	if c2.CallErr(4, uintptr(unsafe.Pointer(&p))) != nil { // GetDisplayName
		return ""
	}
	defer win.TaskMemFree(p)
	n := win.UTF16At(p)
	if strings.HasPrefix(n, "@") { // an indirect resource string, not a name
		return ""
	}
	return n
}

func (a *Audio) outputsLocked() []Output {
	var coll win.Obj
	if a.enum.CallErr(3, eRender, deviceStateActive, uintptr(unsafe.Pointer(&coll))) != nil { // EnumAudioEndpoints
		return nil
	}
	defer coll.Release()
	var n uint32
	_ = coll.CallErr(3, uintptr(unsafe.Pointer(&n)))
	var outs []Output
	for i := uint32(0); i < n; i++ {
		var dev win.Obj
		if coll.CallErr(4, uintptr(i), uintptr(unsafe.Pointer(&dev))) != nil {
			continue
		}
		id := deviceID(dev)
		outs = append(outs, Output{ID: id, Name: friendlyName(dev), Active: id == a.outID})
		dev.Release()
	}
	return outs
}

func friendlyName(dev win.Obj) string {
	var store win.Obj
	if dev.CallErr(4, 0, uintptr(unsafe.Pointer(&store))) != nil { // OpenPropertyStore(STGM_READ)
		return ""
	}
	defer store.Release()
	var pv [24]byte // PROPVARIANT
	if store.CallErr(5, uintptr(unsafe.Pointer(&pkeyFriendlyName)), uintptr(unsafe.Pointer(&pv))) != nil {
		return ""
	}
	defer win.PropVariantClear(unsafe.Pointer(&pv))
	if *(*uint16)(unsafe.Pointer(&pv[0])) != 31 { // VT_LPWSTR
		return ""
	}
	return win.UTF16At(*(*uintptr)(unsafe.Pointer(&pv[8])))
}

func (a *Audio) find(id string) *session {
	for _, s := range a.sess {
		if s.id == id {
			return s
		}
	}
	return nil
}

func (a *Audio) SetAppVolume(id string, p int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.find(id)
	if s == nil {
		return fmt.Errorf("no stream %s", id)
	}
	if !floatArgs {
		return errors.New("per-app volume isn't supported on this architecture")
	}
	v := float32Arg(float32(max(0, min(100, p))) / 100)
	for _, o := range a.sess { // every session of the app, like the mixer row shows them
		if o.pid == s.pid {
			_ = o.vol.CallErr(3, v, 0)
		}
	}
	return nil
}

func (a *Audio) ToggleAppMute(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.find(id)
	if s == nil {
		return fmt.Errorf("no stream %s", id)
	}
	var mute int32
	_ = s.vol.CallErr(6, uintptr(unsafe.Pointer(&mute)))
	for _, o := range a.sess {
		if o.pid == s.pid {
			_ = o.vol.CallErr(5, boolArg(mute == 0), 0)
		}
	}
	return nil
}

// SetOutput makes a device the default output for every role.
func (a *Audio) SetOutput(id string) error {
	pc, err := win.CreateInstance(clsidPolicyConfig, iidIPolicyConfig)
	if err != nil {
		return err
	}
	defer pc.Release()
	p, err := windows.UTF16PtrFromString(id)
	if err != nil {
		return err
	}
	for role := uintptr(0); role < 3; role++ { // console, multimedia, communications
		if err := pc.CallErr(13, uintptr(unsafe.Pointer(p)), role); err != nil { // SetDefaultEndpoint
			return err
		}
	}
	return nil
}
