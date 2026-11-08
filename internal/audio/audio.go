// Package audio talks to PipeWire through its PulseAudio protocol server:
// default sink volume, default source (mic) mute, change notifications.
package audio

import (
	"math"
	"sync"
	"time"

	"github.com/jfreymuth/pulse/proto"
)

type State struct {
	Volume   int // 0..100 (can exceed 100 when boosted; clamped for the device)
	Muted    bool
	Mic      int
	MicMuted bool
}

type Audio struct {
	mu     sync.Mutex
	c      *proto.Client
	closed chan struct{}
	// Changed receives a value (coalesced) whenever the default sink/source changes.
	Changed chan struct{}
}

func Connect() (*Audio, error) {
	c, conn, err := proto.Connect("")
	if err != nil {
		return nil, err
	}
	c.SetTimeout(2 * time.Second)
	a := &Audio{c: c, closed: make(chan struct{}), Changed: make(chan struct{}, 1)}
	c.Callback = func(msg any) {
		switch m := msg.(type) {
		case *proto.SubscribeEvent:
			f := m.Event.GetFacility()
			if f == proto.EventSink || f == proto.EventSource || f == proto.EventServer {
				select {
				case a.Changed <- struct{}{}:
				default:
				}
			}
		case *proto.ConnectionClosed:
			conn.Close()
			select {
			case <-a.closed:
			default:
				close(a.closed)
			}
		}
	}
	props := proto.PropList{
		"application.name":      proto.PropListString("roundbar"),
		"application.icon_name": proto.PropListString("input-dialpad"),
	}
	if err := c.Request(&proto.SetClientName{Props: props}, &proto.SetClientNameReply{}); err != nil {
		conn.Close()
		return nil, err
	}
	mask := proto.SubscriptionMaskSink | proto.SubscriptionMaskSource | proto.SubscriptionMaskServer
	if err := c.Request(&proto.Subscribe{Mask: mask}, nil); err != nil {
		conn.Close()
		return nil, err
	}
	return a, nil
}

// Closed is closed when the server connection drops.
func (a *Audio) Closed() <-chan struct{} { return a.closed }

func pct(v proto.Volume) int { return int(math.Round(float64(v) * 100 / float64(proto.VolumeNorm))) }

func (a *Audio) defaults() (sink, source string, err error) {
	var info proto.GetServerInfoReply
	if err = a.c.Request(&proto.GetServerInfo{}, &info); err != nil {
		return
	}
	return info.DefaultSinkName, info.DefaultSourceName, nil
}

func (a *Audio) State() (State, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var s State
	sink, source, err := a.defaults()
	if err != nil {
		return s, err
	}
	var si proto.GetSinkInfoReply
	if err := a.c.Request(&proto.GetSinkInfo{SinkIndex: proto.Undefined, SinkName: sink}, &si); err == nil {
		s.Volume, s.Muted = pct(si.ChannelVolumes.Avg()), si.Mute
	}
	var so proto.GetSourceInfoReply
	if err := a.c.Request(&proto.GetSourceInfo{SourceIndex: proto.Undefined, SourceName: source}, &so); err == nil {
		s.Mic, s.MicMuted = pct(so.ChannelVolumes.Avg()), so.Mute
	}
	return s, nil
}

func volumes(n int, p int) proto.ChannelVolumes {
	if n == 0 {
		n = 2
	}
	p = max(0, min(100, p))
	v := proto.Volume(math.Round(float64(p) * float64(proto.VolumeNorm) / 100))
	cv := make(proto.ChannelVolumes, n)
	for i := range cv {
		cv[i] = v
	}
	return cv
}

// SetVolume sets the default sink to p percent on all channels and unmutes it.
func (a *Audio) SetVolume(p int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	sink, _, err := a.defaults()
	if err != nil {
		return err
	}
	var si proto.GetSinkInfoReply
	if err := a.c.Request(&proto.GetSinkInfo{SinkIndex: proto.Undefined, SinkName: sink}, &si); err != nil {
		return err
	}
	if si.Mute && p > 0 {
		_ = a.c.Request(&proto.SetSinkMute{SinkIndex: proto.Undefined, SinkName: sink, Mute: false}, nil)
	}
	return a.c.Request(&proto.SetSinkVolume{SinkIndex: proto.Undefined, SinkName: sink, ChannelVolumes: volumes(len(si.ChannelVolumes), p)}, nil)
}

func (a *Audio) SetMicVolume(p int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, source, err := a.defaults()
	if err != nil {
		return err
	}
	var so proto.GetSourceInfoReply
	if err := a.c.Request(&proto.GetSourceInfo{SourceIndex: proto.Undefined, SourceName: source}, &so); err != nil {
		return err
	}
	return a.c.Request(&proto.SetSourceVolume{SourceIndex: proto.Undefined, SourceName: source, ChannelVolumes: volumes(len(so.ChannelVolumes), p)}, nil)
}

// Mic applies "toggle" | "mute" | "unmute" to the default source.
func (a *Audio) Mic(cmd string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, source, err := a.defaults()
	if err != nil {
		return err
	}
	mute := cmd == "mute"
	if cmd == "toggle" {
		var so proto.GetSourceInfoReply
		if err := a.c.Request(&proto.GetSourceInfo{SourceIndex: proto.Undefined, SourceName: source}, &so); err != nil {
			return err
		}
		mute = !so.Mute
	}
	return a.c.Request(&proto.SetSourceMute{SourceIndex: proto.Undefined, SourceName: source, Mute: mute}, nil)
}
