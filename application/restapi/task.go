package restapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/xgfone/ship/v5"
	"github.com/xmx/aegis-agent/application/request"
	"github.com/xmx/aegis-agent/application/response"
	"github.com/xmx/aegis-agent/application/service"
	"github.com/xmx/aegis-common/library/httpkit"
)

func NewTask(svc *service.Task) *Task {
	return &Task{
		svc: svc,
		wsu: httpkit.NewWebsocketUpgrader(),
	}
}

type Task struct {
	svc *service.Task
	wsu *websocket.Upgrader
}

func (tsk *Task) RegisterRoute(r *ship.RouteGroupBuilder) error {
	r.Route("/tasks").GET(tsk.list)
	r.Route("/task/exec").POST(tsk.exec)
	r.Route("/task/kill").DELETE(tsk.kill)
	r.Route("/task/attach").GET(tsk.attach)

	return nil
}

func (tsk *Task) list(c *ship.Context) error {
	tasks := tsk.svc.Tasks()

	ret := make([]*response.Task, 0, len(tasks))
	for _, tk := range tasks {
		name, code, sha1sum := tk.Info()
		ele := &response.Task{
			Name:    name,
			Code:    code,
			SHA1:    sha1sum,
			StartAt: tk.StartAt(),
		}
		ret = append(ret, ele)
	}

	return c.JSON(http.StatusOK, ret)
}

func (tsk *Task) exec(c *ship.Context) error {
	req := new(request.TaskExec)
	if err := c.Bind(req); err != nil {
		return err
	}
	tsk.svc.Exec(req.Name, req.Code)

	return nil
}

func (tsk *Task) kill(c *ship.Context) error {
	req := new(request.QueryNames)
	if err := c.BindQuery(req); err != nil {
		return err
	}

	return tsk.svc.Kill(req.Get())
}

func (tsk *Task) attach(c *ship.Context) error {
	w, r := c.Response(), c.Request()
	ws, err := tsk.wsu.Upgrade(w, r, nil)
	if err != nil {
		c.Warnf("websocket upgrade error", "error", err)
		return err
	}
	defer ws.Close()

	sws := &safeWriter{ws: ws}
	wsout := &playWriter{channel: "stdout", socket: sws}
	wserr := &playWriter{channel: "stderr", socket: sws}

	req := new(request.QueryNames)
	if err = c.BindQuery(req); err != nil {
		_ = wserr.writeError(err)
		return nil
	}

	name := req.Get()
	attrs := []any{"name", name}
	proc := tsk.svc.Lookup(name)
	if proc == nil {
		c.Warnf("任务不存在", attrs...)
		_ = wserr.writeError(errors.New("进程不存在"))
		return nil
	}

	c.Infof("观测任务输出", attrs...)
	stdout, stderr := proc.Output()
	stdout.Append(wsout)
	stderr.Append(wserr)
	defer func() {
		stdout.Remove(wsout)
		stderr.Remove(wserr)
		c.Infof("退出观测", attrs...)
	}()

	_, _ = stderr.Write([]byte("进入任务：" + name))
	ctx := proc.Context()
	context.AfterFunc(ctx, func() {
		//msg := name + " 结束了"
		//if te := proc.Error(); te != nil {
		//	msg += ": " + te.Error()
		//}
		//_, _ = stderr.Write([]byte(msg))
		ws.Close()
	})

	for {
		_, rd, err := ws.NextReader()
		if err != nil {
			break
		}
		io.Copy(io.Discard, rd)
	}

	return nil
}

type playWriter struct {
	channel string
	socket  *safeWriter
}

func (pw *playWriter) Write(p []byte) (int, error) {
	n := len(p)
	data := &playData{Channel: pw.channel, Message: string(p)}
	if err := pw.socket.WriteJSON(data); err != nil {
		return 0, err
	}

	return n, nil
}

func (pw *playWriter) writeError(err error) error {
	_, err = pw.Write([]byte(err.Error()))
	return err
}

type playData struct {
	Channel string `json:"channel"`
	Message string `json:"message"`
}

type safeWriter struct {
	ws *websocket.Conn
	mu sync.Mutex
}

func (s *safeWriter) WriteJSON(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ws.WriteJSON(v)
}
