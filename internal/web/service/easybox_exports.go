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

// SetRemarkTemplate and SetSubClashEnable let EasyBox apply its one-time
// subscription defaults (EasyBox.md §23.1, §25) without the settings form.
func (s *SettingService) SetRemarkTemplate(template string) error {
	return s.setString("remarkTemplate", template)
}

func (s *SettingService) SetSubClashEnable(enable bool) error {
	return s.setBool("subClashEnable", enable)
}
