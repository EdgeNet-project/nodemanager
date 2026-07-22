package metrics

import (
	"time"
)

type Metrics struct {
	Timestamp time.Time `json:"timestamp"`

	CPU     CPUMetrics     `json:"cpu"`
	IO      IOMetrics      `json:"io"`
	Network NetworkMetrics `json:"network"`
}

type CPUMetrics struct {
	UsagePercent float64 `json:"usage_percent"`
	Load1        float64 `json:"load1"`
	Load5        float64 `json:"load5"`
	Load15       float64 `json:"load15"`
}

type IOMetrics struct {
	ReadBytesPerSec  float64 `json:"read_bytes_per_sec"`
	WriteBytesPerSec float64 `json:"write_bytes_per_sec"`
}

type NetworkMetrics struct {
	Interfaces []InterfaceMetrics `json:"interfaces"`
}

type InterfaceMetrics struct {
	Name string `json:"name"`

	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`

	RxBytesPerSec float64 `json:"rx_bytes_per_sec"`
	TxBytesPerSec float64 `json:"tx_bytes_per_sec"`
}

type Collector interface {
	Collect() (*Metrics, error)
}
