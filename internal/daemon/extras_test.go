package daemon

import (
	"context"
	"os"
	"testing"

	"github.com/beedywool-vibecoded/roundbar/internal/config"
)

// ROUNDBAR_NET=1 go test ./internal/daemon -run Weather -v
func TestWeather(t *testing.T) {
	if os.Getenv("ROUNDBAR_NET") == "" {
		t.Skip("network test")
	}
	w, err := fetchWeather(context.Background(), &config.Weather{City: "Paris", Lat: 48.85, Lon: 2.35, Units: "metric"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", *w)
}
