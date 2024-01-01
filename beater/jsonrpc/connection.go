package jsonrpc

import (
	"bufio"
	"fmt"
	"net"
	"net/textproto"
	"sync"
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

// Asynchronous Connection that returns a sending channel and a receiving channel.  also a notification channel.
// Accepts address string: IP:PORT and a termination channel
func AsyncConnect(address string, terminateChan <-chan struct{}, wg *sync.WaitGroup, options *AsyncConnectOptions) (sendChan chan string, receiveChan chan []byte, notifyChan chan *Notify) {
	wg.Add(1)

	sendChan, receiveChan, notifyChan = make(chan string, 10), make(chan []byte), make(chan *Notify, 10)

	cancelChan := make(chan struct{}, 2)

	go func() {
		// defer closing channels after function ends from a connection terminate channel.
		defer func() {
			close(sendChan)
			close(receiveChan)

			notifyChan <- CreateNotify(address, Info, "Shutting down connection routine!")
			time.Sleep(time.Millisecond * time.Duration(500))

			close(notifyChan)

			wg.Done()
		}()

		for {
			d := net.Dialer{Timeout: options.Timeout}

			conn, err := d.Dial("tcp", address)
			if err != nil { // connection failed
				notifyChan <- CreateNotify(address, Error, fmt.Errorf("failed to connect to %s: %w", address, err))
				notifyChan <- CreateNotify(address, Error, fmt.Errorf("trying to reset the connection for %s: %w", address, err))

				// Waiting 10 seconds before trying to reconnect.
				// check for any terminations and exit out of connection routine if does
				ticker := time.NewTicker(options.Retry)
			TICKER:
				for {
					select {
					case <-terminateChan:
						return

					case <-ticker.C:
						break TICKER
					}
				}

			} else { // connection made successfully
				notifyChan <- CreateNotify(address, Info, "Connected!")
				notifyChan <- CreateNotify(address, Reconnect)

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
								notifyChan <- CreateNotify(address, Error, fmt.Errorf("read timeout for %s: %w", address, err))
							} else {
								notifyChan <- CreateNotify(address, Error, fmt.Errorf("read error for %s: %w", address, err))
							}

							cancelChan <- struct{}{}
							break
						}

						receiveChan <- line
					}

					notifyChan <- CreateNotify(address, Info, fmt.Sprintf("closing read routine for %s", address))
				}()

				// create the writing routing to listen on the write channel
				// using textproto to write a line ending with /r/n
				writer := bufio.NewWriter(conn)
				wr := textproto.NewWriter(writer)
			READ:
				for {
					select {
					case data := <-sendChan: // received data into the sending channel
						notifyChan <- CreateNotify(address, Debug, data)

						if err := wr.PrintfLine("%s", data); err != nil {
							notifyChan <- CreateNotify(address, Error, fmt.Errorf("error writing to connection for %s: %w", address, err))
							break READ
						}

					case <-cancelChan: // received a cancel from the read routine
						break READ

					case <-terminateChan: // termination signal received. close the connection go routine
						notifyChan <- CreateNotify(address, Info, fmt.Sprintf("closing connection for %s", address))
						conn.Close()
						return
					}

				}

				// received a cancel or break from the read routine will close the connection and reconnect
				notifyChan <- CreateNotify(address, Info, fmt.Sprintf("closing connection for %s", address))
				conn.Close()
			}
		}
	}()

	return
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
