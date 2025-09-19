package crontab

import (
	"github.com/robfig/cron/v3"
)

type Tasker interface {
	Spec() cron.Schedule
	Call()
}
