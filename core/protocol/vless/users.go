package vless

import (
	"github.com/alireza0/s-ui/core/usersession"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
)

func (h *Inbound) UpdateUsers(users []option.VLESSUser) error {
	h.service.UpdateUsers(common.Map(users, func(it option.VLESSUser) string {
		return it.Name
	}), common.Map(users, func(it option.VLESSUser) string {
		return it.UUID
	}), common.Map(users, func(it option.VLESSUser) string {
		return it.Flow
	}))
	return nil
}

// CloseUserSessions cuts the multiplex sessions of users no longer enabled on
// this inbound. Updating the user list only rejects new sessions: the streams
// of an already authenticated multiplex session are never re-checked.
func (h *Inbound) CloseUserSessions(keep map[string]struct{}) int {
	return h.sessions.CloseUsers(keep)
}

// KickUserSessions cuts the sessions of one user on demand, without locking the
// user out: they may connect again immediately.
func (h *Inbound) KickUserSessions(user string) int {
	return h.sessions.KickUserSessions(user)
}

var _ usersession.Closer = (*Inbound)(nil)
