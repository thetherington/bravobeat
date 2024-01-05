package beater

import (
	"errors"
	"regexp"
	"strings"

	"github.com/thetherington/bravobeat/beater/jsonrpc"
)

var (
	CPUMatch       = regexp.MustCompile(`(?P<node>.*).(?P<plugin>CPU).(?:per_)*(?P<instance>.*)(?:..*value)`)
	MemoryMatch    = regexp.MustCompile(`(?P<node>.*).(?P<plugin>memory).memory-(?P<plugin_instance>.*)(?:..*value)`)
	SwapMatch      = regexp.MustCompile(`(?P<node>.*).(?P<plugin>swap).swap[_-](?P<plugin_instance>.*)(?:..*value)`)
	InterfaceMatch = regexp.MustCompile(`(?P<node>.*).(?P<plugin>interface)-(?P<instance>.*).(?P<plugin_instance>if.*).(?P<metric_instance>tx|rx)`)
	DFMatch        = regexp.MustCompile(`(?P<node>.*).(?P<plugin>df)-(?P<instance>.*).df_complex-(?P<plugin_instance>.*).value`)
	LoadMatch      = regexp.MustCompile(`(?P<node>.*).(?P<plugin>load).load.(?P<plugin_instance>.*)`)
	PTPStatusMatch = regexp.MustCompile(`(?P<node>.*).(?P<plugin>ptp_status).(?P<plugin_instance>.*)(?:..*state)`)
	EthStatusMatch = regexp.MustCompile(`(?P<node>.*).(?P<plugin>eth_status)\.status-(?P<instance>.*)\.(?P<plugin_instance>.*)`)
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

func merge[I any](cs ...<-chan I) <-chan I {
	out := make(chan I)

	send := func(c <-chan I) {
		for n := range c {
			out <- n
		}
	}

	for _, c := range cs {
		go send(c)
	}

	return out
}

func CastParams(msg *jsonrpc.RPCResponse) *RespParams {
	var params RespParams

	if msg.Method == "update_metrics" {
		msg.GetObject(&params)
	}

	return &params
}

// Creates a Metric struct with the key value from the parameters response
func MakeMetric(key string) *Metric {
	return &Metric{
		Key: key,
	}
}

// Parses the key from the struct with a regular expression
func (m *Metric) withMatch(exp *regexp.Regexp) *Metric {
	result := make(map[string]string)

	match := exp.FindStringSubmatch(m.Key)

	for i, name := range exp.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}

	if v, ok := result["node"]; ok {
		m.Node = v
	}
	if v, ok := result["plugin"]; ok {
		m.Plugin = v
	}
	if v, ok := result["instance"]; ok {
		m.Instance = v
	}
	if v, ok := result["plugin_instance"]; ok {
		m.PluginInstance = strings.ReplaceAll(v, "-", "_")
	}
	if v, ok := result["metric_instance"]; ok {
		m.MetricInstance = v
	}

	if m.Node == "" || m.Plugin == "" {
		m.err = errors.New("no node or plugin found")
	}

	return m
}

func (m *Metric) withValue(value float64) *Metric {
	if m.err != nil {
		return m
	}

	switch m.Type {
	case "percent", "load":
		m.Value = value

	case "status":
		m.Value = int(value)

	case "":
		m.Type = "bytes"
		m.Value = int64(value)

	default:
		m.Value = int64(value)
	}

	return m
}

func (m *Metric) withType(dataType string) *Metric {
	if m.err != nil {
		return m
	}

	m.Type = dataType

	return m
}

type InstanceGroups map[string][]*Metric

func MakeInstanceGroups(metrics []*Metric) InstanceGroups {
	// regroup metrics for each nic interfaces
	var sub_group InstanceGroups = make(map[string][]*Metric)

	for _, m := range metrics {
		if _, ok := sub_group[m.Instance]; !ok {
			sub_group[m.Instance] = make([]*Metric, 0)
		}
		sub_group[m.Instance] = append(sub_group[m.Instance], m)
	}

	return sub_group
}
