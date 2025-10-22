package api

import (
	"errors"
	"net/http"
	"reflect"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/guogeer/quasar/v2/log"
)

type Context = gin.Context
type IRoutes = gin.IRoutes

type Handler func(*Context, any) (any, error)

type apiEntry struct {
	h              Handler
	typ            reflect.Type
	requestReader  RequestReader
	responseWriter ResponseWriter
}

type ResponseResult struct {
	Data  any
	Error error
}

type RequestReader interface {
	ReadRequest(*Context, any) error
}

type apiError struct {
	Code string `json:"code,omitempty"`
	Msg  string `json:"msg,omitempty"`
}

type apiCodec struct{}

var validator = binding.Validator

func init() {
	binding.Validator = nil
}

// NOTE 屏蔽了gin的默认validator，期望先解析出数据再校验数据有效性
func BindAll(c *Context, args any) error {
	if err := c.ShouldBindHeader(args); err != nil {
		return err
	}
	if err := c.ShouldBindQuery(args); err != nil {
		return err
	}
	if err := c.ShouldBindUri(args); err != nil {
		return err
	}
	// GET请求可访问JSON等
	b := binding.Default("POST", c.ContentType())
	if err := c.ShouldBindWith(args, b); err != nil {
		return err
	}
	return validator.ValidateStruct(args)
}

func (r *apiCodec) ReadRequest(c *Context, args any) error {
	return BindAll(c, args)
}

func (r *apiCodec) WriteResponse(c *Context, result ResponseResult) {
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, apiError{Code: "system_error", Msg: result.Error.Error()})
	} else {
		c.JSON(http.StatusOK, result.Data)
	}
}

var defaultCodec = &apiCodec{}

type ResponseWriter interface {
	WriteResponse(*Context, ResponseResult)
}

func merge(method, uri string) string {
	return method + " " + uri
}

var apiEntries sync.Map

type Group struct {
	basePath       string
	route          IRoutes
	requestReader  RequestReader
	responseWriter ResponseWriter
}

func NewGroup(basePath string, route IRoutes) *Group {
	return &Group{basePath: basePath, route: route, requestReader: defaultCodec, responseWriter: defaultCodec}
}

func (group *Group) Add(method, uri string, h Handler, args any) {
	group.Handle(method, uri, h, args)
}

func (group *Group) Handle(method, uri string, h Handler, args any) {
	apiEntries.Store(merge(method, group.basePath+uri), &apiEntry{
		h: h, typ: reflect.TypeOf(args), requestReader: group.requestReader, responseWriter: group.responseWriter,
	})
	group.route.Handle(method, uri, dispatchAPI)
}

func (group *Group) SetRequestReader(r RequestReader) {
	group.requestReader = r
}

func (group *Group) SetResponseWriter(w ResponseWriter) {
	group.responseWriter = w
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

func handleRequest(c *Context, method, uri string) (ResponseWriter, any, error) {
	id := merge(method, uri)
	entry, ok := apiEntries.Load(id)
	if !ok {
		return nil, nil, errors.New("handle url " + id + " is not existed")
	}

	api, _ := entry.(*apiEntry)
	args := reflect.New(api.typ.Elem()).Interface()
	if err := api.requestReader.ReadRequest(c, args); err != nil {
		return api.responseWriter, nil, err
	}
	data, err := api.h(c, args)
	return api.responseWriter, data, err
}

// 分发HTTP请求
func dispatchAPI(c *Context) {
	log.Debugf("recv request method %s uri %s", c.Request.Method, c.Request.URL.Path)
	codec, data, err := handleRequest(c, c.Request.Method, c.Request.URL.Path)
	if codec == nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	} else {
		codec.WriteResponse(c, ResponseResult{Data: data, Error: err})

	}
}
