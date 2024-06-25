package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/log"

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
	rawData, _ := c.GetRawData()
	c.Request.Body = io.NopCloser(bytes.NewBuffer(rawData))

	c.Set("body", rawData)
	log.Debugf("recv request method %s uri %s body %s", c.Request.Method, c.Request.RequestURI, rawData)

	buf, err := matchAPI(c, c.Request.Method, c.Request.RequestURI)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	if !c.IsAborted() {
		c.Data(200, "application/json", buf)
	}
}

func Run(addr string) {
	r := gin.Default()
	RunWithEngine(r, addr)
}

func RunWithEngine(r *gin.Engine, addr string) {
	r.Use(func(c *Context) {
		method := c.Request.Method
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", "*") // 可将将 * 替换为指定的域名
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
			c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
			// c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		c.Next()
	})

	apiEntries.Range(func(key, value any) bool {
		data := strings.SplitN(key.(string), " ", 2)
		r.POST(data[1], dispatchAPI)
		return true
	})

	if err := r.Run(addr); err != nil {
		log.Fatalf("start gin server fail, %v", err)
	}
}
