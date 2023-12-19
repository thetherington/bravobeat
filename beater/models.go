package beater

type Message struct {
	Version string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Id      int         `json:"id"`
	Params  *SendParams `json:"params,omitempty"`
}

type Response struct {
	Version string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  *RespParams `json:"params,omitempty"`
}

type SendParams struct {
	Metrics
	Match
}

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

// GetInt converts the rpc response to an int64 and returns it.
//
// If result was not an integer an error is returned.
// func (v *Value) GetInt() (int64, error) {
// 	i, ok := v.Value.(int64)
// 	if !ok {
// 		return 0, fmt.Errorf("conversion to int64 failed")
// 	}

// 	return i, nil
// }

// // GetFloat converts the rpc response to float64 and returns it.
// //
// // If result was not an float64 an error is returned.
// func (v *Value) GetFloat() (float64, error) {
// 	f, ok := v.Value.(float64)
// 	if !ok {
// 		return 0, fmt.Errorf("conversion to float64 failed")
// 	}

// 	return f, nil
// }
