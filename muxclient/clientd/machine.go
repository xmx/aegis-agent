package clientd

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"os"
	"strings"

	"github.com/xmx/aegis-agent/machine"
)

func NewIdent(storeFile string) Identifier {
	return &machineIdent{
		f: storeFile,
	}
}

type machineIdent struct {
	f string
}

func (m *machineIdent) MachineID(rebuild bool) string {
	if !rebuild { // 从缓存读取
		if id := m.raed(); id != "" {
			return id
		}
	}

	id := m.generate()
	if id == "" {
		buf := make([]byte, 20)
		_, _ = rand.Read(buf)
		id = hex.EncodeToString(buf)
	}
	m.store(id)

	return id
}

func (m *machineIdent) raed() string {
	if m.f == "" {
		return ""
	}

	b, err := os.ReadFile(m.f)
	if err != nil {
		return ""
	}

	return strings.TrimSuffix(string(b), "\n")
}

func (m *machineIdent) store(id string) {
	if m.f != "" {
		_ = os.WriteFile(m.f, []byte(id), 0600)
	}
}

func (m *machineIdent) generate() string {
	mid, _ := machine.ID()
	if mid == "" {
		buf := make([]byte, 20)
		_, _ = rand.Read(buf)
		mid = hex.EncodeToString(buf)
	}
	sum := sha1.Sum([]byte(mid))

	return hex.EncodeToString(sum[:])
}
