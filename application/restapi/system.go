package restapi

import (
	"image/png"
	"mime"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xgfone/ship/v5"
	"github.com/xmx/aegis-agent/application/request"
	"github.com/xmx/aegis-agent/application/service"
)

func NewSystem(svc *service.System) *System {
	wsu := &websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		CheckOrigin:      func(r *http.Request) bool { return true },
	}

	return &System{
		svc: svc,
		wsu: wsu,
	}
}

type System struct {
	svc *service.System
	wsu *websocket.Upgrader
}

func (syst *System) RegisterRoute(r *ship.RouteGroupBuilder) error {
	r.Route("/system/ping").GET(syst.ping)
	r.Route("/system/tty").GET(syst.tty)
	r.Route("/system/screenshot").GET(syst.screenshot)
	r.Route("/system/download").GET(syst.download)
	return nil
}

func (syst *System) ping(c *ship.Context) error {
	return c.NoContent(http.StatusNoContent)
}

//goland:noinspection GoUnhandledErrorResult
func (syst *System) tty(c *ship.Context) error {
	req := new(request.SystemTTYSize)
	if err := c.BindQuery(req); err != nil {
		return err
	}

	w, r := c.Response(), c.Request()
	ws, err := syst.wsu.Upgrade(w, r, nil)
	if err != nil {
		c.Errorf("websocket 升级错误", "error", err)
		return nil
	}
	defer ws.Close()

	if err = syst.svc.TTY(ws, req); err != nil {
		c.Errorf("终端运行错误", "error", err)
	}

	return nil
}

func (syst *System) screenshot(c *ship.Context) error {
	imgs, err := syst.svc.Screenshot()
	if err != nil {
		return err
	}

	contentType := mime.TypeByExtension(".png")
	if contentType == "" {
		contentType = ship.MIMEOctetStream
	}

	params := map[string]string{"filename": "screenshot-0.png"}
	disposition := mime.FormatMediaType("inline", params)
	c.SetContentType(contentType)
	c.SetRespHeader(ship.HeaderContentDisposition, disposition)

	return png.Encode(c, imgs[0])
}

func (syst *System) download(c *ship.Context) error {
	return c.Attachment("/Users/wang/Downloads/30G.bin", "")
}
