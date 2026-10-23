package handlers

import (
	"time"

	"spearhook/chattools/chatbus"

	"github.com/gin-gonic/gin"
)

// StreamChatEvents streams async job events (info gathering) for one owned
// session. The chat POST streams the current turn; this GET stream stays open so
// background jobs can report progress into the same conversation.
func StreamChatEvents(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	session, ok := loadOwnedChatSession(c, user)
	if !ok {
		return
	}
	emit, okFlush := beginChatSSE(c)
	if !okFlush {
		return
	}

	events, cancel := chatbus.Subscribe(session.ID)
	defer cancel()

	emit("ready", gin.H{"session": session.ID})
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case ev, open := <-events:
			if !open {
				return
			}
			emit(ev.Name, ev.Payload)
		case <-ticker.C:
			emit("ping", gin.H{})
		}
	}
}
