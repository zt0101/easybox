package easybox

import (
	"errors"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// PortRange is inclusive; users open it once in firewall / cloud security
// group (EasyBox.md §9.2), so lines never land outside it.
type PortRange struct {
	Min int
	Max int
}

var DefaultPortRange = PortRange{Min: 20000, Max: 20099}

var ErrNoFreePort = errors.New("no free port left in the EasyBox port range")

// pickPort scans the range from a random offset with wrap-around, skipping
// reserved ports and anything upstream's own conflict check rejects.
func pickPort(ib *model.Inbound, r PortRange, reserved map[int]bool, offset func(int) int) (int, error) {
	if offset == nil {
		offset = random.Num
	}
	size := r.Max - r.Min + 1
	start := offset(size)
	inbounds := &service.InboundService{}
	for i := range size {
		port := r.Min + (start+i)%size
		if reserved[port] {
			continue
		}
		ib.Port = port
		conflict, err := inbounds.CheckPortConflict(ib)
		if err != nil {
			ib.Port = 0
			return 0, err
		}
		if conflict == "" {
			return port, nil
		}
	}
	ib.Port = 0
	return 0, ErrNoFreePort
}
