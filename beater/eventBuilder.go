package beater

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

const (
	CPU       string = "CPU"
	Memory    string = "memory"
	Swap      string = "swap"
	Interface string = "interface"
	DF        string = "df"
	Load      string = "load"
	PTPStatus string = "ptp_status"
	EthStatus string = "eth_status"
)

var PTP_STATUS_MAP = map[int]string{
	0: "INITIALIZING",
	1: "FAULTY",
	2: "LISTENING(FIRST TIME/SUBSEQUENT RESET)",
	3: "PASSIVE",
	4: "UNCALIBRATED",
	5: "SLAVE",
	6: "PRE-MASTER",
	7: "MASTER",
	8: "DISABLED",
	9: "UNKNOWN STATE",
}

var ETH_STATUS_MAP = map[int]string{
	0: "DOWN",
	1: "UP",
}

type eventMod func(*EventBuilder)

type EventBuilder struct {
	plugin  string
	fields  common.MapStr
	metrics []*Metric
	actions []eventMod
}

func NewEventBuilder(plugin string) *EventBuilder {
	return &EventBuilder{
		plugin: plugin,
		fields: common.MapStr{
			"plugin": plugin,
		},
	}
}

func (b *EventBuilder) WithSubGroupedEvents(metrics []*Metric) *EventBuilder {
	b.actions = append([]eventMod{func(eb *EventBuilder) {
		eb.metrics = append(eb.metrics, metrics...)

		for _, m := range metrics {
			eb.fields.DeepUpdate(common.MapStr{
				eb.plugin: common.MapStr{
					m.MetricInstance: common.MapStr{
						m.PluginInstance: m.Value,
					},
				},
			})

		}
	}}, b.actions...)

	return b
}

func (b *EventBuilder) WithGroupedEvents(metrics []*Metric) *EventBuilder {
	b.actions = append([]eventMod{func(eb *EventBuilder) {
		eb.metrics = append(eb.metrics, metrics...)

		for _, m := range metrics {
			eb.fields.DeepUpdate(common.MapStr{
				eb.plugin: common.MapStr{
					m.PluginInstance: m.Value,
				},
			})
		}
	}}, b.actions...)

	return b
}

func (b *EventBuilder) WithSingleEvent(metric *Metric) *EventBuilder {
	b.actions = append([]eventMod{func(eb *EventBuilder) {
		eb.metrics = append(eb.metrics, metric)

		if metric.PluginInstance != "" {
			eb.fields.DeepUpdate(common.MapStr{
				eb.plugin: common.MapStr{
					metric.PluginInstance: metric.Value,
				},
			})
		} else {
			eb.fields.DeepUpdate(common.MapStr{
				eb.plugin: common.MapStr{
					"value": metric.Value,
				},
			})
		}
	}}, b.actions...)

	return b
}

func (b *EventBuilder) WithNode(address string, node ...string) *EventBuilder {
	b.actions = append(b.actions, func(eb *EventBuilder) {
		var n string

		if len(node) > 0 {
			n = node[0]
		} else if len(eb.metrics) > 0 {
			n = eb.metrics[0].Node
		} else {
			n = address
		}

		p := strings.Split(address, ":")

		node := common.MapStr{
			"ip":   p[0],
			"name": n,
		}

		eb.fields.Update(common.MapStr{
			"node": node,
		})
	})

	return b
}

type InstanceMap struct {
	key   string
	value string
}

func (b *EventBuilder) WithInstance(instanceMap InstanceMap) *EventBuilder {
	b.actions = append(b.actions, func(eb *EventBuilder) {
		eb.fields.DeepUpdate(common.MapStr{
			eb.plugin: common.MapStr{
				instanceMap.key: instanceMap.value,
			},
		})
	})

	return b
}

func (b *EventBuilder) WithUsagePct() *EventBuilder {
	b.actions = append(b.actions, func(eb *EventBuilder) {
		var (
			used     float64
			free     float64
			used_pct float64
			flatmap  common.MapStr = eb.fields.Flatten()
		)

		if val, err := flatmap.GetValue(fmt.Sprintf("%s.used", eb.plugin)); err == nil {
			used = float64(val.(int64))
		}

		if val, err := flatmap.GetValue(fmt.Sprintf("%s.free", eb.plugin)); err == nil {
			free = float64(val.(int64))
		}

		if used > 0 {
			used_pct = used / (free + used)
		}

		eb.fields.DeepUpdate(common.MapStr{
			eb.plugin: common.MapStr{
				"used_pct": math.Floor(used_pct*100) / 100,
			},
		})
	})

	return b
}

type StatusMap map[int]string

func (b *EventBuilder) WithEnumStatus(field string, statusMap StatusMap) *EventBuilder {
	b.actions = append(b.actions, func(eb *EventBuilder) {
		flatmap := eb.fields.Flatten()

		if v, err := flatmap.GetValue(fmt.Sprintf("%s.%s", eb.plugin, field)); err == nil {
			index := v.(int)

			if s, ok := statusMap[index]; ok {
				eb.fields.DeepUpdate(common.MapStr{
					eb.plugin: common.MapStr{
						"description": s,
					},
				})
			}
		}
	})

	return b
}

func (b *EventBuilder) Build() beat.Event {
	for _, a := range b.actions {
		a(b)
	}

	var (
		keys      = make([]string, 0)
		data_type string
	)

	for _, m := range b.metrics {
		keys, data_type = append(keys, m.Key), m.Type
	}

	b.fields.Update(common.MapStr{
		"keys":      keys,
		"data_type": data_type,
	})

	event := beat.Event{
		Timestamp: time.Now(),
		Fields:    b.fields,
	}

	return event
}
