package restapi

import (
	"net/http"

	"github.com/xgfone/ship/v5"
	"github.com/xmx/aegis-agent/client/tunnel"
)

func NewSystem(cli tunnel.Client) *System {
	return &System{}
}

type System struct{}

func (s *System) RegisterRoute(r *ship.RouteGroupBuilder) error {
	r.Route("/health/ping").GET(s.ping)
	return nil
}

func (s *System) ping(c *ship.Context) error {
	return c.NoContent(http.StatusNoContent)
}
