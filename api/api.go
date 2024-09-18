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

func (group *Group) Add(method, uri string, h Handler, i any) {
	apiEntries.Store(merge(method, group.basePath+uri), &apiEntry{h: h, typ: reflect.TypeOf(i), codec: group.codec})
	group.route.Handle(method, uri, dispatchAPI)
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

	if err := r.Run(addr); err != nil {
		log.Fatalf("start gin server fail, %v", err)
	}
}
