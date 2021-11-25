package bgcheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2/bson"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/bgcheck"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils/messaging"
	"gitlab.com/learnt/api/pkg/utils/messaging/mail"
)

type response struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Raw     string `json:"raw"`
}

func (h *handler) candidateCreateHandler(c *gin.Context) {
	var candidate *bgcheck.Candidate

	if err := c.BindJSON(&candidate); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Message: "invalid form submitted",
			Raw:     err.Error(),
		})
		return
	}

	//Set the webhook URL and pass "true" to initialize communication with the applicant
	cbURL, err := core.AppURL("/bgcheck/webhook")
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Error:   true,
			Message: "failed to create callback url",
			Raw:     err.Error(),
		})
	}
	candidate.CallbackURL = cbURL
	// For a turn-controlled flow, set EmailCandidate = true. Turn will immediately initiate communication with the candidate.
	candidate.EmailCandidate = true

	newCandidate, err := h.API.CreateCandidate(candidate)
	if err != nil {
		logger.GetCtx(c).Errorf("failed to create candidate: %v", err)
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Message: "couldn't create candidate",
			Raw:     err.Error(),
		})
		return
	}

	bgCheckData := &store.UserBGCheckData{
		CandidateID: newCandidate.ID,
		ShortID:     "",
		State:       "invited",
		Finished:    false,
	}

	if err := h.Users.SetBGCheckData(bson.ObjectIdHex(candidate.ReferenceID), bgCheckData); err != nil {
		logger.GetCtx(c).Errorf("failed to save background check details to database: %v", err)
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Message: "couldn't update user with candidate ID",
			Raw:     err.Error(),
		})
		return
	}

	user, exist := h.Users.ByID(bson.ObjectIdHex(candidate.ReferenceID))
	if !exist {
		logger.GetCtx(c).Errorf("error finding user to update approval, user %s does not exist", candidate.ReferenceID)
		c.JSON(http.StatusInternalServerError, "error updating user status")
		return
	}

	if err := user.UpdateApprovalStatus(store.ApprovalStatusBackgroundCheckRequested); err != nil {
		logger.GetCtx(c).Errorf("error updating approval status for user %s | cause: %#v", user.ID, err)
		c.JSON(http.StatusInternalServerError, "error updating user status")
		return
	}

	c.JSON(http.StatusOK, newCandidate)
}

func (h *handler) candidateGetHandler(c *gin.Context) {

	user, exist := h.Users.ByID(bson.ObjectIdHex(c.Param("id")))
	if !exist {
		logger.GetCtx(c).Errorf("error finding user, user %s does not exist", c.Param("id"))
		c.JSON(http.StatusInternalServerError, "error getting user")
		return
	}

	c.JSON(http.StatusOK, user.BGCheckData)
}

func (h *handler) reportGetHandler(c *gin.Context) {

	user, exist := h.Users.ByID(bson.ObjectIdHex(c.Param("id")))
	if !exist {
		logger.GetCtx(c).Errorf("error finding user, user %s does not exist", c.Param("id"))
		c.JSON(http.StatusInternalServerError, "error updating user status")
		return
	}

	logger.GetCtx(c).Infof("found user %s with data %#v", user.ID, user.BGCheckData)

	report, err := h.API.RetrieveReport(user.BGCheckData.CandidateID)
	if err != nil && errors.Is(err, bgcheck.ErrNotReady) {
		logger.GetCtx(c).Infof("candidate report for %s is not ready: %v", user.BGCheckData.CandidateID, err)
		c.JSON(http.StatusOK, response{
			Error:   false,
			Message: "report not ready",
			Raw:     err.Error(),
		})
		return
	} else if err != nil {
		logger.GetCtx(c).Errorf("could not find candidate: %v", err)
		c.JSON(http.StatusNotFound, response{
			Error:   false,
			Message: "candidate not found",
			Raw:     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, report)
}

func (h *handler) webhookHandler(c *gin.Context) {

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		logger.GetCtx(c).Errorf("error on webhook request: %v", err)
		c.JSON(http.StatusBadRequest, "invalid request")
		return
	}

	var webhookData *bgcheck.Report
	err = json.Unmarshal([]byte(body), &webhookData)
	if err != nil {
		logger.GetCtx(c).Errorf("error unmarshalling webhook data: %v", err)
		c.JSON(http.StatusBadRequest, "invalid request")
		return
	}

	err = h.processReport(webhookData)
	if err != nil {
		logger.GetCtx(c).Errorf("error processing webhook report for bgcheck: %v", err)
	}

	c.Status(http.StatusOK)
}

func (h *handler) processReport(webhook *bgcheck.Report) error {

	user, err := h.Users.ByCandidateID(webhook.WorkerUUID)
	if err != nil {
		return fmt.Errorf("failed to retrieve candidate by ID %s: %v", webhook.WorkerUUID, err)
	}
	if user == nil {
		return fmt.Errorf("user not found for candidate %s", webhook.WorkerUUID)
	}

	data := store.UserBGCheckData{
		CandidateID: webhook.WorkerUUID,
		ShortID:     webhook.TurnID,
		Finished:    webhook.Complete,
		State:       webhook.PartnerWorkerStatus,
	}

	if err := h.Users.SetBGCheckData(user.ID, &data); err != nil {
		return fmt.Errorf("failed update bgcheck data %v for candidate %s: %w", data, user.ID, err)
	}

	if err := user.UpdateApprovalStatus(store.ApprovalStatusBackgroundCheckCompleted); err != nil {
		return fmt.Errorf("error updating approval status for user %s | cause: %w", user.ID, err)
	}

	tutorProfileURL, err := core.AppURL("/admin/tutors/pending/%s", user.ID.Hex())
	if err != nil {
		return fmt.Errorf("failed to create tutor profile URL for user %s: %w", user.ID.Hex(), err)
	}

	//BUG the error returned from SendTo is lost?
	go mail.GetSender(config.GetConfig()).SendTo(messaging.HIRING_EMAIL, messaging.TPL_BACKGROUND_CHECK_COMPLETE, &messaging.P{
		"CANDIDATE_LINK": tutorProfileURL,
		"TUTOR_NAME":     user.Name(),
	})

	return nil
}
