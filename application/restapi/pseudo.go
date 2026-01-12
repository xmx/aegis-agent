package restapi

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xgfone/ship/v5"
	"github.com/xmx/aegis-agent/application/request"
	"github.com/xmx/aegis-agent/application/service"
)

func NewPseudo(svc *service.Pseudo) *Pseudo {
	wsu := &websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		CheckOrigin:      func(r *http.Request) bool { return true },
	}

	return &Pseudo{
		svc: svc,
		wsu: wsu,
	}
}

type Pseudo struct {
	svc *service.Pseudo
	wsu *websocket.Upgrader
}

func (psd *Pseudo) RegisterRoute(r *ship.RouteGroupBuilder) error {
	r.Route("/pseudo/tty").GET(psd.tty)

	return nil
}

//goland:noinspection GoUnhandledErrorResult
func (psd *Pseudo) tty(c *ship.Context) error {
	req := new(request.PseudoSize)
	if err := c.BindQuery(req); err != nil {
		return err
	}

	w, r := c.Response(), c.Request()
	ws, err := psd.wsu.Upgrade(w, r, nil)
	if err != nil {
		c.Errorf("websocket 升级错误", "error", err)
		return nil
	}
	defer ws.Close()

	if err = psd.svc.TTY(ws, req); err != nil {
		c.Errorf("终端运行错误", "error", err)
	}

	return nil
}
