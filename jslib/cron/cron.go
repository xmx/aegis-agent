package cron

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/sobek"
	"github.com/robfig/cron/v3"
	"github.com/xmx/aegis-common/jsos/jsvm"
)

func New() jsvm.Module {
	return new(cronLoad)
}

type cronLoad struct {
	eng jsvm.Engineer
}

func (cl *cronLoad) Preload(eng jsvm.Engineer) (string, any, bool) {
	cl.eng = eng

	vals := map[string]any{
		"secondOptional": cron.SecondOptional,
		"second":         cron.Second,
		"minute":         cron.Minute,
		"hour":           cron.Hour,
		"dom":            cron.Dom,
		"month":          cron.Month,
		"dow":            cron.Dow,
		"descriptor":     cron.Descriptor,
		"new":            cl.newCron,
		"withSeconds":    cron.WithSeconds,
		"withLocation":   cron.WithLocation,
		"withParser":     cron.WithParser,
		"newParser":      cron.NewParser,
		"every":          cron.Every,
		"parseStandard":  cron.ParseStandard,
	}

	return "github.com/robfig/cron/v3", vals, true
}

func (cl *cronLoad) newCron(opts ...cron.Option) (*sobek.Object, error) {
	mod := &cronModule{eng: cl.eng, done: make(chan struct{})}

	opts = append(opts, cron.WithChain(mod.serializer)) // 防止并发 panic
	ctab := cron.New(opts...)
	mod.ctab = ctab

	final := cl.eng.Finalizer()
	final.Add(mod.stop)

	vm := cl.eng.Runtime()
	obj := vm.NewObject()
	if err := obj.Set("start", mod.start); err != nil {
		return nil, err
	}
	if err := obj.Set("stop", mod.stop); err != nil {
		return nil, err
	}
	if err := obj.Set("addJob", mod.addJob); err != nil {
		return nil, err
	}
	if err := obj.Set("schedule", mod.schedule); err != nil {
		return nil, err
	}
	if err := obj.Set("remove", mod.remove); err != nil {
		return nil, err
	}
	if err := obj.Set("entries", mod.entries); err != nil {
		return nil, err
	}
	if err := obj.Set("wait", mod.wait); err != nil {
		return nil, err
	}

	return obj, nil
}

type cronModule struct {
	eng     jsvm.Engineer
	ctab    *cron.Cron
	done    chan struct{}
	closed  atomic.Bool
	mutex   sync.Mutex
	running bool
}

func (cl *cronModule) start() {
	cl.ctab.Start()
}

func (cl *cronModule) stop() {
	if !cl.closed.CompareAndSwap(false, true) {
		return
	}
	cl.ctab.Stop()
	close(cl.done)
}

func (cl *cronModule) addJob(spec string, cmd func()) (cron.EntryID, error) {
	return cl.ctab.AddJob(spec, cron.FuncJob(cmd))
}

func (cl *cronModule) schedule(sched cron.Schedule, cmd func()) cron.EntryID {
	return cl.ctab.Schedule(sched, cron.FuncJob(cmd))
}

func (cl *cronModule) remove(id cron.EntryID) {
	cl.ctab.Remove(id)
}

func (cl *cronModule) wait(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	parent := cl.eng.Context()

	select {
	case <-parent.Done():
	case <-ctx.Done():
	case <-cl.done:
	}
}

func (cl *cronModule) entries() []*entry {
	ents := cl.ctab.Entries()
	result := make([]*entry, 0, len(ents))
	for _, ent := range ents {
		result = append(result, &entry{
			ID:   ent.ID,
			Next: ent.Next,
			Prev: ent.Prev,
		})
	}

	return result
}

func (cl *cronModule) serializer(job cron.Job) cron.Job {
	return cron.FuncJob(func() {
		cl.mutex.Lock()
		defer cl.mutex.Unlock()

		job.Run()
	})
}

type entry struct {
	ID   cron.EntryID `json:"id"`
	Next time.Time    `json:"next"`
	Prev time.Time    `json:"prev"`
}
