package anytls

import (
	"github.com/alireza0/s-ui/core/usersession"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"

	anytls "github.com/anytls/sing-anytls"
)

func (h *Inbound) UpdateUsers(users []option.AnyTLSUser) error {
	h.service.UpdateUsers(common.Map(users, func(it option.AnyTLSUser) anytls.User {
		return (anytls.User)(it)
	}))
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
