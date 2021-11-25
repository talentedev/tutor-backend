package intercom

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/services/intercom"
)

func newApplicationHandler(c *gin.Context) {
	data := &intercom.Contact{
		Role: intercom.RoleLead,
	}
	err := c.BindJSON(&data)
	if err != nil {
		logger.GetCtx(c).Errorf("Error reading request data: %v", err)
		c.JSON(http.StatusBadRequest, nil)
		return
	}
	res := intercom.SearchContact(data.Email)
	var contact *intercom.Contact
	if res != nil && res.TotalCount > 0 {
		contact = &res.Data[0]
		if contact.Id != "" {
			logger.GetCtx(c).Infof("Found contact %s\n", contact.Id)
		}
	} else {
		contact = intercom.CreateContact(data, "tutors")
		if contact != nil && contact.Id != "" {
			logger.GetCtx(c).Infof("Created lead contact %s\n", contact.Id)
		}
	}

	if contact.Id != "" {
		tag := intercom.GetTag("apply1")
		if tag != nil {
			for _, t := range contact.Tags.Data {
				if t.Id == tag.Id {
					logger.GetCtx(c).Debug("Contact is already tagged")
					c.JSON(http.StatusOK, nil)
					return
				}
			}

			tag = intercom.TagContact(tag.Id, contact.Id)
			if tag != nil {
				logger.GetCtx(c).Infof("Added tag %s to contact %s\n", tag.Name, contact.Id)
				c.JSON(http.StatusOK, nil)
			}
			return
		}
		logger.GetCtx(c).Errorf("Unable to tag contact %s\n", contact.Id)
		c.JSON(http.StatusNotFound, "unable to tag contact")
		return
	}
	c.JSON(http.StatusNotFound, nil)
}

func Setup(g *gin.RouterGroup) {
	g.POST("/new-application", newApplicationHandler)
}
