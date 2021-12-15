package internal

import (
	"encoding/json"
	"reflect"
)

type ForwardArgs struct {
	ServerName string
	MsgId      string
	MsgData    json.RawMessage
}

type M map[string]interface{}

func (m M) MarshalJSON() ([]byte, error) {
	copyM := map[string]interface{}{}
	for k, v := range m {
		if v != nil {
			switch ref := reflect.ValueOf(v); ref.Kind() {
			case reflect.Ptr, reflect.Slice:
				if !ref.IsNil() {
					copyM[k] = v
				}
			}
		}
	}
	return json.Marshal(copyM)
}
