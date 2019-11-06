package core

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics for monitoring service.
var (
	//blockHeight prometheus metric.
	blockHeight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Current index of processed block",
			Name:      "current_block_height",
			Namespace: "neogo",
		},
	)
	//persistedHeight prometheus metric.
	persistedHeight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Current persisted block count",
			Name:      "current_persisted_height",
			Namespace: "neogo",
		},
	)
	//headerHeight prometheus metric.
	headerHeight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Current header height",
			Name:      "current_header_height",
			Namespace: "neogo",
		},
	)
	//mempoolUnsortedTx prometheus metric.
	mempoolUnsortedTx = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Mempool Unsorted TXs",
			Name:      "mempool_unsorted_tx",
			Namespace: "neogo",
		},
	)
	//mempoolUnverifiedTx prometheus metric.
	mempoolUnverifiedTx = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Mempool Unverified TXs",
			Name:      "mempool_unverified_tx",
			Namespace: "neogo",
		},
	)
)

func init() {
	prometheus.MustRegister(
		blockHeight,
		persistedHeight,
		headerHeight,
		mempoolUnsortedTx,
		mempoolUnverifiedTx,
	)
}

func updatePersistedHeightMetric(pHeight uint32) {
	persistedHeight.Set(float64(pHeight))
}

func updateHeaderHeightMetric(hHeight int) {
	headerHeight.Set(float64(hHeight))
}

func updateBlockHeightMetric(bHeight uint32) {
	blockHeight.Set(float64(bHeight))
}

func updateMempoolMetrics(unsortedTxnLen int, unverifiedTxnLen int) {
	mempoolUnsortedTx.Set(float64(unsortedTxnLen))
	mempoolUnverifiedTx.Set(float64(unverifiedTxnLen))
}