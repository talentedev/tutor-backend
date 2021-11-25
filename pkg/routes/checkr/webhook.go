package checkr

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/checkr"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/store"
	m "gitlab.com/learnt/api/pkg/utils/messaging"
	"gitlab.com/learnt/api/pkg/utils/messaging/mail"
)

func (h *handler) checkrWebhookHandler(c *gin.Context) {
	var checkrWebhook *checkr.Webhook
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, "invalid request")
		return
	}

	err1 := json.Unmarshal([]byte(body), &checkrWebhook)
	if err1 != nil {
		c.JSON(http.StatusBadRequest, "invalid request")
		return
	}

	if checkrWebhook.Object != "event" {
		c.JSON(http.StatusBadRequest, "invalid object")
		return
	}

	if checkrWebhook.Type == "report.completed" {
		h.processReport(c, checkrWebhook)
		c.Status(http.StatusOK)
		return
	} else if strings.HasPrefix(checkrWebhook.Type, "report") {
		c.Status(http.StatusNoContent)
		return
	}

	if strings.HasPrefix(checkrWebhook.Type, "invitation") {
		c.Status(http.StatusNoContent)
		return
	}

	// we didn't know what to do with it...
	c.JSON(http.StatusUnprocessableEntity, "invalid object")
}

func (h *handler) processReport(c *gin.Context, checkrWebhook *checkr.Webhook) {
	// marshal and unmarshal for workaround to convert from map[string]interface{} to struct
	objstr, err := json.Marshal(checkrWebhook.Data.Object)
	if err != nil {
		logger.GetCtx(c).Errorf("error parsing payload: %#v", checkrWebhook.Data)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	var checkrReport checkr.Report
	err = json.Unmarshal(objstr, &checkrReport)
	if err != nil {
		logger.GetCtx(c).Errorf("error parsing payload: %+v", checkrWebhook.Data)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	logger.GetCtx(c).Infof("candidate %s report completed", checkrReport.CandidateID)
	user, _ := h.Users.ByCandidateID(checkrReport.CandidateID)
	if user != nil {
		if err := h.Users.SetCheckrData(
			user.ID,
			&store.UserCheckrData{
				CandidateID: checkrReport.CandidateID,
				ReportID:    checkrReport.ID,
				Finished:    true,
				Status:      string(checkrReport.Status),
			}); err != nil {
			c.JSON(http.StatusInternalServerError, "invalid object")
			return
		}

		if err := user.UpdateApprovalStatus(store.ApprovalStatusBackgroundCheckCompleted); err != nil {
			logger.GetCtx(c).Errorf("error updating approval status for user %s | cause: %#v", user.ID, err)
			c.JSON(http.StatusInternalServerError, "error updating user status")
			return
		}

		tutorProfileURL, err := core.AppURL("/admin/tutors/pending/%s", user.ID.Hex())
		if err != nil {
			c.JSON(http.StatusInternalServerError, response{Error: true, Message: err.Error(), Raw: err.Error()})
			return
		}

		go mail.GetSender(config.GetConfig()).SendTo(m.HIRING_EMAIL, m.TPL_BACKGROUND_CHECK_COMPLETE, &m.P{
			"CANDIDATE_LINK": tutorProfileURL,
			"TUTOR_NAME":     user.Name(),
		})
	} else {
		logger.GetCtx(c).Errorf("user not found for candidate %s", checkrReport.CandidateID)
	}
}
