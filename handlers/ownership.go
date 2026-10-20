package handlers

import (
	"net/http"

	"spearhook/middleware"
	"spearhook/models"
	"spearhook/platform"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func currentUserOrAbort(c *gin.Context) (models.User, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return models.User{}, false
	}
	return user, true
}

func scopeByOwner(db *gorm.DB, user models.User) *gorm.DB {
	return platform.ScopeByOwner(db, user)
}

func canAccessOwned(user models.User, createdBy uint) bool {
	return platform.CanAccessOwned(user, createdBy)
}

func denyUnlessOwned(c *gin.Context, createdBy uint) bool {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return false
	}
	if canAccessOwned(user, createdBy) {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
	return false
}

func ensureProjectAccess(c *gin.Context, project models.Project) bool {
	return denyUnlessOwned(c, project.CreatedBy)
}

func ensureOwnedRobotIDs(c *gin.Context, db *gorm.DB, user models.User, robotIDs []uint) bool {
	if user.Role.IsAdmin() || len(robotIDs) == 0 {
		return true
	}
	var count int64
	if err := db.Model(&models.Robot{}).
		Where("id IN ? AND created_by = ?", robotIDs, user.ID).
		Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate robots"})
		return false
	}
	if int(count) != len(robotIDs) {
		c.JSON(http.StatusForbidden, gin.H{"error": "One or more robots are not accessible"})
		return false
	}
	return true
}

func robotIDsFromModels(robots []models.Robot) []uint {
	ids := make([]uint, 0, len(robots))
	for _, robot := range robots {
		ids = append(ids, robot.ID)
	}
	return ids
}
