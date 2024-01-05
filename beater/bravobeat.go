package beater

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/thetherington/bravobeat/beater/jsonrpc"
	"github.com/thetherington/bravobeat/config"
)

// bravobeat configuration.
type bravobeat struct {
	done       chan struct{}
	bravoNodes map[string]*BravoNode
	config     config.Config
	client     beat.Client
}

type BravoNode struct {
	address string
	rpc     jsonrpc.RPCClient
}

// New creates an instance of bravobeat.
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	bt := &bravobeat{
		done:       make(chan struct{}),
		config:     c,
		bravoNodes: make(map[string]*BravoNode),
	}

	return bt, nil
}

// Run starts bravobeat.
func (bt *bravobeat) Run(b *beat.Beat) error {
	logp.Info("bravobeat is running! Hit CTRL-C to stop it.")

	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	// create slices for the events and notifys that will be merged later.
	eventsChans := make([]<-chan []beat.Event, 0)
	notifyChans := make([]<-chan *jsonrpc.Notify, 0)

	// iterate through each node in the configuration
	for _, node := range bt.config.Nodes {
		// build a subscription request for each metrics in the node configuration
		requests := make([]*jsonrpc.RPCRequest, 0)

		for _, metric := range node.Metrics {
			req := jsonrpc.NewRequest("subscribe", Match{
				Interval: int(node.Period.Seconds()),
				Match:    fmt.Sprintf(`.*\.%s[\.-].*`, metric),
			})

			requests = append(requests, req)
		}

		options := jsonrpc.AsyncConnectOptions{
			Timeout: 5 * time.Second,
			Retry:   15 * time.Second,
		}

		// rpc client instantiation with subs
		rpc := jsonrpc.NewClient(node.Address, jsonrpc.WithAsyncConnection(options), jsonrpc.WithSubscriptions(requests...))

		client := &BravoNode{
			address: node.Address,
			rpc:     rpc,
		}

		bt.bravoNodes[node.Address] = client

		// pipeline the receive channel from the async connection to.
		// []byte -> RPCResponse -> RespParams -> []beat.Event
		rpcRespChan := pipeline(rpc.GetRxChan(), rpc.UnMarshalResponse)
		paramsChan := pipeline(rpcRespChan, CastParams)
		eventsChan := pipeline(paramsChan, client.EventBuilder)

		eventsChans = append(eventsChans, eventsChan)
		notifyChans = append(notifyChans, rpc.GetNotifyChan())
	}

	// merge events and notify chans together
	events := merge(eventsChans...)
	notifys := merge(notifyChans...)

	for {
		select {

		case <-bt.done:
			return nil

		case n := <-notifys:
			switch n.Type {
			case jsonrpc.Reconnect:
				if n, ok := bt.bravoNodes[n.Address]; ok {
					n.rpc.ReSubscribe()
				}

			case jsonrpc.Error:
				logp.Err("%s", n.Error.Error())

			case jsonrpc.Info:
				logp.Info("%s %s", n.Address, n.Message)

			case jsonrpc.Debug:
				logp.Debug("msg", "%s %s", n.Address, n.Message)
			}

		case events := <-events:
			bt.client.PublishAll(events)
			logp.Info("events")
		}
	}
}

// Stop stops bravobeat.
func (bt *bravobeat) Stop() {
	var wg sync.WaitGroup

	for _, node := range bt.bravoNodes {
		wg.Add(1)

		go func(rpc jsonrpc.RPCClient) {
			defer wg.Done()
			rpc.RequestAsync("unsubscribe")
			rpc.CloseConnection()
		}(node.rpc)
	}

	wg.Wait()

	bt.client.Close()
	close(bt.done)
}

func (c *BravoNode) EventBuilder(params *RespParams) []beat.Event {
	// group parameters {host: { plugin: [Metric...] },... },...
	nodeGroups := make(map[string]map[string][]*Metric)

	updateHostGroups := func(node string, plugin string, metric *Metric) {
		// check if host key exists and if not create it
		if _, ok := nodeGroups[node]; !ok {
			nodeGroups[node] = make(map[string][]*Metric, 0)
		}

		// check if plugin key exists in host and if not create it
		if _, ok := nodeGroups[node][plugin]; !ok {
			nodeGroups[node][plugin] = make([]*Metric, 0)
		}

		nodeGroups[node][plugin] = append(nodeGroups[node][plugin], metric)
	}

	// grouping response into [Hosts][Plugins] = []Metric
	// incase there are different plugin metrics in the response message with different hosts
	for k, v := range params.Metrics {
		switch {
		case strings.Contains(k, CPU):
			m := MakeMetric(k).withMatch(CPUMatch).withType("percent").withValue(v.Value)
			if m.err == nil {
				updateHostGroups(m.Node, CPU, m)
			}

		case strings.Contains(k, Load):
			m := MakeMetric(k).withMatch(LoadMatch).withType("load").withValue(v.Value)
			if m.err == nil {
				updateHostGroups(m.Node, Load, m)
			}

		case strings.Contains(k, Memory):
			if m := MakeMetric(k).withMatch(MemoryMatch).withValue(v.Value); m.err == nil {
				updateHostGroups(m.Node, Memory, m)
			}

		case strings.Contains(k, Swap):
			if m := MakeMetric(k).withMatch(SwapMatch).withValue(v.Value); m.err == nil {
				updateHostGroups(m.Node, Swap, m)
			}

		case strings.Contains(k, Interface):
			if m := MakeMetric(k).withMatch(InterfaceMatch).withValue(v.Value); m.err == nil {
				updateHostGroups(m.Node, Interface, m)
			}

		case strings.Contains(k, DF):
			if m := MakeMetric(k).withMatch(DFMatch).withValue(v.Value); m.err == nil {
				updateHostGroups(m.Node, DF, m)
			}

		case strings.Contains(k, PTPStatus):
			m := MakeMetric(k).withMatch(PTPStatusMatch).withType("status").withValue(v.Value)
			if m.err == nil {
				updateHostGroups(m.Node, PTPStatus, m)
			}

		case strings.Contains(k, EthStatus):
			m := MakeMetric(k).withMatch(EthStatusMatch).withType("status").withValue(v.Value)
			if m.err == nil {
				updateHostGroups(m.Node, EthStatus, m)
			}
		}
	}

	fmt.Println("groups", fmt.Sprintf("%+v", nodeGroups))

	// build events
	events := make([]beat.Event, 0)

	for node, plugins := range nodeGroups {
		for plugin, metrics := range plugins {

			switch plugin {
			case CPU: // one event per CPU metrics
				for _, m := range metrics {
					events = append(events,
						NewEventBuilder(plugin).
							WithSingleEvent(m).
							WithInstance(InstanceMap{"instance", m.Instance}).
							WithNode(c.address, node).
							Build(),
					)
				}

			case Memory, Swap: // one event for all metrics (grouped)
				events = append(events,
					NewEventBuilder(plugin).
						WithGroupedEvents(metrics).
						WithUsagePct().
						WithNode(c.address, node).
						Build(),
				)

			case Load:
				events = append(events,
					NewEventBuilder(plugin).
						WithGroupedEvents(metrics).
						WithNode(c.address, node).
						Build(),
				)

			case Interface: // one event for each NIC (grouped)
				for nic, metrics := range MakeInstanceGroups(metrics) {
					events = append(events,
						NewEventBuilder(plugin).
							WithSubGroupedEvents(metrics).
							WithInstance(InstanceMap{"port", nic}).
							WithNode(c.address, node).
							Build(),
					)
				}

			case DF: // one event for each mount (grouped)
				for mount, metrics := range MakeInstanceGroups(metrics) {
					events = append(events,
						NewEventBuilder(plugin).
							WithGroupedEvents(metrics).
							WithInstance(InstanceMap{"mount", mount}).
							WithUsagePct().
							WithNode(c.address, node).
							Build(),
					)
				}

			case PTPStatus:
				events = append(events,
					NewEventBuilder(plugin).
						WithSingleEvent(metrics[0]).
						WithEnumStatus("ptp_state", PTP_STATUS_MAP).
						WithNode(c.address, node).
						Build(),
				)

			case EthStatus:
				for nic, metrics := range MakeInstanceGroups(metrics) {
					events = append(events,
						NewEventBuilder(plugin).
							WithSingleEvent(metrics[0]).
							WithInstance(InstanceMap{"port", nic}).
							WithEnumStatus("operational", ETH_STATUS_MAP).
							WithNode(c.address, node).
							Build(),
					)
				}

			}

		}
	}

	return events
}
