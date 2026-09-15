package hysteria2

import (
	"github.com/alireza0/s-ui/core/usersession"

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

// CloseUserSessions cuts the sessions of users no longer enabled on this
// inbound. Updating the user list only rejects new sessions: an already
// authenticated client keeps being served until its session is cut.
func (h *Inbound) CloseUserSessions(keep map[string]struct{}) int {
	return h.sessions.CloseUsers(keep)
}

// KickUserSessions cuts the sessions of one user on demand, without locking the
// user out: they may connect again immediately.
func (h *Inbound) KickUserSessions(user string) int {
	return h.sessions.KickUserSessions(user)
}

var _ usersession.Closer = (*Inbound)(nil)
