package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"sync"

	"github.com/guogeer/quasar/v2/log"

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

type MessageCodec interface {
	ParseRequest([]byte) ([]byte, error)
	ResponseError(any, error) ([]byte, error)
}

func merge(method, uri string) string {
	return method + " " + uri
}

var apiEntries sync.Map

type Group struct {
	codec    MessageCodec
	basePath string
	route    IRoutes
}

func NewGroup(basePath string, route IRoutes, codec MessageCodec) *Group {
	return &Group{basePath: basePath, route: route, codec: codec}
}

func (group *Group) Add(method, uri string, h Handler, args any) {
	group.Handle(method, uri, h, args)
}

func (group *Group) Handle(method, uri string, h Handler, args any) {
	apiEntries.Store(merge(method, group.basePath+uri), &apiEntry{h: h, typ: reflect.TypeOf(args), codec: group.codec})
	group.route.Handle(method, uri, dispatchAPI)
}

func (group *Group) POST(name string, h Handler, args interface{}) {
	group.Handle("POST", name, h, args)
}

func (group *Group) GET(name string, h Handler, args interface{}) {
	group.Handle("GET", name, h, args)
}

func (group *Group) PUT(name string, h Handler, args interface{}) {
	group.Handle("PUT", name, h, args)
}

func (group *Group) DELETE(name string, h Handler, args interface{}) {
	group.Handle("DELETE", name, h, args)
}

func handleAPI(c *Context, method, uri string) ([]byte, error) {
	id := merge(method, uri)
	rawData, _ := c.GetRawData()
	c.Request.Body = io.NopCloser(bytes.NewBuffer(rawData))

	entry, ok := apiEntries.Load(id)
	if !ok {
		return nil, errors.New("handle url " + id + " is not existed")
	}

	api, _ := entry.(*apiEntry)
	data, err := api.codec.ParseRequest(rawData)
	if err != nil {
		return nil, err
	}

	args := reflect.New(api.typ.Elem()).Interface()
	if len(data) > 0 {
		if err := json.Unmarshal(data, args); err != nil {
			return nil, err
		}
	}
	if err := c.ShouldBindHeader(args); err != nil {
		return nil, err
	}
	if err := c.ShouldBindQuery(args); err != nil {
		return nil, err
	}

	resp, err := api.h(c, args)
	return api.codec.ResponseError(resp, err)
}

// 分发HTTP请求
func dispatchAPI(c *Context) {
	log.Debugf("recv request method %s uri %s", c.Request.Method, c.Request.URL.Path)

	buf, err := handleAPI(c, c.Request.Method, c.Request.URL.Path)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	} else {
		c.JSON(http.StatusOK, json.RawMessage(buf))
	}
}
