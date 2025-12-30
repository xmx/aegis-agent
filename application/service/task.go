package service

import (
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

func (tsk *Task) Tasks() jstask.Tasks {
	tasks := tsk.mana.Tasks()
	tasks.Sort()
	return tasks
}

func (tsk *Task) Lookup(name string) *jstask.Task {
	return tsk.mana.Lookup(name)
}

func (tsk *Task) Exec(name, code string) {
	go func() {
		attrs := []any{"name", name}
		tsk.log.Info("添加了一个任务", attrs...)
		execed, _, err := tsk.mana.Exec(nil, name, code)
		if err != nil {
			attrs = append(attrs, "error", err)
			tsk.log.Warn("任务执行出错", attrs...)
			return
		}
		if !execed {
			tsk.log.Info("任务无需重复运行", attrs...)
		}
		tsk.log.Info("任务执行完毕", attrs...)
	}()
}

func (tsk *Task) Kill(name string) error {
	return tsk.mana.Kill(name, &jstask.TaskError{Name: name, Text: "主动结束任务"})
}
