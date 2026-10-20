package service

import "github.com/mhsanaei/3x-ui/v3/internal/database/model"

// CheckPortConflict runs the AddInbound port check up front so EasyBox can pick
// a free port; "" means free. Kept in its own file to stay clear of upstream edits.
func (s *InboundService) CheckPortConflict(inbound *model.Inbound) (string, error) {
	conflict, err := s.checkPortConflict(inbound, 0)
	if err != nil || conflict == nil {
		return "", err
	}
	return conflict.String(), nil
}
