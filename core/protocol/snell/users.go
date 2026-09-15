package snell

import (
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

// UpdateUsers swaps the user list of a running multi-user inbound, so adding or
// removing a client does not need the inbound to be torn down and rebuilt.
//
// Snell authenticates every request, including the ones that follow on a reused
// connection, so a removed user is locked out as soon as the service is
// updated; no session has to be cut beyond the connections already in flight.
func (h *Inbound) UpdateUsers(users []option.SnellUser) error {
	if h.multiService == nil {
		return E.New("snell: inbound is not multi-user")
	}
	userList := make([]int, len(users))
	keyList := make([][]byte, len(users))
	for index, user := range users {
		userList[index] = index
		keyList[index] = []byte(user.UserKey)
	}
	// Store first: the service is what hands out the indexes read from this
	// list, so it must never point past the list a connection can see.
	h.users.Store(&users)
	err := h.multiService.UpdateUsers(userList, keyList)
	if err != nil {
		return err
	}
	return nil
}
