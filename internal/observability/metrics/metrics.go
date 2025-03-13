package metrics

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/utils"
)

type Outcome string

const (
	Success                  Outcome       = "success"
	Error                    Outcome       = "error"
	MetricRequestTimeout     time.Duration = 5 * time.Second
	MetricRequestIdleTimeout time.Duration = 10 * time.Second
)

func (O Outcome) String() string {
	return string(O)
}

var (
	once                                         sync.Once
	metricsRouter                                *chi.Mux
	pollerDurationHistogram                      *prometheus.HistogramVec
	btcClientDurationHistogram                   *prometheus.HistogramVec
	invalidTransactionsCounter                   *prometheus.CounterVec
	dbLatency                                    *prometheus.HistogramVec
	failedVerifyingUnbondingTxsCounter           prometheus.Counter
	failedVerifyingStakingWithdrawalTxsCounter   prometheus.Counter
	failedVerifyingUnbondingWithdrawalTxsCounter prometheus.Counter
)

// Init initializes the metrics package.
func Init(metricsPort int) {
	once.Do(func() {
		initMetricsRouter(metricsPort)
		registerMetrics()
	})
}

// initMetricsRouter initializes the metrics router.
func initMetricsRouter(metricsPort int) {
	metricsRouter = chi.NewRouter()
	metricsRouter.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(w, r)
	})
	// Create a custom server with timeout settings
	metricsAddr := fmt.Sprintf(":%d", metricsPort)
	server := &http.Server{
		Addr:         metricsAddr,
		Handler:      metricsRouter,
		ReadTimeout:  MetricRequestTimeout,
		WriteTimeout: MetricRequestTimeout,
		IdleTimeout:  MetricRequestIdleTimeout,
	}

	// Start the server in a separate goroutine
	go func() {
		log.Printf("Starting metrics server on %s", metricsAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msgf("Error starting metrics server on %s", metricsAddr)
		}
	}()
}

// registerMetrics initializes and register the Prometheus metrics.
func registerMetrics() {
	pollerDurationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "poll_duration_seconds",
			Help:    "Histogram of poll durations in seconds.",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1200, 3600},
		},
		[]string{"poller_name", "status"},
	)

	btcClientDurationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "btcclient_duration_seconds",
			Help:    "Histogram of btcclient durations in seconds.",
			Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"function", "status"},
	)

	invalidTransactionsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "invalid_txs_counter",
			Help: "Total number of invalid transactions",
		},
		[]string{
			"tx_type",
		},
	)

	failedVerifyingUnbondingTxsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "failed_verifying_unbonding_txs_counter",
			Help: "Total number of failed verifying unbonding txs",
		},
	)

	failedVerifyingStakingWithdrawalTxsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "failed_verifying_staking_withdrawal_txs_counter",
			Help: "Total number of failed verifying staking withdrawal txs",
		},
	)

	failedVerifyingUnbondingWithdrawalTxsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "failed_verifying_unbonding_withdrawal_txs_counter",
			Help: "Total number of failed verifying unbonding withdrawal txs",
		},
	)

	dbLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "db_latency_seconds",
			Help: "Latency of db method calls",
		},
		[]string{"method", "status"},
	)

	prometheus.MustRegister(
		pollerDurationHistogram,
		btcClientDurationHistogram,
		invalidTransactionsCounter,
		failedVerifyingUnbondingTxsCounter,
		failedVerifyingStakingWithdrawalTxsCounter,
		failedVerifyingUnbondingWithdrawalTxsCounter,
		dbLatency,
	)
}

func RecordBtcClientMetrics[T any](clientRequest func() (T, error)) (T, error) {
	var result T
	// 1 for the caller, 2 for the caller of the caller
	// We are using 2, b/c the clientCallWithRetry is used as a wrapper for the actual client request
	functionName := utils.GetFunctionName(2)

	start := time.Now()

	// Perform the client request
	result, err := clientRequest()
	// Determine the outcome status based on whether an error occurred
	status := Success
	if err != nil {
		status = Error
	}

	// Calculate the duration
	duration := time.Since(start).Seconds()

	// Use WithLabelValues to specify the labels and call Observe to record the duration
	btcClientDurationHistogram.WithLabelValues(functionName, status.String()).Observe(duration)

	return result, err
}

func IncrementInvalidStakingWithdrawalTxCounter() {
	invalidTransactionsCounter.WithLabelValues("withdraw_staking_transactions").Inc()
}

func IncrementInvalidUnbondingWithdrawalTxCounter() {
	invalidTransactionsCounter.WithLabelValues("withdraw_unbonding_transactions").Inc()
}

func IncrementInvalidUnbondingTxCounter() {
	invalidTransactionsCounter.WithLabelValues("unbonding_transactions").Inc()
}

func IncrementFailedVerifyingUnbondingTxCounter() {
	failedVerifyingUnbondingTxsCounter.Inc()
}

func IncrementFailedVerifyingStakingWithdrawalTxCounter() {
	failedVerifyingStakingWithdrawalTxsCounter.Inc()
}

func IncrementFailedVerifyingUnbondingWithdrawalTxCounter() {
	failedVerifyingUnbondingWithdrawalTxsCounter.Inc()
}

func ObserveDBLatency(method string, duration time.Duration, failure bool) {
	status := Success
	if failure {
		status = Error
	}

	dbLatency.WithLabelValues(method, status.String()).Observe(duration.Seconds())
}

func ObservePollerDuration(pollerName string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "failure"
	}
	pollerDurationHistogram.WithLabelValues(pollerName, status).Observe(duration.Seconds())
}
