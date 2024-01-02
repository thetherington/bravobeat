package jsonrpc

import (
	"bufio"
	"fmt"
	"net"
	"net/textproto"
	"time"
)

const (
	Reconnect int = iota
	Error
	Info
	Debug
)

type Notify struct {
	Address string
	Error   error
	Message string
	Type    int
}

// Create a asynchrounous connection that reads and writes through channels
//
// Asynchronous Connection has a reconnection system with timeouts from AsyncConnectionOptions
// Default timeout 5 seconds and retry is 10 seconds
func WithAsyncConnection(options ...AsyncConnectOptions) func(*rpcClient) {
	var (
		sendChan        chan string   = make(chan string, 10)
		receiveChan     chan []byte   = make(chan []byte)
		notifyChan      chan *Notify  = make(chan *Notify, 10)
		terminationChan chan struct{} = make(chan struct{})
		restartChan     chan struct{} = make(chan struct{})

		// default connection options
		o *AsyncConnectOptions = &AsyncConnectOptions{
			Timeout: 5 * time.Second,
			Retry:   10 * time.Second,
		}
	)

	if len(options) > 0 {
		if options[0].Timeout > 0 {
			o.Timeout = options[0].Timeout
		}

		if options[0].Retry > 0 {
			o.Retry = options[0].Retry
		}
	}

	return func(rpc *rpcClient) {
		// cleanup function to close the tx,rx and notify channels
		cleanup := func() {
			close(sendChan)
			close(receiveChan)

			notifyChan <- CreateNotify(rpc.address, Info, "Shutting down connection routine!")
			time.Sleep(time.Millisecond * time.Duration(500))

			close(notifyChan)

			rpc.wg.Done()
		}

		rpc.wg.Add(1)
		go func() {
			// blocks beat from exiting until deferred cleanup function is completed
			defer cleanup()

			for {
				d := net.Dialer{Timeout: o.Timeout}

				conn, err := d.Dial("tcp", rpc.address)
				if err != nil { // connection failed

					notifyChan <- CreateNotify(rpc.address, Error, fmt.Errorf("failed to connect to %s: %w", rpc.address, err))
					notifyChan <- CreateNotify(rpc.address, Error, fmt.Errorf("trying to reset the connection for %s: %w", rpc.address, err))

					// Waiting 10 seconds (default) before trying to reconnect.
					// check for any terminations and exit out of connection routine if does
					ticker := time.NewTicker(o.Retry)
				TICKER:
					for {
						select {
						case <-terminationChan:
							return

						case <-ticker.C:
							break TICKER
						}
					}

				} else { // connection made successfully
					notifyChan <- CreateNotify(rpc.address, Info, "Connected!")
					notifyChan <- CreateNotify(rpc.address, Reconnect)

					// create the reading routine and sending bytes into the receive channel
					// using the textproto package to read anything ending /r/n
					go func() {
						reader := bufio.NewReader(conn)
						tp := textproto.NewReader(reader)

						for {
							conn.SetReadDeadline(time.Now().Add(60 * time.Second))

							line, err := tp.ReadLineBytes()
							if err != nil {
								if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
									notifyChan <- CreateNotify(rpc.address, Error, fmt.Errorf("read timeout for %s: %w", rpc.address, err))
								} else {
									notifyChan <- CreateNotify(rpc.address, Error, fmt.Errorf("read error for %s: %w", rpc.address, err))
								}

								restartChan <- struct{}{}
								break
							}

							receiveChan <- line
						}

						notifyChan <- CreateNotify(rpc.address, Info, fmt.Sprintf("closing read routine for %s", rpc.address))
					}()

					// create the writing routing to listen on the write channel
					// using textproto to write a line ending with /r/n
					writer := bufio.NewWriter(conn)
					wr := textproto.NewWriter(writer)
				READ:
					for {
						select {
						case data := <-sendChan: // received data into the sending channel
							notifyChan <- CreateNotify(rpc.address, Debug, data)

							if err := wr.PrintfLine("%s", data); err != nil {
								notifyChan <- CreateNotify(rpc.address, Error, fmt.Errorf("error writing to connection for %s: %w", rpc.address, err))
								break READ
							}

						case <-restartChan: // received a cancel from the read routine
							break READ

						case <-terminationChan: // termination signal received. close the connection go routine
							notifyChan <- CreateNotify(rpc.address, Info, fmt.Sprintf("closing connection for %s", rpc.address))
							conn.Close()
							return
						}

					}

					// received a cancel or break from the read routine will close the connection and reconnect
					notifyChan <- CreateNotify(rpc.address, Info, fmt.Sprintf("closing connection for %s", rpc.address))
					conn.Close()
				}
			}
		}()

		rpc.txChan = sendChan
		rpc.rxChan = receiveChan
		rpc.notifyChan = notifyChan
		rpc.terminateChan = terminationChan
	}
}

// create a notifcation message for the notify channel
func CreateNotify(address string, Type int, params ...any) *Notify {
	switch Type {
	case Reconnect:
		return &Notify{
			Address: address,
			Type:    Reconnect,
		}
	case Error:
		return &Notify{
			Address: address,
			Type:    Error,
			Error:   params[0].(error),
		}
	case Info:
		return &Notify{
			Address: address,
			Type:    Info,
			Message: params[0].(string),
		}
	case Debug:
		return &Notify{
			Address: address,
			Type:    Debug,
			Message: params[0].(string),
		}
	}

	return nil
}
