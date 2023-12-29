package beater

import (
	"fmt"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/thetherington/bravobeat/beater/jsonrpc"
	"github.com/thetherington/bravobeat/config"
)

// bravobeat configuration.
type bravobeat struct {
	done       chan struct{}
	rpcClients []jsonrpc.RPCClient
	config     config.Config
	client     beat.Client
}

// New creates an instance of bravobeat.
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	clients := make([]jsonrpc.RPCClient, 0)

	for _, node := range c.Nodes {
		client, err := jsonrpc.NewClient(node.Address)
		if err != nil {
			return nil, fmt.Errorf("error creating jsonrpc client: %v", err)
		}

		clients = append(clients, client)
	}

	bt := &bravobeat{
		done:       make(chan struct{}),
		rpcClients: clients,
		config:     c,
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

	eventsChans := make([]<-chan []beat.Event, 0)

	for _, client := range bt.rpcClients {
		byteChan := client.ScanConnectionAsync()

		rpcRespChan := pipeline(byteChan, client.UnMarshalResponse)
		paramsChan := pipeline(rpcRespChan, CastParams)
		eventsChan := pipeline(paramsChan, EventBuilder)

		eventsChans = append(eventsChans, eventsChan)
	}

	events := merge(eventsChans...)

	// params := Metrics{
	// 	Metrics: []string{
	// 		"lms-bravo-studio.ogtcao0imjmuvavytj1q4k5x0g.bx.internal.cloudapp.net.CPU.overall.value",
	// 		"lms-bravo-studio.ogtcao0imjmuvavytj1q4k5x0g.bx.internal.cloudapp.net.memory.memory-cached.value",
	// 	},
	// 	Interval: 15,
	// }

	for i, node := range bt.config.Nodes {
		params := Match{
			Interval: int(node.Period.Seconds()),
		}

		params.Match = ".*CPU.*"
		bt.rpcClients[i].RequestAsync("subscribe", params)

		params.Match = ".*memory.*"
		bt.rpcClients[i].RequestAsync("subscribe", params)

	}

	// ticker := time.NewTicker(bt.config.Period)
	// counter := 1

	for {
		select {
		case <-bt.done:
			return nil
		// case <-ticker.C:
		// case jsonresp := <-rpcResponse:
		// 	if jsonresp.Result != nil {
		// 		if result, _ := jsonresp.GetBool(); result {
		// 			logp.Info("subscription successful")
		// 		}
		// 	}
		// case resp := <-paramsChan:
		// 	if resp != nil {
		// 		// logp.Info(fmt.Sprintf("%+v", resp))
		// 		for k, v := range resp.Metrics {
		// 			logp.Info(k)
		// 			logp.Info(fmt.Sprintf("%d", int(v.Value)))

		// 			// val, err := v.GetFloat()
		// 			// if err != nil {
		// 			// 	fmt.Println(err)
		// 			// } else {
		// 			// 	logp.Info(fmt.Sprintf("%d", int64(val)))
		// 			// }

		// 		}
		// 	}

		// }
		case events := <-events:
			bt.client.PublishAll(events)
			logp.Info("events")
		}

		// event := beat.Event{
		// 	Timestamp: time.Now(),
		// 	Fields: common.MapStr{
		// 		"type":    b.Info.Name,
		// 		"counter": counter,
		// 	},
		// }
		// bt.client.Publish(event)
		// logp.Info("Event sent")
		// counter++
	}
}

// Stop stops bravobeat.
func (bt *bravobeat) Stop() {
	for _, client := range bt.rpcClients {
		client.RequestAsync("unsubscribe")
		client.CloseConnection()
	}

	bt.client.Close()
	close(bt.done)
}
