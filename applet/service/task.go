package service

import (
	"context"
	"log/slog"

	"github.com/xmx/aegis-common/jsos/jstask"
)

func NewTask(mana jstask.Manager, log *slog.Logger) *Task {
	return &Task{
		mana: mana,
		log:  log,
	}
}

type Task struct {
	mana jstask.Manager
	log  *slog.Logger
}

func (tsk *Task) Tasks() []jstask.Tasker {
	return tsk.mana.Tasks()
}

func (tsk *Task) Find(pid uint64) jstask.Tasker {
	return tsk.mana.Find(pid)
}

func (tsk *Task) Exec(name, code string) {
	go tsk.mana.Exec(context.Background(), name, code)
	tsk.log.Info("任务已执行", "name", name)
}
