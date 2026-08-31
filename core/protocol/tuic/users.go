package tuic

import (
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/gofrs/uuid/v5"
)

func (h *Inbound) UpdateUsers(users []option.TUICUser) error {
	userList := make([]string, 0, len(users))
	userUUIDList := make([][16]byte, 0, len(users))
	userPasswordList := make([]string, 0, len(users))
	for index, user := range users {
		if user.UUID == "" {
			return E.New("missing uuid for user ", index)
		}
		userUUID, err := uuid.FromString(user.UUID)
		if err != nil {
			return E.Cause(err, "invalid uuid for user ", index)
		}
		userList = append(userList, user.Name)
		userUUIDList = append(userUUIDList, userUUID)
		userPasswordList = append(userPasswordList, user.Password)
	}
	h.server.UpdateUsers(userList, userUUIDList, userPasswordList)
	return nil
}
