package beater

import (
	"fmt"
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
	rpcClients map[string]jsonrpc.RPCClient
	config     config.Config
	client     beat.Client
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
		rpcClients: make(map[string]jsonrpc.RPCClient),
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
				Match:    fmt.Sprintf(".*%s.*", metric),
			})

			requests = append(requests, req)
		}

		options := jsonrpc.AsyncConnectOptions{
			Timeout: 5 * time.Second,
			Retry:   15 * time.Second,
		}

		// client instantiation with subs
		client := jsonrpc.NewClient(node.Address, jsonrpc.WithAsyncConnection(options), jsonrpc.WithSubscriptions(requests...))

		bt.rpcClients[node.Address] = client

		// pipeline the receive channel from the async connection to.
		// []byte -> RPCResponse -> RespParams -> []beat.Event
		rpcRespChan := pipeline(client.GetRxChan(), client.UnMarshalResponse)
		paramsChan := pipeline(rpcRespChan, CastParams)
		eventsChan := pipeline(paramsChan, EventBuilder)

		eventsChans = append(eventsChans, eventsChan)
		notifyChans = append(notifyChans, client.GetNotifyChan())
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
				if c, ok := bt.rpcClients[n.Address]; ok {
					c.ReSubscribe()
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

	for _, client := range bt.rpcClients {
		wg.Add(1)

		go func(c jsonrpc.RPCClient) {
			defer wg.Done()
			c.RequestAsync("unsubscribe")
			c.CloseConnection()
		}(client)
	}

	wg.Wait()

	bt.client.Close()
	close(bt.done)
}
