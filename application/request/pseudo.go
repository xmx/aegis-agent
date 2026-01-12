package request

import (
	"strconv"

	"github.com/creack/pty"
)

type PseudoSize struct {
	Row uint16 `json:"row" query:"row"`
	Col uint16 `json:"col" query:"col"`
}

func (s PseudoSize) IsZero() bool {
	return s.Row == 0 && s.Col == 0
}

func (s PseudoSize) Winsize() *pty.Winsize {
	return &pty.Winsize{Rows: s.Row, Cols: s.Col}
}

func (s PseudoSize) String() string {
	x := strconv.FormatUint(uint64(s.Row), 10)
	y := strconv.FormatUint(uint64(s.Col), 10)

	return x + "x" + y
}
