package me

import (
	"fmt"
	"net/http"
	"time"

	"gitlab.com/learnt/api/pkg/store"

	"gopkg.in/mgo.v2/bson"

	"github.com/gin-gonic/gin"
)

type availabilityRequest struct {
	From      time.Time `json:"from" binding:"required"`
	To        time.Time `json:"to" binding:"required"`
	Recurrent bool      `json:"recurrent"`
}

type updateAvailabilityRequest struct {
	Recurrent bool `json:"recurrent"`
}

func updateAvailability(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		return
	}

	id := c.Param("id")
	if !bson.IsObjectIdHex(id) {
		c.JSON(http.StatusBadRequest, updateResponse{
			Error:   true,
			Message: "invalid availability id",
		})
		return
	}

	var req updateAvailabilityRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, updateResponse{
			Error:   true,
			Message: "invalid form provided",
		})
		return
	}

	user.SetAvailabilityRecurrency(bson.ObjectIdHex(id), req.Recurrent)

	go notifyProfileChangeFor(user, ProfileUpdateAvailability)
}

func updateBlackout(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		return
	}

	id := c.Param("id")
	if !bson.IsObjectIdHex(id) {
		c.JSON(http.StatusBadRequest, updateResponse{
			Error:   true,
			Message: "invalid availability id",
		})
		return
	}

	var req updateAvailabilityRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, updateResponse{
			Error:   true,
			Message: "invalid form provided",
		})
		return
	}

	user.SetBlackoutRecurrency(bson.ObjectIdHex(id), req.Recurrent)

	go notifyProfileChangeFor(user, ProfileUpdateAvailability)
}

func removeAvailability(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		return
	}

	id := c.Param("id")
	if !bson.IsObjectIdHex(id) {
		c.JSON(http.StatusBadRequest, updateResponse{
			Error:   true,
			Message: "invalid availability id",
		})
		return
	}

	if err := user.RemoveAvailability(bson.ObjectIdHex(id)); err != nil {
		c.JSON(http.StatusBadRequest, updateResponse{
			Error:   true,
			Message: "couldn't remove availability",
		})
		return
	}

	go notifyProfileChangeFor(user, ProfileUpdateAvailability)
}

func removeBlackout(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		return
	}

	id := c.Param("id")
	if !bson.IsObjectIdHex(id) {
		c.JSON(http.StatusBadRequest, updateResponse{
			Error:   true,
			Message: "invalid availability id",
		})
		return
	}

	if err := user.RemoveBlackout(bson.ObjectIdHex(id)); err != nil {
		c.JSON(http.StatusBadRequest, updateResponse{
			Error:   true,
			Message: "couldn't remove availability",
		})
		return
	}

	go notifyProfileChangeFor(user, ProfileUpdateAvailability)
}

func createAvailability(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		return
	}

	req := availabilityRequest{}
	res := updateResponse{}
	res.Data.Fields = make(map[string]string)

	if err := c.BindJSON(&req); err != nil {
		res.Message = "invalid fields provided"
		res.Data.Raw = fmt.Errorf("invalid fields: %s", err).Error()
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if !req.Recurrent {

		if req.From.Before(time.Now()) {
			res.Data.Fields["from"] = "starting time is in the past"
		}

		if req.To.Before(time.Now()) {
			res.Data.Fields["to"] = "ending time is in the past"
		}

		if len(res.Data.Fields) > 0 {
			res.Message = "couldn't add availability"
			c.JSON(http.StatusBadRequest, res)
			return
		}
	}

	userLoc := user.TimezoneLocation()

	av := &store.AvailabilitySlot{
		From: req.From.In(userLoc),
		To:   req.To.In(userLoc),
	}

	if err := user.AddAvailability(av, req.Recurrent); err != nil {
		res.Message = "couldn't add availability"
		res.Data.Raw = fmt.Errorf("%s", err).Error()
		c.JSON(http.StatusBadRequest, res)
	}

	go notifyProfileChangeFor(user, ProfileUpdateAvailability)
}

func createBlackout(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		return
	}

	req := availabilityRequest{}
	res := updateResponse{}
	res.Data.Fields = make(map[string]string)

	if err := c.BindJSON(&req); err != nil {
		res.Message = "invalid fields provided"
		res.Data.Raw = fmt.Errorf("invalid fields: %s", err).Error()
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if !req.Recurrent {

		if req.From.Before(time.Now()) {
			res.Data.Fields["from"] = "starting time is in the past"
		}

		if req.To.Before(time.Now()) {
			res.Data.Fields["to"] = "ending time is in the past"
		}

		if len(res.Data.Fields) > 0 {
			res.Message = "couldn't add availability"
			c.JSON(http.StatusBadRequest, res)
			return
		}
	}

	userLoc := user.TimezoneLocation()

	av := &store.AvailabilitySlot{
		From: req.From.In(userLoc),
		To:   req.To.In(userLoc),
	}

	if err := user.AddBlackout(av, req.Recurrent); err != nil {
		res.Message = "couldn't add availability"
		res.Data.Raw = fmt.Errorf("%s", err).Error()
		c.JSON(http.StatusBadRequest, res)
	}

	go notifyProfileChangeFor(user, ProfileUpdateAvailability)
}
