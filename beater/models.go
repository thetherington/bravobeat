package beater

type Metrics struct {
	Metrics  []string `json:"metrics,omitempty"`
	Interval int      `json:"metrics_interval,omitempty"`
}

type Metric struct {
	Host     string      `json:"host,omitempty"`
	Plugin   string      `json:"plugin,omitempty"`
	Instance string      `json:"instance,omitempty"`
	Type     string      `json:"type,omitempty"`
	Value    interface{} `json:"value,omitempty"`
}

type Match struct {
	Match    string `json:"match,omitempty"`
	Interval int    `json:"metrics_interval,omitempty"`
}

type RespParams struct {
	Metrics  map[string]Value `json:"metrics"`
	Interval int              `json:"interval"`
}

type Value struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}
