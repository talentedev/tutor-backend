package checkr

import (
	"gitlab.com/learnt/api/config"
	"net/http"
	"net/url"
	"strings"
)

// API is the top level component that wraps calls to the Checkr API
type API struct {
	endpoints map[string]string
}

const (
	candidate  = "candidate"
	report     = "report"
	invitation = "invitation"
)

// New returns a new instance of the checkr API
func New() *API {
	return &API{
		endpoints: map[string]string{
			candidate:  "https://api.checkr.com/v1/candidates",
			report:     "https://api.checkr.com/v1/reports",
			invitation: "https://api.checkr.com/v1/invitations",
		},
	}
}

func (a *API) CreateCandidate(c *Candidate) (*Candidate, error) {
	return createCandidate(a.endpoints[candidate], c)
}

func (a *API) RetrieveCandidate(id string) (*Candidate, error) {
	return retrieveCandidate(a.endpoints[candidate], id)
}

func (a *API) ListCandidates(filters *CandidateFilters, params *PaginationParams) (*CandidateList, error) {
	return listCandidates(a.endpoints[candidate], filters, params)
}

func (a *API) CreateReport(c *Report) (*Report, error) {
	return createReport(a.endpoints[report], c)
}

func (a *API) RetrieveReport(id string) (*Report, error) {
	return retrieveReport(a.endpoints[report], id)
}

func (a *API) CreateInvitation(c *Invitation) (*Invitation, error) {
	return createInvitation(a.endpoints[invitation], c)
}

func (a *API) RetrieveSSNTrace(id string) (*SSNTrace, error) {
	return retrieveSSNTrace(id)
}

func (a *API) RetrieveSexOffender(id string) (*SexOffenderSearch, error) {
	return retrieveSexOffender(id)
}

func (a *API) RetrieveCriminalSearch(searchType string, id string) (CriminalSearch, error) {
	return retrieveCriminalSearch(searchType, id)
}

func get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(config.GetConfig().GetString("service.checkr.key"), "")

	return http.DefaultClient.Do(req)
}

func postForm(url string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(config.GetConfig().GetString("service.checkr.key"), "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return http.DefaultClient.Do(req)
}
