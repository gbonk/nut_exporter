package collectors

import (
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var validStates = []string{"Done and passed", "Done and warning", "Done and error", "Aborted", "In progress", "No test initiated", "Test scheduled"}

func (c *NutCollector) UpdateUpsTestResultMetric(ch chan<- prometheus.Metric, metricVar NutVariable) {

	c.logger.Debug("UpdateUpsTestResultMetric: Start")

	name := strings.ReplaceAll(metricVar.Name, ".", "_")
	name = strings.ReplaceAll(name, "-", "_")

	fqName := prometheus.BuildFQName(c.opts.Namespace, "", name)

	varDesc := prometheus.NewDesc(fqName,
		fmt.Sprintf("%s (%s)", metricVar.Description, metricVar.Name),
		[]string{"flag"}, nil,
	)

	for _, state := range validStates {
		var value float64 = 0
		if state == metricVar.Value {
			value = 1
		}

		// Emit the metric directly into the Prometheus channel
		ch <- prometheus.MustNewConstMetric(
			varDesc,
			prometheus.GaugeValue,
			value,
			state,
		)
	}
}
