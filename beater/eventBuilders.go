package beater

import (
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

func CreateNodeInfo(address string, hostname string) common.MapStr {
	p := strings.Split(address, ":")

	return common.MapStr{
		"ip":   p[0],
		"name": hostname,
	}
}

func CreateUsagePercent(usageKey string, freeKey string, fields *common.MapStr) {
	var (
		used     float64
		free     float64
		used_pct float64
	)

	if val, err := fields.GetValue(usageKey); err == nil {
		used = float64(val.(int64))
	}

	if val, err := fields.GetValue(freeKey); err == nil {
		free = float64(val.(int64))
	}

	if used > 0 {
		used_pct = used / (free + used)
	}

	fields.Update(common.MapStr{
		"used_pct": math.Floor(used_pct*100) / 100,
	})
}

func CPUEvent(address string, metric *Metric) beat.Event {
	fields := common.MapStr{
		"instance": metric.Instance,
		"value":    metric.Value,
	}

	event := beat.Event{
		Timestamp: time.Now(),
		Fields: common.MapStr{
			"node":      CreateNodeInfo(address, metric.Node),
			"CPU":       fields,
			"plugin":    CPU,
			"data_type": metric.Type,
			"keys":      []string{metric.Key},
		},
	}

	return event
}

type InstanceMap struct {
	key   string
	value string
}

func GroupedEvent(address string, metrics []*Metric, plugin string, instance ...InstanceMap) beat.Event {
	var (
		keys   = make([]string, 0)
		fields = common.MapStr{}
	)

	for _, m := range metrics {
		fields.Update(common.MapStr{
			m.PluginInstance: m.Value,
		})

		keys = append(keys, m.Key)
	}

	if len(instance) > 0 {
		fields.Update(common.MapStr{
			instance[0].key: instance[0].value,
		})
	}

	if plugin == Memory || plugin == Swap || plugin == DF {
		CreateUsagePercent("used", "free", &fields)
	}

	event := beat.Event{
		Timestamp: time.Now(),
		Fields: common.MapStr{
			"node":      CreateNodeInfo(address, metrics[0].Node),
			plugin:      fields,
			"plugin":    plugin,
			"data_type": metrics[0].Type,
			"keys":      keys,
		},
	}

	return event
}

func InterfaceEvent(address string, port string, metrics []*Metric) beat.Event {
	var (
		keys   = make([]string, 0)
		fields = common.MapStr{
			"port": port,
			"rx":   common.MapStr{},
			"tx":   common.MapStr{},
		}
	)

	for _, m := range metrics {
		fields.DeepUpdate(common.MapStr{
			m.MetricInstance: common.MapStr{
				m.PluginInstance: m.Value,
			},
		})

		keys = append(keys, m.Key)
	}

	event := beat.Event{
		Timestamp: time.Now(),
		Fields: common.MapStr{
			"node":      CreateNodeInfo(address, metrics[0].Node),
			"interface": fields,
			"plugin":    Interface,
			"data_type": metrics[0].Type,
			"keys":      keys,
		},
	}

	return event
}

func SingleEventStatus(address string, metric *Metric, plugin string, statusMap map[int]string, instance ...InstanceMap) beat.Event {
	fields := common.MapStr{}

	index := metric.Value.(int)

	if v, ok := statusMap[index]; ok {
		fields.Update(common.MapStr{
			"description": v,
		})
	}

	fields.Update(common.MapStr{
		"value": metric.Value,
	})

	if len(instance) > 0 {
		fields.Update(common.MapStr{
			instance[0].key: instance[0].value,
		})
	}

	event := beat.Event{
		Timestamp: time.Now(),
		Fields: common.MapStr{
			"node":      CreateNodeInfo(address, metric.Node),
			plugin:      fields,
			"plugin":    plugin,
			"data_type": metric.Type,
			"keys":      []string{metric.Key},
		},
	}

	return event
}
