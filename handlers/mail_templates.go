package handlers

import (
	"net/http"
	"strings"

	"fishing-platform-backend/config"
	"fishing-platform-backend/models"
	"github.com/gin-gonic/gin"
)

type mailTemplatePayload struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	IsHTML  *bool  `json:"is_html"`
}

func normalizeMailTemplate(payload mailTemplatePayload) (models.MailTemplate, string) {
	name := strings.TrimSpace(payload.Name)
	subject := strings.TrimSpace(payload.Subject)
	body := payload.Body
	if name == "" {
		return models.MailTemplate{}, "name is required"
	}
	if subject == "" {
		return models.MailTemplate{}, "subject is required"
	}
	if strings.TrimSpace(body) == "" {
		return models.MailTemplate{}, "body is required"
	}
	isHTML := true
	if payload.IsHTML != nil {
		isHTML = *payload.IsHTML
	}
	return models.MailTemplate{
		Name:    name,
		Subject: subject,
		Body:    body,
		IsHTML:  isHTML,
	}, ""
}

func GetMailTemplates(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var items []models.MailTemplate
	if err := scopeByOwner(db, user).Order("updated_at DESC").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list mail templates"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func GetMailTemplate(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var item models.MailTemplate
	if err := scopeByOwner(db, user).First(&item, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mail template not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

func CreateMailTemplate(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	var payload mailTemplatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}
	row, errMsg := normalizeMailTemplate(payload)
	if errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}
	row.CreatedBy = user.ID
	if err := config.GetDB().Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create mail template"})
		return
	}
	c.JSON(http.StatusCreated, row)
}

func UpdateMailTemplate(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var item models.MailTemplate
	if err := scopeByOwner(db, user).First(&item, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mail template not found"})
		return
	}
	var payload mailTemplatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}
	next, errMsg := normalizeMailTemplate(payload)
	if errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}
	item.Name = next.Name
	item.Subject = next.Subject
	item.Body = next.Body
	item.IsHTML = next.IsHTML
	if err := db.Save(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update mail template"})
		return
	}
	c.JSON(http.StatusOK, item)
}

func DeleteMailTemplate(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var item models.MailTemplate
	if err := scopeByOwner(db, user).First(&item, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mail template not found"})
		return
	}
	if err := db.Delete(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete mail template"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
