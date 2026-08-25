package hysteria2

import (
	"github.com/sagernet/sing-box/option"
)

func (h *Inbound) UpdateUsers(users []option.Hysteria2User) error {
	userList := make([]string, 0, len(users))
	userPasswordList := make([]string, 0, len(users))
	for _, user := range users {
		userList = append(userList, user.Name)
		userPasswordList = append(userPasswordList, user.Password)
	}
	h.service.UpdateUsers(userList, userPasswordList)
	return nil
}
