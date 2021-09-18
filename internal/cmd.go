package internal

import "encoding/json"

type ForwardArgs struct {
	ServerName string
	MsgId      string
	MsgData    json.RawMessage
}
