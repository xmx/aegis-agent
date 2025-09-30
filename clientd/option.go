package clientd

import "net/http"

type Identifier interface {
	MachineID(rebuild bool) string
}

type option struct {
	identifier Identifier
	server     *http.Server
}

type OptionFunc func(option) option

func WithIdentifier(id Identifier) OptionFunc {
	return func(o option) option {
		o.identifier = id
		return o
	}
}

func WithHTTPServer(s *http.Server) OptionFunc {
	return func(o option) option {
		o.server = s
		return o
	}
}

func WithHTTPHandler(h http.Handler) OptionFunc {
	return func(o option) option {
		o.server = &http.Server{Handler: h}
		return o
	}
}

func WithLogger() {

}
