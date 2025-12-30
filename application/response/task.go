package response

import (
	"slices"
	"time"
)

type Process struct {
	PID       uint64    `json:"pid"`
	Name      string    `json:"name"`
	StartedAt time.Time `json:"started_at"`
}

type Processes []*Process

func (ps Processes) Sort() {
	slices.SortFunc(ps, func(a, b *Process) int {
		aid, bid := a.PID, b.PID
		if aid == bid {
			return 0
		} else if aid < bid {
			return -1
		}
		return 1
	})
}

type Task struct {
	Name    string    `json:"name"`
	Code    string    `json:"code"`
	SHA1    string    `json:"sha1"`
	StartAt time.Time `json:"start_at"`
}
