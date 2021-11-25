package services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/store"
)

const (
	googleGeocodeKey                  = "AIzaSyA3c50qvfZ98V8kGNWwwm1My42SitWfYT8"
	googleGeocodeAPIURL               = "https://maps.googleapis.com/maps/api/geocode/json"
	googleGeocodeStatusOK             = "OK"
	googleGeocodeStatusZeroResults    = "ZERO_RESULTS"
	googleGeocodeStatusOverQueryLimit = "OVER_QUERY_LIMIT"
)

type geocodeGeometry struct {
	Location     *store.Coordinates `json:"location"`
	LocationType string             `json:"location_type"`
	Viewport     struct {
		Northeast *store.Coordinates `json:"northeast"`
		Southwest *store.Coordinates `json:"southwest"`
	} `json:"viewport"`
}

type geocodeComponents struct {
	LongName  string   `json:"long_name"`
	ShortName string   `json:"short_name"`
	Types     []string `json:"types"`
}

type geocodeResult struct {
	AddressComponents []geocodeComponents `json:"address_components"`
	FormattedAddress  string              `json:"formatted_address"`
	Geometry          *geocodeGeometry    `json:"geometry"`
	PlaceID           string              `json:"place_id"`
	Types             []string            `json:"types"`
}

type geocodeGoogleResponse struct {
	Results []geocodeResult `json:"results"`
	Status  string          `json:"status"`
}

// AddressToCoordinates calls to googel geocodeAPI to get coordinates from an address
// https://developers.google.com/maps/documentation/geocoding/intro
func AddressToCoordinates(address string) (*store.Coordinates, error) {
	geo, err := requestAPI(address)
	if err != nil {
		return nil, err
	}
	if len(geo.Results) == 0 {
		return nil, errors.New("no google geocode results")
	}
	return geo.Results[0].Geometry.Location, nil
}

func requestAPI(address string) (response *geocodeGoogleResponse, err error) {
	if address == "" {
		return nil, errors.New("Address is empty")
	}

	requestURL, _ := url.Parse(googleGeocodeAPIURL)

	params := url.Values{}
	params.Add("language", "en")
	params.Add("address", address)
	params.Add("key", googleGeocodeKey)

	requestURL.RawQuery = params.Encode()

	resp, err := http.Get(requestURL.String())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to make geocode request")
	}
	defer resp.Body.Close()

	responseDataRaw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to ready response body")
	}

	if err := json.Unmarshal(responseDataRaw, &response); err != nil {
		return nil, errors.Wrap(err, "Failed to map data to internal structure")
	}

	if response.Status != googleGeocodeStatusOK || len(response.Results) == 0 {
		logger.Get().Info("Google Geocode Status:", response.Status, "Address:", address)
		return nil, errors.New("No results from geocode request")
	}

	return
}

func rawToUserLocations(response *geocodeGoogleResponse) (locations []*store.UserLocation) {
	locations = make([]*store.UserLocation, 0)

	for index, result := range response.Results {
		location := &store.UserLocation{
			Position: &store.GeoJSON{
				Type:        "Point",
				Coordinates: result.Geometry.Location,
			},
			Country:    response.getName(index, []string{"country"}),
			State:      response.getName(index, []string{"administrative_area_level_1"}),
			City:       response.getName(index, []string{"locality"}),
			Address:    response.getName(index, []string{"street_number"}) + " " + response.getName(index, []string{"route"}) + "",
			PostalCode: response.getName(index, []string{"postal_code"}),
		}

		locations = append(locations, location)
	}

	return
}

func (g *geocodeGoogleResponse) getName(index int, types []string) (name string) {
	if len(g.Results) == 0 || index >= len(g.Results) {
		return
	}

	for _, component := range g.Results[index].AddressComponents {
		for _, responseType := range types {
			for _, componentType := range component.Types {
				if responseType == componentType {
					return component.LongName
				}
			}
		}
	}

	return
}

type locationsService struct{}

// GetLocations gets a struct that holds functions for working with locations
func GetLocations() *locationsService {
	return &locationsService{}
}

func (s *locationsService) LocationUpdate(field string, loc *store.UserLocation) (err error) {
	if loc == nil {
		return errors.New("location is nil")
	}

	address := loc.String()

	if field == "coordinates" {
		address = fmt.Sprintf("%v,%v", loc.Position.Coordinates.Lat, loc.Position.Coordinates.Lng)
	} else if loc.City == "" && loc.Country == "" && loc.Address == "" {
		return
	}

	locations, err := s.GetUserLocations(address)
	if err != nil {
		return err
	}

	if len(locations) >= 1 {
		item := locations[0]
		loc.Position = item.Position
		loc.Country = item.Country
		loc.State = item.State
		loc.City = item.City
		loc.Address = item.Address
		loc.PostalCode = item.PostalCode
	}

	return
}

func (s *locationsService) GetUserLocations(address string) (locations []*store.UserLocation, err error) {
	response, err := requestAPI(address)
	if err != nil {
		return nil, err
	}

	return rawToUserLocations(response), nil
}
