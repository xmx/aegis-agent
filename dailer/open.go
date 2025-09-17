package dailer

import (
	"time"

	"github.com/xmx/aegis-common/transport"
)

type Config struct {
	DialConfig transport.DialConfig
	Timeout    time.Duration
}

func (c Config) Open() {

}
