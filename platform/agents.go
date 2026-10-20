package platform

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"spearhook/models"
)

// AgentOfflineAfter is the heartbeat window before an agent is considered offline.
const AgentOfflineAfter = 60 * time.Second

// CurrentAgentStatus derives online/offline from the last heartbeat.
func CurrentAgentStatus(agent models.Agent, now time.Time) models.AgentStatus {
	if agent.LastSeenAt == nil || now.Sub(*agent.LastSeenAt) > AgentOfflineAfter {
		return models.AgentStatusOffline
	}
	return models.AgentStatusOnline
}

// GenerateURLToken returns a URL-safe random token with the given prefix.
func GenerateURLToken(prefix string, size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(buf), nil
}
