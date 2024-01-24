package api

import (
	"encoding/json"
	"errors"
	"reflect"
	"sync"

	"quasar/cmd"
	"quasar/log"

	"github.com/gin-gonic/gin"
)

type Context = gin.Context
type IRoutes = gin.IRoutes

type Handler func(*Context, any) (any, error)

type apiEntry struct {
	h     Handler
	typ   reflect.Type
	codec MessageCodec
}

type wrapper struct {
	m  *sync.Map
	id string
	e  *apiEntry
}

func (ctx wrapper) SetCodec(codec MessageCodec) wrapper {
	e := ctx.e
	ctx.e = &apiEntry{h: e.h, typ: e.typ, codec: codec}
	ctx.m.Store(ctx.id, ctx.e)
	return ctx
}

var apiEntries sync.Map

type MessageCodec interface {
	Encode(any) ([]byte, error)
	Decode([]byte) ([]byte, error)
}

type jsonMessageCodec struct{}

func (codec *jsonMessageCodec) Encode(data any) ([]byte, error) {
	return json.Marshal(data)
}

func (codec *jsonMessageCodec) Decode(buf []byte) ([]byte, error) {
	return buf, nil
}

type CmdMessageCodec struct{}

func (codec *CmdMessageCodec) Encode(data any) ([]byte, error) {
	return cmd.Encode("", data)
}

func (codec *CmdMessageCodec) Decode(buf []byte) ([]byte, error) {
	pkg, err := cmd.Decode(buf)
	return pkg.Data, err
}

func merge(method, uri string) string {
	return method + " " + uri
}

var jsonCodec = &jsonMessageCodec{}

func Add(method, uri string, h Handler, i any) wrapper {
	ctx := wrapper{
		id: merge(method, uri),
		m:  &apiEntries,
		e:  &apiEntry{h: h, typ: reflect.TypeOf(i), codec: jsonCodec},
	}
	ctx.m.Store(ctx.id, ctx.e)
	return ctx
}

func matchAPI(c *Context, method, uri string) ([]byte, error) {
	id := merge(method, uri)
	body, _ := c.Get("body")
	rawData, _ := body.([]byte)
	entry, ok := apiEntries.Load(id)
	if !ok {
		return nil, errors.New("dispatch handler: " + id + " is not existed")
	}

	api, _ := entry.(*apiEntry)
	data, err := api.codec.Decode(rawData)
	if err != nil {
		return nil, err
	}

	args := reflect.New(api.typ.Elem()).Interface()
	if err := json.Unmarshal(data, args); err != nil {
		return nil, err
	}
	resp, err := api.h(c, args)
	if err != nil {
		return nil, err
	}
	return api.codec.Encode(resp)
}

// 处理游戏内请求
func dispatchAPI(c *Context) {
	rawData, _ := c.GetRawData() // 只能读一次
	c.Set("body", rawData)
	log.Debugf("recv request method %s uri %s body %s", c.Request.Method, c.Request.RequestURI, rawData)

	buf, err := matchAPI(c, c.Request.Method, c.Request.RequestURI)
	if err != nil {
		buf, _ = json.Marshal(map[string]any{"Code": 1, "Msg": err.Error()})
		log.Warnf("dispatch api error: %v", err)
	}
	c.Data(200, "application/json", buf)
}

func Run(addr string) {
	r := gin.Default()
	r.Use(func(c *Context) {
		c.Writer.Header().Add("Access-Control-Allow-Origin", "*")
	})

	apiEntries.Range(func(key, value any) bool {
		r.POST(key.(string), dispatchAPI)
		return true
	})

	if err := r.Run(addr); err != nil {
		log.Fatalf("start gin server fail, %v", err)
	}
}
