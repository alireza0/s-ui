package vmess

import (
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
)

func (h *Inbound) UpdateUsers(users []option.VMessUser) error {
	return h.service.UpdateUsers(common.Map(users, func(it option.VMessUser) string {
		return it.Name
	}), common.Map(users, func(it option.VMessUser) string {
		return it.UUID
	}), common.Map(users, func(it option.VMessUser) int {
		return it.AlterId
	}))
}
