package bgcheck

import (
	"bytes"
	"fmt"
	"net/http"

	"gitlab.com/learnt/api/config"
)

// Candidate represents a candidate to be screened.
type Candidate struct {
	ID             string `json:"worker_id"`
	ReferenceID    string `json:"reference_id"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`           // format: XXXXXXXXXX
	EmailCandidate bool   `json:"email_candidate"` //Set to true for the vendor-initiated flow.
	CallbackURL    string `json:"callback_url"`
}

// API is the top level component that wraps calls to the background check API
type API struct {
	endpoints map[string]string
}

type Report struct {
	Checks              Checks `json:"checks"`
	DOB                 string `json:"date_of_birth"`
	FullName            string `json:"name"`
	Email               string `json:"email"`
	PartnerWorkerStatus string `json:"partner_worker_status"`
	DashboardURL        string `json:"dashboard_url"`
	TurnID              string `json:"turn_id"`
	ID                  string `json:"reference_id"`
	CallbackUUID        string `json:"callback_uuid"`
	WorkerUUID          string `json:"worker_uuid"`
	Complete            bool   `json:"complete"`
}

type Checks struct {
	Criminal    interface{} `json:"criminal"`
	SexOffender interface{} `json:"sex_offender"`
	SSN         interface{} `json:"ssn"`
	SSNStatus   string      `json:"ssn_status"`
}

const (
	search  = "search"
	status  = "status"
	details = "details"
)

// New returns a new instance of the background check API
func New() *API {
	return &API{
		endpoints: map[string]string{
			search:  "https://api.turning.io/v1/person/search_async",
			status:  "https://api.turning.io/v1/person/%s/status",
			details: "https://api.turning.io/v1/person/%s/details",
		},
	}
}

// CreateCandidate calls the background check service API to create a candidate and returns the response candidate with the new worker ID
func (a *API) CreateCandidate(c *Candidate) (*Candidate, error) {
	return createCandidate(a.endpoints[search], c)
}

// RetrieveCandidate gets the candidate report if it exists.
// Returns ErrNotReady if the invitation had been sent but the report is not complete.
func (a *API) RetrieveReport(id string) (*Report, error) {
	return retrieveReport(a.endpoints[details], id)
}

func get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.GetConfig().Service.BGCheck.Auth))

	return http.DefaultClient.Do(req)
}

func postJSON(url string, json []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(json))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.GetConfig().Service.BGCheck.Auth))
	req.Header.Add("Content-Type", "application/json")

	return http.DefaultClient.Do(req)
}
