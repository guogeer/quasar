package cmd

// 2017-11-13
// 服务器内部请求增加身份验证，第一个数据包数据为Sign校验串

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/guogeer/quasar/log"
	"github.com/guogeer/quasar/util"
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
		AddSession(&Session{Id: ssid, Out: c})
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

func (c *ServeConn) serve() {
	doneCtx, cancel := context.WithCancel(context.Background())
	go func() {
		defer func() {
			// 关闭网络连接
			c.rwc.Close()
			// 当前上下文
			ctx := &Context{Ssid: c.ssid, Out: c}
			defaultCmdSet.Handle(ctx, "CMD_Close", nil)
			defaultCmdSet.Handle(ctx, "FUNC_Close", nil)

			// 删除会话
			RemoveSession(c.ssid)
		}()

		for {
			select {
			case buf, ok := <-c.send:
				if !ok {
					return
				}
				// 忽略过大消息
				if _, err := c.writeMsg(RawMessage, buf); err != nil {
					log.Debugf("write %v", err)
					if err != errTooLargeMessage {
						return
					}
				}
				saveBuf(buf)
			case <-doneCtx.Done():
				return
			}
		}
	}()

	// 读关闭通知
	defer cancel()
	// 新连接5s内未收到有效数据判定无效
	c.rwc.SetReadDeadline(time.Now().Add(5 * time.Second))

	var isAuth bool
	for {
		mt, buf, err := c.TCPConn.ReadMessage()
		if err != nil {
			if err != io.EOF {
				log.Debug(err)
			}
			return
		}
		if !isAuth {
			if _, err = defaultAuthParser.Decode(buf); err != nil {
				return
			}
		}
		if mt == PingMessage {
			c.rwc.SetReadDeadline(time.Now().Add(pongWait))
		}

		isAuth = true
		if mt == RawMessage {
			pkg, err := defaultRawParser.Decode(buf)
			if err != nil {
				return
			}

			ctx := &Context{
				Out:        c,
				Ssid:       pkg.Ssid,
				ServerName: pkg.ServerName,
				ClientAddr: pkg.ClientAddr,
			}
			err = defaultCmdSet.Handle(ctx, pkg.Id, pkg.Data)
			if err != nil {
				log.Debugf("handle msg[%s] error: %v", buf, err)
			}
		}
		saveBuf(buf)
	}
}
