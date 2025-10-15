package restapi

import (
	"bufio"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xgfone/ship/v5"
)

func NewEcho() *Echo {
	return &Echo{
		wsu: &websocket.Upgrader{
			HandshakeTimeout: 10 * time.Second,
			CheckOrigin:      func(r *http.Request) bool { return true },
		},
	}
}

type Echo struct {
	wsu *websocket.Upgrader
}

func (ech *Echo) RegisterRoute(r *ship.RouteGroupBuilder) error {
	r.Route("/echo/chat").GET(ech.chat)

	return nil
}

func (ech *Echo) chat(c *ship.Context) error {
	w, r := c.Response(), c.Request()
	cli, err := ech.wsu.Upgrade(w, r, nil)
	if err != nil {
		c.Errorf("websocket 升级失败", "error", err)
		return nil
	}
	defer cli.Close()

	c.Infof("websocket 升级成功")

	lis, err := net.Listen("tcp", ":9900")
	if err != nil {
		return nil
	}
	defer lis.Close()

	conn, err := lis.Accept()
	if err != nil {
		return nil
	}
	defer conn.Close()

	rd := bufio.NewReader(conn)
	for {
		line, _, err := rd.ReadLine()
		if err != nil {
			break
		}

		if err = cli.WriteMessage(websocket.TextMessage, line); err != nil {
			conn.Close()
			break
		}
	}

	//for {
	//	mt, msg, exx := cli.ReadMessage()
	//	if exx != nil {
	//		c.Errorf("读取消息错误", "error", exx)
	//		break
	//	}
	//	c.Infof("[broker receive message] >>> ", "message", string(msg))
	//	msg = append(msg, []byte(" 响应测试")...)
	//	if err = cli.WriteMessage(mt, msg); err != nil {
	//		break
	//	}
	//}
	c.Warnf("websocket disconnect")

	return nil
}
