package beater

type Metrics struct {
	Metrics  []string `json:"metrics,omitempty"`
	Interval int      `json:"metrics_interval,omitempty"`
}

type Metric struct {
	Node           string      `json:"node,omitempty"`
	Plugin         string      `json:"plugin,omitempty"`
	PluginInstance string      `json:"plugin_instance,omitempty"`
	Instance       string      `json:"instance,omitempty"`
	MetricInstance string      `json:"metric_instance,omitempty"`
	Type           string      `json:"type,omitempty"`
	Value          interface{} `json:"value,omitempty"`
	Key            string      `json:"key,omitempty"`
	err            error
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
