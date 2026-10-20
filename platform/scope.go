// Package platform holds the shared business primitives used by both the HTTP
// handlers and the chat tool layer. Keeping them here lets chattools wrap real
// platform behaviour without importing handlers (which would create a cycle).
package platform

import (
	"spearhook/models"

	"gorm.io/gorm"
)

// ScopeByOwner restricts queries to the user's own rows unless the user is an admin.
func ScopeByOwner(db *gorm.DB, user models.User) *gorm.DB {
	if user.Role.IsAdmin() {
		return db
	}
	return db.Where("created_by = ?", user.ID)
}

// CanAccessOwned reports whether the user may access a row created by createdBy.
func CanAccessOwned(user models.User, createdBy uint) bool {
	return user.Role.IsAdmin() || createdBy == user.ID
}
