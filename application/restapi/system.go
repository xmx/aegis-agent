package restapi

import (
	"net/http"

	"github.com/xgfone/ship/v5"
)

func NewSystem() *System {
	return &System{}
}

type System struct{}

func (s *System) RegisterRoute(r *ship.RouteGroupBuilder) error {
	r.Route("/system/ping").GET(s.ping)
	return nil
}

func (s *System) ping(c *ship.Context) error {
	return c.NoContent(http.StatusNoContent)
}
