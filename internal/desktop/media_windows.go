package desktop

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"sync"
	"time"
	"unsafe"

	"github.com/beedywool-vibecoded/roundbar/internal/cover"
	"github.com/beedywool-vibecoded/roundbar/internal/win"
	"golang.org/x/sys/windows"
)

// The Global System Media Transport Controls (what the volume flyout shows):
// GlobalSystemMediaTransportControlsSessionManager through WinRT. We follow
// its current session, the one media keys control, and get told about every
// change by event handlers; nothing polls. Play/pause/next/prev are the
// device's own HID media keys, which Windows sends to that same session.

const (
	classManager   = "Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager"
	iidManagerStat = "2050c4ee-11a0-57de-aed7-c97c70338245"
	iidManager     = "cace8eac-e86e-504a-ab31-5ff8ff1bce49"
	iidSession     = "7148c835-9b14-5ae2-ab85-dc9b1c14e1a8"
	piidTypedEvent = "9de1c534-6ae1-11e0-84e1-18a905bcc53f"

	playbackPlaying = 4
)

var (
	shcore                        = windows.NewLazySystemDLL("shcore.dll")
	procCreateStreamOverRandomAcc = shcore.NewProc("CreateStreamOverRandomAccessStream")
	iidIStream                    = win.IID("0000000c-0000-0000-c000-000000000046")
	iidIRandomAccessStream        = win.IID("905a0fe1-bc53-11df-8c49-001e4fc686da")
)

// typedHandler is the IID of TypedEventHandler<sender, args> for these runtime classes.
func typedHandler(senderClass, senderIID, argsClass, argsIID string) string {
	return win.ParamIID(win.SigGeneric(piidTypedEvent, win.SigClass(senderClass, senderIID), win.SigClass(argsClass, argsIID)))
}

type smtc struct {
	mgr     win.Obj
	changed func() // tells the Desktop (coalesced on its side)

	mu                sync.Mutex
	cur               win.Obj // current session, AddRef'd
	propsTok, playTok int64
	propsCB, playCB   win.Obj
	curCB, sessionsCB win.Obj
	rebind            chan struct{}
}

func newSMTC(changed func()) (*smtc, error) {
	stat, err := win.ActivationFactory(classManager, win.IID(iidManagerStat))
	if err != nil {
		return nil, err
	}
	defer stat.Release()
	op, err := stat.Get(6) // RequestAsync
	if err != nil {
		return nil, err
	}
	mgr, err := win.Await(op, 10*time.Second, true)
	if err != nil {
		return nil, err
	}
	m := &smtc{mgr: mgr, changed: changed, rebind: make(chan struct{}, 1)}
	again := func(_, _ win.Obj) {
		select {
		case m.rebind <- struct{}{}:
		default:
		}
	}
	m.curCB = win.Delegate(typedHandler(classManager, iidManager, "Windows.Media.Control.CurrentSessionChangedEventArgs", "6969cb39-0bfa-5fe0-8d73-09cc5e5408e1"), again)
	m.sessionsCB = win.Delegate(typedHandler(classManager, iidManager, "Windows.Media.Control.SessionsChangedEventArgs", "bbf0cd32-42c4-5a58-b317-f34bbfbd26e0"), again)
	notify := func(_, _ win.Obj) { m.changed() }
	const classSession = "Windows.Media.Control.GlobalSystemMediaTransportControlsSession"
	m.propsCB = win.Delegate(typedHandler(classSession, iidSession, "Windows.Media.Control.MediaPropertiesChangedEventArgs", "7d3741cb-adf0-5cef-91ba-cfabcdd77678"), notify)
	m.playCB = win.Delegate(typedHandler(classSession, iidSession, "Windows.Media.Control.PlaybackInfoChangedEventArgs", "786756c2-bc0d-50a5-8807-054291fef139"), notify)
	var tok int64
	if err := mgr.CallErr(8, uintptr(m.curCB), uintptr(unsafe.Pointer(&tok))); err != nil { // add_CurrentSessionChanged
		log.Printf("media: CurrentSessionChanged: %v", err)
	}
	if err := mgr.CallErr(10, uintptr(m.sessionsCB), uintptr(unsafe.Pointer(&tok))); err != nil { // add_SessionsChanged
		log.Printf("media: SessionsChanged: %v", err)
	}
	m.bind()
	go func() {
		for range m.rebind {
			m.bind()
			m.changed()
		}
	}()
	cover.RegisterScheme("smtc", m.openThumbnail)
	return m, nil
}

// bind follows the manager's current session (event handlers move along).
func (m *smtc) bind() {
	cur, _ := m.mgr.Get(6) // GetCurrentSession (null when nothing plays)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cur != 0 {
		_ = m.cur.CallErr(30, uintptr(m.propsTok)) // remove_MediaPropertiesChanged
		_ = m.cur.CallErr(28, uintptr(m.playTok))  // remove_PlaybackInfoChanged
		m.cur.Release()
	}
	m.cur = cur
	if cur != 0 {
		_ = cur.CallErr(29, uintptr(m.propsCB), uintptr(unsafe.Pointer(&m.propsTok))) // add_MediaPropertiesChanged
		_ = cur.CallErr(27, uintptr(m.playCB), uintptr(unsafe.Pointer(&m.playTok)))   // add_PlaybackInfoChanged
	}
}

func (m *smtc) session() win.Obj {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cur.AddRef()
	return m.cur
}

func (m *smtc) media() (Media, bool) {
	s := m.session()
	if s == 0 {
		return Media{}, false
	}
	defer s.Release()
	var out Media
	if app, err := s.GetString(6); err == nil { // SourceAppUserModelId
		out.Player = playerName(app)
	}
	if info, err := s.Get(9); err == nil { // GetPlaybackInfo
		var status int32
		_ = info.CallErr(7, uintptr(unsafe.Pointer(&status)))
		out.Playing = status == playbackPlaying
		if c, err := info.Get(6); err == nil { // Controls
			var next, prev int32 // WinRT booleans are one byte; the rest stays zero
			_ = c.CallErr(12, uintptr(unsafe.Pointer(&next)))
			_ = c.CallErr(13, uintptr(unsafe.Pointer(&prev)))
			out.CanNext, out.CanPrev = next&0xff != 0, prev&0xff != 0
			c.Release()
		}
		info.Release()
	}
	hasThumb := false
	if op, err := s.Get(7); err == nil { // TryGetMediaPropertiesAsync
		if p, err := win.Await(op, 3*time.Second, true); err == nil {
			out.Title, _ = p.GetString(6)
			out.Artist, _ = p.GetString(9)
			out.Album, _ = p.GetString(10)
			if t, err := p.Get(15); err == nil { // Thumbnail
				hasThumb = true
				t.Release()
			}
			p.Release()
		}
	}
	if hasThumb {
		// Track identity in the URL, so the daemon fetches the art again exactly when it changes.
		h := sha1.Sum([]byte(out.Player + "\x00" + out.Title + "\x00" + out.Artist + "\x00" + out.Album))
		out.ArtURL = "smtc:" + hex.EncodeToString(h[:8])
	}
	return out, true
}

// openThumbnail reads the current session's cover art (Thumbnail →
// OpenReadAsync → a classic IStream over the WinRT stream).
func (m *smtc) openThumbnail(ctx context.Context, _ string) (io.ReadCloser, error) {
	s := m.session()
	if s == 0 {
		return nil, errors.New("no media session")
	}
	defer s.Release()
	op, err := s.Get(7)
	if err != nil {
		return nil, err
	}
	p, err := win.Await(op, 3*time.Second, true)
	if err != nil {
		return nil, err
	}
	defer p.Release()
	ref, err := p.Get(15)
	if err != nil {
		return nil, errors.New("no artwork")
	}
	defer ref.Release()
	op, err = ref.Get(6) // OpenReadAsync
	if err != nil {
		return nil, err
	}
	ras, err := win.Await(op, 5*time.Second, true)
	if err != nil {
		return nil, err
	}
	defer ras.Release()
	rs, err := ras.QueryInterface(iidIRandomAccessStream)
	if err != nil {
		return nil, err
	}
	defer rs.Release()
	var stream win.Obj
	if r, _, _ := procCreateStreamOverRandomAcc.Call(uintptr(rs), uintptr(unsafe.Pointer(iidIStream)), uintptr(unsafe.Pointer(&stream))); win.HR(r) != nil {
		return nil, win.HR(r)
	}
	defer stream.Release()
	var buf bytes.Buffer
	chunk := make([]byte, 64*1024)
	for buf.Len() < 16<<20 && ctx.Err() == nil {
		var n uint32
		r := stream.Call(3, uintptr(unsafe.Pointer(&chunk[0])), uintptr(len(chunk)), uintptr(unsafe.Pointer(&n))) // ISequentialStream::Read
		buf.Write(chunk[:n])
		if win.HR(r) != nil || n == 0 || r == 1 { // S_FALSE: end of stream
			break
		}
	}
	if buf.Len() == 0 {
		return nil, errors.New("empty artwork")
	}
	return io.NopCloser(&buf), nil
}

func (m *smtc) close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cur != 0 {
		_ = m.cur.CallErr(30, uintptr(m.propsTok))
		_ = m.cur.CallErr(28, uintptr(m.playTok))
		m.cur.Release()
		m.cur = 0
	}
}
