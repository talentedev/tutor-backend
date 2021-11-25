package search

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

type searchResponse struct {
	Tutors      []store.UserDto `json:"tutors"`
	TriedOnline bool            `json:"tried_online"`
	QueryError  string          `json:"query_error,omitempty"`
}

func search(c *gin.Context) {
	s := services.GetSearch()

	// reset filters
	s.Clear()

	if c.Query("instantBook") == "true" {
		s.InstantBook(true)
	}

	if c.Query("instantSession") == "true" {
		s.InstantSession(true)
	}

	user, userExists := store.GetUser(c)
	if userExists && user.Role == store.RoleTutor {
		s.ExcludeLoggedUser(user)
		s.Timezone(user.Timezone)
	}

	if price := c.Query("price"); price != "" {
		priceRange := strings.Split(price, "-")

		if len(priceRange) != 2 {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("Invalid price range"))
			return
		}

		minPrice, err := strconv.Atoi(priceRange[0])
		if err != nil {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("Invalid min price"))
			return
		}

		maxPrice, err := strconv.Atoi(priceRange[1])
		if err != nil {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("Invalid max price"))
			return
		}

		s.Price(minPrice, maxPrice)
	}

	if general := c.Query("when"); general != "" {
		// handle general times
		strAvs := strings.Split(general, ",")
		avs := make([]store.GeneralAvailability, 0)

		for _, av := range strAvs {
			if when, err := strconv.ParseInt(av, 10, 32); err == nil {
				avs = append(avs, store.GeneralAvailability(when))
			}
		}

		s.GeneralAvailability(avs)
	}

	if specific := c.Query("specific"); specific != "" {
		// handle specific times
		s.SpecificAvailability(specific)
	}

	meetOnline := c.Query("meetonline")
	meetPlace, placeOK := c.GetQuery("meetplace")

	if err := setMeetingPlace(meetPlace); err != nil {
		// failing silently to allow searching?
		logger.GetCtx(c).Errorf("Could not set meeting place: %v", err)
	}

	if sub := c.Query("subject"); sub != "" {
		if !bson.IsObjectIdHex(sub) {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("Invalid subject id"))
			return
		}
		s.SubjectFilter(bson.ObjectIdHex(sub))
	}

	if query := c.Query("query"); query != "" {
		s.Query(query)
	}

	var res searchResponse

	res.Tutors = make([]store.UserDto, 0)

	results, err := s.Do(meetPlace != "", meetOnline != "")

	if err != nil {
		res.QueryError = err.Error()
	}

	var verifyDuplicates = make(map[string]bool, 0)

	for _, result := range results {
		dto := result.Dto()
		if _, b := verifyDuplicates[dto.ID.Hex()]; b {
			continue
		}
		verifyDuplicates[dto.ID.Hex()] = true

		if result.IsTestAccount && userExists && shouldIncludeTestAccountTutor(c, user, dto) {
			res.Tutors = append(res.Tutors, *dto)
		} else if !result.IsTestAccount {
			res.Tutors = append(res.Tutors, *dto)
		}
	}

	res.TriedOnline = meetOnline != "" || !placeOK

	c.JSON(http.StatusOK, res)
}

func shouldIncludeTestAccountTutor(c *gin.Context, user *store.UserMgo, dto *store.UserDto) bool {
	// test account, return everything
	if user.IsTestStudent() || user.IsTestTutor() || auth.IsAdmin(c) {
		return true
	}

	// test accounts only should find non-test accounts
	if !user.IsTestStudent() && !dto.IsTestAccount {
		return true
	}

	return false
}

func setMeetingPlace(meetPlace string) error {
	if meetPlace == "" {
		return nil
	}

	s := services.GetSearch()

	parts := strings.Split(meetPlace, ",")
	if len(parts) == 2 {
		lat, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return fmt.Errorf("address with only one comma couldn't parse search latitude as a float: %w", err)
		}

		lng, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return fmt.Errorf("address with only one comma couldn't parse search longitude as a float: %w", err)
		}

		if lat != 0 && lng != 0 {
			s.MeetLocation(&store.Coordinates{Lat: lat, Lng: lng})
			return nil
		}
	}

	c, err := services.AddressToCoordinates(meetPlace)
	if err != nil {
		return errors.Wrap(err, "setMeetPlace failed to convert an address to coordinates")
	}

	s.MeetLocation(c)
	return nil
}

func Setup(g *gin.RouterGroup) {
	g.GET("", auth.MiddlewareSilent, search)
}
