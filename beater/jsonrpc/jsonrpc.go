package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/textproto"
	"reflect"
	"strconv"
)

const (
	jsonrpcVersion = "2.0"
	delimeter      = "\r\n"
)

type RPCClient interface {
	CloseConnection()
	RequestAsync(method string, params ...interface{}) error
	Dispatch(request *RPCRequest) error
	ScanConnectionAsync() <-chan []byte
	UnMarshalResponse(byteData []byte) *RPCResponse
}

type RPCRequest struct {
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      int         `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
}

func NewRequest(method string, params ...interface{}) *RPCRequest {
	request := &RPCRequest{
		Method:  method,
		Params:  Params(params...),
		JSONRPC: jsonrpcVersion,
	}

	return request
}

func NewRequestWithID(id int, method string, params ...interface{}) *RPCRequest {
	request := &RPCRequest{
		ID:      id,
		Method:  method,
		Params:  Params(params...),
		JSONRPC: jsonrpcVersion,
	}

	return request
}

type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Method  string      `json:"method,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      int         `json:"id"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error function is provided to be used as error object.
func (e *RPCError) Error() string {
	return strconv.Itoa(e.Code) + ": " + e.Message
}

type rpcClient struct {
	address          string
	conn             net.Conn
	defaultRequestID int
	close            bool
}

func NewClient(address string) (RPCClient, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	rpcClient := &rpcClient{
		address: address,
		conn:    conn,
		close:   false,
	}

	return rpcClient, nil
}

func (c *rpcClient) CloseConnection() {
	c.conn.Close()
	c.close = true
}

func (client *rpcClient) RequestAsync(method string, params ...interface{}) error {
	request := &RPCRequest{
		ID:      client.defaultRequestID,
		Method:  method,
		Params:  Params(params...),
		JSONRPC: jsonrpcVersion,
	}

	return client.Dispatch(request)
}

func (client *rpcClient) Dispatch(request *RPCRequest) error {
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(client.conn, string(data)+delimeter)
	if err != nil {
		return err
	}

	return nil
}

func (client *rpcClient) ScanConnectionAsync() <-chan []byte {
	out := make(chan []byte)
	// errChan := make(chan error)

	go func() {
		reader := bufio.NewReader(client.conn)
		tp := textproto.NewReader(reader)

		for {
			line, err := tp.ReadLineBytes()
			if err != nil {
				if client.close {
					break
				}

				netOpError, ok := err.(*net.OpError)
				if ok && netOpError.Err.Error() == "use of closed network connection" {
					continue
				}
			}

			fmt.Println(string(line))
			out <- line
		}

		close(out)
	}()

	return out
}

func (client *rpcClient) UnMarshalResponse(byteData []byte) *RPCResponse {
	var resp *RPCResponse

	err := json.Unmarshal(byteData, &resp)
	if err != nil {
		fmt.Println(err)
	}

	return resp
}

func Params(params ...interface{}) interface{} {
	var finalParams interface{}

	// if params was nil skip this and p stays nil
	if params != nil {
		switch len(params) {
		case 0: // no parameters were provided, do nothing so finalParam is nil and will be omitted
		case 1: // one param was provided, use it directly as is, or wrap primitive types in array
			if params[0] != nil {
				var typeOf reflect.Type

				// traverse until nil or not a pointer type
				for typeOf = reflect.TypeOf(params[0]); typeOf != nil && typeOf.Kind() == reflect.Ptr; typeOf = typeOf.Elem() {
				}

				if typeOf != nil {
					// now check if we can directly marshal the type or if it must be wrapped in an array
					switch typeOf.Kind() {
					// for these types we just do nothing, since value of p is already unwrapped from the array params
					case reflect.Struct:
						finalParams = params[0]
					case reflect.Array:
						finalParams = params[0]
					case reflect.Slice:
						finalParams = params[0]
					case reflect.Interface:
						finalParams = params[0]
					case reflect.Map:
						finalParams = params[0]
					default: // everything else must stay in an array (int, string, etc)
						finalParams = params
					}
				}
			} else {
				finalParams = params
			}
		default: // if more than one parameter was provided it should be treated as an array
			finalParams = params
		}
	}

	return finalParams
}

// GetInt converts the rpc response to an int64 and returns it.
//
// If result was not an integer an error is returned.
func (RPCResponse *RPCResponse) GetInt() (int64, error) {
	val, ok := RPCResponse.Result.(json.Number)
	if !ok {
		return 0, fmt.Errorf("could not parse int64 from %s", RPCResponse.Result)
	}

	i, err := val.Int64()
	if err != nil {
		return 0, err
	}

	return i, nil
}

// GetFloat converts the rpc response to float64 and returns it.
//
// If result was not an float64 an error is returned.
func (RPCResponse *RPCResponse) GetFloat() (float64, error) {
	val, ok := RPCResponse.Result.(json.Number)
	if !ok {
		return 0, fmt.Errorf("could not parse float64 from %s", RPCResponse.Result)
	}

	f, err := val.Float64()
	if err != nil {
		return 0, err
	}

	return f, nil
}

// GetBool converts the rpc response to a bool and returns it.
//
// If result was not a bool an error is returned.
func (RPCResponse *RPCResponse) GetBool() (bool, error) {
	val, ok := RPCResponse.Result.(bool)
	if !ok {
		return false, fmt.Errorf("could not parse bool from %s", RPCResponse.Result)
	}

	return val, nil
}

// GetString converts the rpc response to a string and returns it.
//
// If result was not a string an error is returned.
func (RPCResponse *RPCResponse) GetString() (string, error) {
	val, ok := RPCResponse.Result.(string)
	if !ok {
		return "", fmt.Errorf("could not parse string from %s", RPCResponse.Result)
	}

	return val, nil
}

// GetObject converts the rpc response to an arbitrary type.
//
// The function works as you would expect it from json.Unmarshal()
func (RPCResponse *RPCResponse) GetObject(toType interface{}) error {
	js, err := json.Marshal(RPCResponse.Params)
	if err != nil {
		return err
	}

	err = json.Unmarshal(js, toType)
	if err != nil {
		return err
	}

	return nil
}
