package cmd

// 2017-11-13
// 服务器内部请求增加身份验证，第一个数据包数据为Sign校验串

import (
	"context"
	"github.com/guogeer/husky/log"
	"github.com/guogeer/husky/util"
	"io"
	"net"
	"time"
)

const (
	ServerRouter = "router"
)

type Server struct {
	Addr string
}

func (srv *Server) Serve(l net.Listener) error {
	defer l.Close()
	var tempDelay time.Duration
	for {
		rwc, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0

		ssid := util.GUID()
		c := &ServeConn{
			server: srv,
			TCPConn: &TCPConn{
				ssid: ssid,
				rwc:  rwc,
				send: make(chan []byte, 32<<10),
			},
		}
		// log.Info("create guid", ssid)
		addSession(&Session{Id: ssid, Out: c})
		go c.serve()
	}
}

func (srv *Server) ListenAndServe() error {
	addr := srv.Addr
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %v", err)
	}
	return srv.Serve(l)
}

func ListenAndServe(addr string) error {
	srv := &Server{Addr: addr}
	return srv.ListenAndServe()
}

type ServeConn struct {
	server *Server
	*TCPConn
}

// 网络连接异常关闭后，优先通知主逻辑Goroutine，写Goroutine收到回复后
// 继续读取写队列，缓存回收完毕后，关闭写队列，关闭写Goroutine
func (c *ServeConn) serve() {
	doneCtx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer func() {
			ticker.Stop() // 关闭定时器
			// 关闭网络连接
			c.rwc.Close()
			// 当前上下文
			ctx := &Context{Ssid: c.ssid, Out: c}
			defaultCmdSet.Handle(ctx, "CMD_Close", nil)
			defaultCmdSet.Handle(ctx, "FUNC_Close", nil)

			// 删除会话
			removeSession(c.ssid)
		}()

		for {
			select {
			case buf, ok := <-c.send:
				if ok == false {
					return
				}
				if _, err := c.writeMsg(RawMessage, buf); err != nil {
					log.Debugf("write %v", err)
					return
				}
			case <-ticker.C: // heart beat
				if _, err := c.writeMsg(PingMessage, nil); err != nil {
					return
				}

			case <-doneCtx.Done():
				return
			}
		}
	}()

	// 读关闭通知
	defer cancel()
	c.rwc.SetReadDeadline(time.Now().Add(2 * time.Second))

	var isFirstPackage = true
	for {
		mt, buf, err := c.TCPConn.ReadMessage()
		if err != nil {
			if err != io.EOF {
				log.Error(err)
			}
			return
		}
		if isFirstPackage == true {
			if _, err = defaultAuthParser.Decode(buf); err != nil {
				return
			}
		}
		if isFirstPackage || mt == PongMessage {
			c.rwc.SetReadDeadline(time.Now().Add(pongWait))
		}

		isFirstPackage = false
		if mt == RawMessage {
			pkg, err := defaultRawParser.Decode(buf)
			if err != nil {
				return
			}

			id, ssid, data := pkg.Id, pkg.Ssid, pkg.Data
			err = defaultCmdSet.Handle(&Context{Out: c, Ssid: ssid}, id, data)
			if err != nil {
				log.Debug(err)
			}
		}
	}
}
