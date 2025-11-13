package restapi

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/xgfone/ship/v5"
	"github.com/xmx/aegis-agent/application/errcode"
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
	ret := make(response.Tasks, 0, len(tasks))
	for _, tk := range tasks {
		pid, name, status := tk.PID(), tk.Name(), tk.Status()
		ele := &response.Task{PID: pid, Name: name, Status: status}
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
	req := new(request.TaskPID)
	if err := c.BindQuery(req); err != nil {
		return err
	}
	if task := tsk.svc.Find(req.PID); task != nil {
		task.Kill("remote killed")
	} else {
		return errcode.FmtTaskNotExists.Fmt(req.PID)
	}

	return nil
}

func (tsk *Task) attach(c *ship.Context) error {
	w, r := c.Response(), c.Request()
	ws, err := tsk.wsu.Upgrade(w, r, nil)
	if err != nil {
		c.Errorf("websocket upgrade error", "error", err)
		return err
	}
	defer ws.Close()

	wsout := &playWriter{channel: "stdout", socket: ws}
	wserr := &playWriter{channel: "stderr", socket: ws}

	req := new(request.TaskPID)
	if err = c.BindQuery(req); err != nil {
		_ = wserr.writeError(err)
		return nil
	}
	proc := tsk.svc.Find(req.PID)
	if proc == nil {
		_ = wserr.writeError(errors.New("process not found"))
		return nil
	}

	stdout, stderr := proc.Engineer().Output()
	stdout.Attach(wsout)
	stderr.Attach(wserr)
	defer func() {
		stdout.Detach(wsout)
		stderr.Detach(wserr)
	}()

	ctx := proc.Engineer().Context()
	context.AfterFunc(ctx, func() {
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
	socket  *websocket.Conn
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
