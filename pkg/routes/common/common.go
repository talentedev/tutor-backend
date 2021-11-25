package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"

	"github.com/gin-gonic/gin"
)

func geocode(c *gin.Context) {
	address := c.Query("address")

	if address == "" {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				"Address is not specified.",
			),
		)

		return
	}

	locations, err := services.GetLocations().GetUserLocations(address)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				fmt.Sprintf("Error getting user location based on address: %s", err.Error()),
			),
		)

		return
	}

	c.JSON(200, locations)
}

func geocodeLocationUpdate(c *gin.Context) {
	location := store.UserLocation{}

	if c.Request.ContentLength != 0 {
		if err := c.BindJSON(&location); err != nil {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("invalid form provided"))
			return
		}
	}

	if c.Query("lat") != "" && c.Query("lng") != "" {
		lat, errLat := strconv.ParseFloat(c.Query("lat"), 64)
		lng, errLng := strconv.ParseFloat(c.Query("lng"), 64)

		if errLat != nil || errLng != nil {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("Lat or Lng is incorrect!"))
			return
		}

		location.Position = &store.GeoJSON{
			Type: "Point",
			Coordinates: &store.Coordinates{
				Lat: lat,
				Lng: lng,
			},
		}
	}

	if err := services.GetLocations().LocationUpdate(c.Query("field"), &location); err != nil {
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, location)
}

type ipUserCountryDetails struct {
	Region struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"region"`
}

func getIPLocation(c *gin.Context) {
	response, err := http.Get(
		fmt.Sprint("http://usercountry.com/v1.0/json/", utils.GetIP(c)),
	)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	data, err := ioutil.ReadAll(response.Body)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	details := ipUserCountryDetails{}

	if err := json.Unmarshal(data, &details); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	c.JSON(http.StatusOK, map[string]float64{
		"lat": details.Region.Latitude,
		"lng": details.Region.Longitude,
	})
}

// Setup adds the common routes to the router
func Setup(g *gin.Engine) {
	g.GET("/geocode", geocode)
	g.POST("/geocode/location/update", geocodeLocationUpdate)
	g.GET("/geocode/ip", getIPLocation)

	g.GET("/version", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain", []byte(core.AppVersion))
	})
}
