package collectors

import (
	"log/slog"
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNutCollector_Collect(t *testing.T) {

	slogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	pmChan := make(chan prometheus.Metric, 10)

	go func() {
		for item := range pmChan {
			// Optional: Assert things about 'item' here if needed
			slogger.Info("Item Description: " + item.Desc().String())
		}
	}()

	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		opts   NutCollectorOpts
		logger *slog.Logger
		// Named input parameters for target function.
		ch         chan<- prometheus.Metric
		mockClient *fakeNutClient
	}{
		// Test cases
		{
			name:   "Error - Multiple UPS Devices Detected Without Query Filter",
			opts:   NutCollectorOpts{Namespace: "test_ups", Ups: "", DisableDeviceInfo: true},
			logger: slogger,
			mockClient: &fakeNutClient{
				getUPSListFunc: func() ([]NutUPS, error) {
					return []NutUPS{
						fakeNutUPS{name: "ups-alpha"},
						fakeNutUPS{name: "ups-beta"},
					}, nil
				},
			},
		},
		// {
		// 	name: "test case name",
		// 	opts: NutCollectorOpts{
		// 		Server:     "nut-lv.bonk.cc",
		// 		ServerPort: 3493,
		// 	},
		// 	logger: slogger,
		// 	ch:     pmChan,
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewNutCollector(tt.opts, tt.logger)
			if err != nil {
				t.Fatalf("could not construct receiver type: %v", err)
			}
			// Override the global connection hook pointing to our table's dynamic fakeClient config
			nutConnectHook = func(server string, port int) (NutClient, error) {
				return tt.mockClient, nil
			}

			c.Collect(tt.ch)
		})
	}
}

func TestNutCollector_Collect_Live(t *testing.T) {

	slogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	pmChan := make(chan prometheus.Metric, 10)

	go func() {
		for item := range pmChan {
			// Optional: Assert things about 'item' here if needed
			slogger.Info("Item Description: " + item.Desc().String())
		}
	}()

	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		opts   NutCollectorOpts
		logger *slog.Logger
		// Named input parameters for target function.
		ch         chan<- prometheus.Metric
		mockClient *fakeNutClient
	}{
		// Test cases
		{
			name: "test case name",
			opts: NutCollectorOpts{
				Server:     "nut-lv.bonk.cc",
				ServerPort: 3493,
			},
			logger: slogger,
			ch:     pmChan,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewNutCollector(tt.opts, tt.logger)
			if err != nil {
				t.Fatalf("could not construct receiver type: %v", err)
			}

			c.Collect(tt.ch)
		})
	}
}
