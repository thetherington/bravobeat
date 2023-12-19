package beater

import (
	"fmt"
	"strings"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/thetherington/bravobeat/beater/jsonrpc"
)

func pipeline[I any, O any](input <-chan I, process func(I) O) <-chan O {
	out := make(chan O)

	go func() {
		for in := range input {
			out <- process(in)
		}

		close(out)
	}()

	return out
}

func CastParams(msg *jsonrpc.RPCResponse) *RespParams {
	var params RespParams

	if msg.Method == "update_metrics" {
		msg.GetObject(&params)
	}

	return &params
}

func EventBuilder(params *RespParams) []beat.Event {
	// group parameters
	hostGroups := make(map[string][]Metric)

	for k, v := range params.Metrics {
		switch {
		case strings.Contains(k, "CPU"):
			// fmt.Println(k, fmt.Sprintf("%+v", v))

			parts := strings.Split(k, "CPU")

			host := strings.TrimSuffix(parts[0], ".")

			m := Metric{
				Host:     host,
				Plugin:   "CPU",
				Instance: strings.TrimSuffix(strings.TrimPrefix(parts[1], "."), ".value"),
				Value:    v.Value,
				Type:     "percentage",
			}

			if _, ok := hostGroups[host]; !ok {
				hostGroups[host] = make([]Metric, 0)
			}

			hostGroups[host] = append(hostGroups[host], m)

		case strings.Contains(k, "memory"):
			parts := strings.Split(k, ".memory.")

			host := strings.TrimSuffix(parts[0], ".")

			m := Metric{
				Host:     host,
				Plugin:   "memory",
				Instance: strings.TrimSuffix(strings.TrimPrefix(parts[1], "."), ".value"),
				Value:    int64(v.Value),
				Type:     "bytes",
			}

			if _, ok := hostGroups[host]; !ok {
				hostGroups[host] = make([]Metric, 0)
			}

			hostGroups[host] = append(hostGroups[host], m)

		case strings.Contains("interfaces", k):
		}
	}

	fmt.Println("groups", fmt.Sprintf("%+v", hostGroups))

	// build events
	events := make([]beat.Event, 0)

	for _, v := range hostGroups {

		for _, m := range v {
			event := beat.Event{
				Timestamp: time.Now(),
				Fields: common.MapStr{
					"device": m.Host,
					"plugin": m.Plugin,
					m.Plugin: common.MapStr{
						"instance": m.Instance,
						"value":    m.Value,
						"type":     m.Type,
					},
				},
			}

			events = append(events, event)
		}

	}

	return events
}

// {"jsonrpc": "2.0", "params": {"metrics": {"lms-bravo-studio.ogtcao0imjmuvavytj1q4k5x0g.bx.internal.cloudapp.net.CPU.overall.value": {"timestamp": "1702912816", "value": 84.5}, "lms-bravo-studio.ogtcao0imjmuvavytj1q4k5x0g.bx.internal.cloudapp.net.memory.memory-cached.value": {"timestamp": "1702912809", "value": 2234269696.0}}, "interval": 15}, "method": "update_metrics"}

// &{JSONRPC:2.0 Result:<nil>

// 	Params:map[interval:15 metrics:map[lms-bravo-studio.ogtcao0imjmuvavytj1q4k5x0g.bx.internal.cloudapp.net.CPU.overall.value:map[timestamp:1702914481 value:60.4] lms-bravo-studio.ogtcao0imjmuvavytj1q4k5x0g.bx.internal.cloudapp.net.memory.memory-cached.value:map[timestamp:1702914479 value:2.338029568e+09]]] Error:<nil> ID:0}
