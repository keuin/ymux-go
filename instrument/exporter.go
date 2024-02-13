package instrument

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"time"
)

const (
	infoKey       = "instrument"
	labelUsername = "username"
	labelServerID = "serverId"
)

func SetInstrument(c *gin.Context, info RequestInfo) {
	c.Set(infoKey, info)
}

// Exporter exports the following metrics for the yggdrasil server:
//   - request_process_time_seconds
//   - total_request_count
//   - success_count
//   - fail_count
//   - logged_in_count
//   - not_logged_in_count
type Exporter struct {
	requestProcessTime prometheus.HistogramVec
	totalRequestCount  prometheus.CounterVec
	successCount       prometheus.CounterVec
	failCount          prometheus.CounterVec
	loggedInCount      prometheus.CounterVec
	notLoggedInCount   prometheus.CounterVec
}

func (e *Exporter) initMetrics() {
	e.requestProcessTime.WithLabelValues("", "")
	e.totalRequestCount.WithLabelValues("", "")
	e.successCount.WithLabelValues("", "")
	e.failCount.WithLabelValues("", "")
	e.loggedInCount.WithLabelValues("", "")
	e.notLoggedInCount.WithLabelValues("", "")
}

func (e *Exporter) Describe(descs chan<- *prometheus.Desc) {
	e.requestProcessTime.Describe(descs)
	e.totalRequestCount.Describe(descs)
	e.successCount.Describe(descs)
	e.failCount.Describe(descs)
	e.loggedInCount.Describe(descs)
	e.notLoggedInCount.Describe(descs)
}

func (e *Exporter) Collect(metrics chan<- prometheus.Metric) {
	e.requestProcessTime.Collect(metrics)
	e.totalRequestCount.Collect(metrics)
	e.successCount.Collect(metrics)
	e.failCount.Collect(metrics)
	e.loggedInCount.Collect(metrics)
	e.notLoggedInCount.Collect(metrics)
}

// Instrument incoming `/hasJoined` request.
// The handler must call SetInstrument before returning.
func (e *Exporter) Instrument(c *gin.Context) {
	t0 := time.Now()
	c.Next()
	dur := time.Since(t0)
	ri := c.MustGet(infoKey).(RequestInfo)

	labels := prometheus.Labels{
		labelUsername: ri.Username,
		labelServerID: ri.ServerID,
	}

	e.requestProcessTime.With(labels).Observe(dur.Seconds())
	e.totalRequestCount.With(labels).Inc()
	if ri.Success {
		e.successCount.With(labels).Inc()
		if ri.LoggedIn {
			e.loggedInCount.With(labels).Inc()
		} else {
			e.notLoggedInCount.With(labels).Inc()
		}
	} else {
		e.failCount.With(labels).Inc()
	}
}

type RequestInfo struct {
	// ProcessTime is the total time elapsed for the HTTP request
	ProcessTime time.Duration
	// Success is true if and only if all requests to upstreams has succeeded
	Success bool
	// Username is from `hasJoined` API call parameter
	Username string
	// ServerID is from `hasJoined` API call parameter
	ServerID string
	// LoggedIn is the HTTP response
	LoggedIn bool
}

func NewExporter(r prometheus.Registerer) *Exporter {
	const (
		namespace = "ymux"
		subsystem = "api"
	)
	labels := []string{labelUsername, labelServerID}
	exp := &Exporter{
		requestProcessTime: *promauto.With(r).NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "request_process_time_seconds",
			Help:      "time used for serving one /hasJoined request",
		}, labels),
		totalRequestCount: *promauto.With(r).NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "total_request_count",
			Help:      "requests processed by this process",
		}, labels),
		successCount: *promauto.With(r).NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "success_count",
			Help:      "successful requests",
		}, labels),
		failCount: *promauto.With(r).NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "fail_count",
			Help:      "errored requests",
		}, labels),
		loggedInCount: *promauto.With(r).NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "logged_in_count",
			Help:      "requests with 204 (not logged in) /hasJoined result",
		}, labels),
		notLoggedInCount: *promauto.With(r).NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "not_logged_in_count",
			Help:      "requests with 200 (logged in) /hasJoined result",
		}, labels),
	}
	exp.initMetrics()
	return exp
}
