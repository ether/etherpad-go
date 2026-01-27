package stats

import (
	"time"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/adaptor/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	etherpadActivePads = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "etherpad",
			Name:      "active_pads",
			Help:      "Number of currently active Etherpad pads",
		},
	)

	etherpadTotalUsers = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "etherpad",
			Name:      "total_users",
			Help:      "Total number of connected Etherpad users",
		},
	)
)

func Init(store *lib.InitStore) {
	checks := []Checker{
		DBChecker{store.Store},
		EtherpadChecker{store.Handler.SessionStore},
	}

	version, releaseID := settings.BuildInfo()
	store.C.Get("/health", Handler(
		version,
		releaseID,
		"etherpad-api",
		checks,
	))

	if store.RetrievedSettings.EnableMetrics {
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				stats, err := store.Handler.SessionStore.GetStats()
				if err != nil {
					continue
				}

				etherpadActivePads.Set(float64(stats.ActivePads))
				etherpadTotalUsers.Set(float64(stats.ActiveUsers))
			}
		}()
		reg := prometheus.NewRegistry()
		reg.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
			etherpadActivePads,
			etherpadTotalUsers,
		)
		handler := promhttp.HandlerFor(
			reg,
			promhttp.HandlerOpts{},
		)
		store.C.Get("/metrics", adaptor.HTTPHandler(handler))
	}
}
