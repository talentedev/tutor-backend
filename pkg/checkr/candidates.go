package checkr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"time"

	"github.com/google/go-querystring/query"
)

// Candidate represents a candidate to be screened.
type Candidate struct {
	ID        string         `json:"id" url:"-"`
	Object    ResourceObject `json:"object" url:"object,omitempty"`
	URI       string         `json:"uri" url:"-"`
	CreatedAt time.Time      `json:"created_at" url:"-"`

	FirstName  string `json:"first_name" url:"first_name,omitempty"`
	MiddleName string `json:"middle_name" url:"middle_name,omitempty"`
	LastName   string `json:"last_name" url:"last_name,omitempty"`

	NoMiddleName     bool   `json:"no_middle_name" url:"no_middle_name,omitempty"`
	MotherMaidenName string `json:"mother_maiden_name" url:"mother_maiden_name,omitempty"`
	Email            string `json:"email" url:"email"`
	Phone            string `json:"phone" url:"phone,omitempty"` // format: XXXXXXXXXX
	ZipCode          string `json:"zipcode" url:"zipcode,omitempty"`
	DateOfBirth      string `json:"dob" url:"dob,omitempty"` // format: YYYY-MM-DD

	SocialSecurityNumber string `json:"ssn" url:"ssn,omitempty"` // format: XXX-XX-XXXX

	DriverLicenseNumber string `json:"driver_license_number" url:"-"`
	DriverLicenseState  string `json:"driver_license_state" url:"-"` // format: ST

	PreviousDriverLicenseNumber string `json:"previous_driver_license_number" url:"-"`
	PreviousDriverLicenseState  string `json:"previous_driver_license_state" url:"-"`

	CopyRequested bool   `json:"copy_requested" url:"copy_requested,omitempty"`
	CustomID      string `json:"custom_id" url:"-"`

	ReportIDs []string `json:"report_ids" url:"-"`
	GeoIDs    []string `json:"geo_ids" url:"-"`
}

type CandidateList struct {
	Candidates []Candidate `json:"data"`
	Pagination
}

// CandidateFilters is used in the request to list existing Candidates.
type CandidateFilters struct {
	Email         string       `json:"email" url:"email,omitempty"`
	FullName      string       `json:"full_name" url:"full_name,omitempty"`
	Adjudication  Adjudication `json:"adjudication" url:"adjudication,omitempty"`
	CustomID      string       `json:"custom_id" url:"custom_id,omitempty"`
	CreatedAfter  string       `json:"created_after" url:"created_after,omitempty"`
	CreatedBefore string       `json:"created_before" url:"created_before,omitempty"`
	GeoID         string       `json:"geo_id" url:"geo_id,omitempty"`
}

func (cf CandidateFilters) SetValues(values *url.Values) error {
	v, err := query.Values(&cf)
	if err != nil {
		return err
	}
	for k := range v {
		values.Set(k, v.Get(k))
	}
	return nil
}

/*
DEFINITION
POST https://api.checkr.com/v1/candidates

EXAMPLE REQUEST
$ curl -X POST https://api.checkr.com/v1/candidates \
              -u 83ebeabdec09f6670863766f792ead24d61fe3f9: \
              -d first_name=John \
              -d middle_name=Alfred \
              -d last_name=Smith \
              -d email=john.smith@gmail.com \
              -d phone=5555555555 \
              -d zipcode=90401 \
              -d dob=1970-01-22 \
              -d ssn=543-43-4645 \
              -d driver_license_number=F211165 \
              -d driver_license_state=CA

*/

func createCandidate(endpoint string, ckr *Candidate) (*Candidate, error) {
	values, err := query.Values(ckr)
	if err != nil {
		return nil, fmt.Errorf("couldn't format candidate to url values: %s", err)
	}

	res, err := postForm(endpoint, values)
	if err != nil {
		return nil, fmt.Errorf("couldn't post to create new candidate: %s", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read response body: %s", err)
	}

	if res.StatusCode >= 300 {
		return nil, NewStatusError(res.StatusCode, body)
	}

	var candidate *Candidate
	if err := json.Unmarshal(body, &candidate); err != nil {
		return nil, NewStatusErrorf(res.StatusCode, "couldn't unmarshal body into struct: %s", err)
	}

	return candidate, nil
}

/*
DEFINITION
GET https://api.checkr.com/v1/candidates/:id

EXAMPLE REQUEST
$ curl -X GET https://api.checkr.com/v1/candidates/e44aa283528e6fde7d542194 -u 83ebeabdec09f6670863766f792ead24d61fe3f9:
*/

func retrieveCandidate(endpoint string, id string) (*Candidate, error) {
	endpoint = endpoint + "/" + id
	res, err := get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("couldn't get report: %s", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read response body: %s", err)
	}

	if res.StatusCode >= 300 {
		return nil, NewStatusError(res.StatusCode, body)
	}

	var candidate *Candidate
	if err := json.Unmarshal(body, &candidate); err != nil {
		return nil, NewStatusErrorf(res.StatusCode, "couldn't unmarshal body into struct: %s", err)
	}

	return candidate, nil
}

/*
DEFINITION
GET https://api.checkr.com/v1/candidates

EXAMPLE REQUEST
$ curl -X GET https://api.checkr.com/v1/candidates -u 83ebeabdec09f6670863766f792ead24d61fe3f9:
*/

func listCandidates(endpoint string, filters *CandidateFilters, params *PaginationParams) (*CandidateList, error) {
	values := url.Values{}

	if filters != nil {
		if err := filters.SetValues(&values); err != nil {
			return nil, err
		}
	}

	if params != nil {
		if err := params.SetValues(&values); err != nil {
			return nil, err
		}
	}

	res, err := get(fmt.Sprintf("%s?%s", endpoint, values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("couldn't get paginated candidates: %s", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read response body: %s", err)
	}

	if res.StatusCode >= 300 {
		return nil, NewStatusError(res.StatusCode, body)
	}

	var cl CandidateList
	if err := json.Unmarshal(body, &cl); err != nil {
		return nil, NewStatusErrorf(res.StatusCode, "couldn't unmarshal body into struct: %s", err)
	}

	return &cl, nil
}
