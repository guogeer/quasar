package cmd

import (
	"context"
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
				pong: make(chan bool, 1),
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
			c.Close() // 关闭网络连接

			RemoveSession(c.ssid) // 删除会话
			defaultCmdSet.Handle(&Context{Ssid: c.ssid, Out: c}, "FUNC_Close", nil)
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
			case <-c.pong:
				if _, err := c.writeMsg(PongMessage, []byte{}); err != nil {
					return
				}
			case <-doneCtx.Done():
				return
			}
		}
	}()

	defer func() {
		cancel() // 读关闭通知
		if c.pong != nil {
			close(c.pong)
		}
	}()
	// 新连接5s内未收到有效数据判定无效
	c.rwc.SetReadDeadline(time.Now().Add(5 * time.Second))

	for needAuth := true; true; needAuth = false {
		mt, buf, err := c.TCPConn.ReadMessage()
		if err != nil {
			// log.Debug(err)
			return
		}

		// 第一个包校验数据安全
		parser := rawParser
		if needAuth {
			mt, parser = RawMessage, authParser
			c.rwc.SetReadDeadline(time.Now().Add(pongWait))
		}
		if mt == RawMessage {
			pkg, err := parser.Decode(buf)
			if err != nil {
				log.Debug(err)
				return
			}
			// 忽略校验包空数据
			if pkg.Id == "" && needAuth {
				continue
			}

			ctx := &Context{
				Out:        c,
				Ssid:       pkg.Ssid,
				ServerName: pkg.ServerName,
				ClientAddr: pkg.ClientAddr,
			}
			if err := defaultCmdSet.Handle(ctx, pkg.Id, pkg.Data); err != nil {
				log.Debugf("handle msg[%s] error: %v", buf, err)
			}
		}
	}
}
