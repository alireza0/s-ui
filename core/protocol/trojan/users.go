package trojan

import (
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
)

func (h *Inbound) UpdateUsers(users []option.TrojanUser) error {
	return h.service.UpdateUsers(common.Map(users, func(it option.TrojanUser) string {
		return it.Name
	}), common.Map(users, func(it option.TrojanUser) string {
		return it.Password
	}))
}
