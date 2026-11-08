package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"time"

	"github.com/beedywool-vibecoded/roundbar/internal/config"
)

// ---- time -----------------------------------------------------------------

// sendTime gives the device the clock: UTC epoch, local offset and the
// offsets of every world-clock zone the config mentions.
func (d *Daemon) sendTime() {
	now := time.Now()
	_, off := now.Zone()
	zones := map[string]int{}
	d.mu.Lock()
	for _, p := range d.cfg.Profiles {
		for _, pg := range p.Pages {
			for _, z := range pg.Zones {
				if loc, err := time.LoadLocation(z.TZ); err == nil {
					_, o := now.In(loc).Zone()
					zones[z.TZ] = o
				}
			}
		}
	}
	d.mu.Unlock()
	d.send(map[string]any{"t": "time", "epoch": now.Unix(), "offset": off, "zones": zones})
}

// ---- weather --------------------------------------------------------------

type weatherNow struct {
	City  string  `json:"city"`
	Temp  float64 `json:"temp"`
	Code  int     `json:"code"`
	Hi    float64 `json:"hi"`
	Lo    float64 `json:"lo"`
	Units string  `json:"units"`
	Day   bool    `json:"day"`
}

// needsWeather: a clock page with weather on.
func needsWeather(c *config.Config) bool {
	if c.Weather == nil {
		return false
	}
	for _, p := range c.Profiles {
		for _, pg := range p.Pages {
			if pg.Type == "clock" && (pg.ShowWeather == nil || *pg.ShowWeather) {
				return true
			}
		}
	}
	return false
}

func fetchWeather(ctx context.Context, w *config.Weather) (*weatherNow, error) {
	q := url.Values{}
	q.Set("latitude", fmt.Sprint(w.Lat))
	q.Set("longitude", fmt.Sprint(w.Lon))
	q.Set("current", "temperature_2m,weather_code,is_day")
	q.Set("daily", "temperature_2m_max,temperature_2m_min")
	q.Set("forecast_days", "1")
	q.Set("timezone", "auto")
	if w.Units == "imperial" {
		q.Set("temperature_unit", "fahrenheit")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.open-meteo.com/v1/forecast?"+q.Encode(), nil)
	req.Header.Set("User-Agent", "Roundbar")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather: HTTP %d", resp.StatusCode)
	}
	var r struct {
		Current struct {
			Temp  float64 `json:"temperature_2m"`
			Code  int     `json:"weather_code"`
			IsDay int     `json:"is_day"`
		} `json:"current"`
		Daily struct {
			Max []float64 `json:"temperature_2m_max"`
			Min []float64 `json:"temperature_2m_min"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	out := &weatherNow{City: w.City, Temp: r.Current.Temp, Code: r.Current.Code, Units: w.Units, Day: r.Current.IsDay == 1}
	if len(r.Daily.Max) > 0 && len(r.Daily.Min) > 0 {
		out.Hi, out.Lo = r.Daily.Max[0], r.Daily.Min[0]
	}
	return out, nil
}

// clockLoop keeps the device's time and weather fresh while it's attached:
// time hourly (DST), weather every 30 minutes, nothing at all otherwise.
func (d *Daemon) clockLoop() {
	var last *weatherNow
	var lastAt time.Time
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	refresh := func(force bool) {
		d.mu.Lock()
		connected := d.port != nil
		cfg := d.cfg
		d.mu.Unlock()
		if !connected {
			return
		}
		if force || time.Now().Minute() == 0 {
			d.sendTime()
		}
		if !needsWeather(cfg) {
			return
		}
		if last == nil || time.Since(lastAt) > 30*time.Minute {
			if w, err := fetchWeather(context.Background(), cfg.Weather); err == nil {
				last, lastAt = w, time.Now()
			} else {
				log.Printf("weather: %v", err)
			}
		}
		if last != nil && (force || time.Since(lastAt) < time.Minute) {
			d.send(map[string]any{"t": "weather", "city": last.City, "temp": last.Temp, "code": last.Code,
				"hi": last.Hi, "lo": last.Lo, "units": last.Units, "day": last.Day})
		}
	}
	for {
		select {
		case <-d.clockKick:
			refresh(true)
		case <-tick.C:
			refresh(false)
		}
	}
}

func (d *Daemon) kickClock() {
	select {
	case d.clockKick <- struct{}{}:
	default:
	}
}

// ---- timer ----------------------------------------------------------------

func (d *Daemon) timerDone(label, phase string) {
	title := "Timer done"
	switch phase {
	case "work":
		title = "Time for a break"
	case "break":
		title = "Back to work"
	}
	if label == "" {
		label = "Roundbar"
	}
	spawn("notify-send", "-a", "Roundbar", "-i", "alarm-symbolic", "-u", "critical", title, label)
	if p, err := exec.LookPath("canberra-gtk-play"); err == nil {
		spawn(p, "-i", "complete", "-d", "Roundbar timer")
	}
}
