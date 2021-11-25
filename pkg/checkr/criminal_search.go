package checkr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"
)

const criminalSearchURL = "https://api.checkr.com/v1/%TYPE%_criminal_searches"

// CriminalCharge represents a criminal charge associated to criminal records.
type CriminalCharge struct {
	Charge          string `json:"charge"`
	ChargeType      string `json:"charge_type"`
	ChargeID        string `json:"charge_id"`
	Classification  string `json:"classification"`
	Deposition      string `json:"deposition"`
	Defendant       string `json:"defendant"`
	Plaintiff       string `json:"plaintiff"`
	Sentence        string `json:"sentence"`
	Disposition     string `json:"disposition"`
	ProbationStatus string `json:"probation_status"`
	OffenseDate     string `json:"offense_date"`
	DepositionDate  string `json:"deposition_date"`
	ArrestDate      string `json:"arrest_date"`
	ChargeDate      string `json:"charge_date"`
	SentenceDate    string `json:"sentence_date"`
	DispositionDate string `json:"disposition_date"`
}

// CriminalRecord represents a criminal record associated to a criminal search.
type CriminalRecord struct {
	CaseNumber        string           `json:"case_number"`
	FileDate          string           `json:"file_date"`
	ArrestingAgency   string           `json:"arresting_agency"`
	CourtJurisdiction string           `json:"court_jurisdiction"`
	CourtOfRecord     string           `json:"court_of_record"`
	DateOfBirth       string           `json:"dob"`
	FullName          string           `json:"full_name"`
	Charges           []CriminalCharge `json:"charges"`
}

type CriminalSearch interface {
	CriminalType() string
}

// NationalCriminalSearch represents an instant multi-state search of criminal records.
type NationalCriminalSearch struct {
	ID     string         `json:"id"`
	Object ResourceObject `json:"object"`
	URI    string         `json:"uri"`

	Status Status `json:"status"`

	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	TurnaroundTime int        `json:"turnaround_time"`

	Records []CriminalRecord `json:"records"`
}

func (n NationalCriminalSearch) CriminalType() string {
	return "national"
}

// CountyCriminalSearch represents a search of criminal records in a specific county.
type CountyCriminalSearch struct {
	NationalCriminalSearch

	County string `json:"county"`
	State  string `json:"state"`
}

func (c CountyCriminalSearch) CriminalType() string {
	return "county"
}

// StateCriminalSearch represents a search of criminal records in a specific state.
type StateCriminalSearch struct {
	NationalCriminalSearch

	State string `json:"state"`
}

func (s StateCriminalSearch) CriminalType() string {
	return "state"
}

func retrieveCriminalSearch(searchType string, id string) (CriminalSearch, error) {
	switch searchType {
	case "national", "county", "state":
	default:
		return nil, fmt.Errorf("invalid search type provided")
	}

	url := strings.Replace(criminalSearchURL, "%TYPE%", searchType, 1)
	res, err := get(fmt.Sprintf("%s/%s", url, id))
	if err != nil {
		return nil, fmt.Errorf("couldn't get %s criminal search: %s", searchType, err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read response body: %s", err)
	}

	switch res.StatusCode {
	case 200, 201:
		return getCriminalSearch(searchType, body)
	case 400, 401, 403, 404, 409, 429:
		var responseErr *ResponseError
		if err := json.Unmarshal(body, &responseErr); err != nil {
			return nil, fmt.Errorf("couldn't unmarshal err body into struct: %s", err)
		}
		return nil, fmt.Errorf("received 4xx status code: %s", responseErr)
	case 500:
		return nil, fmt.Errorf("received internal server error")
	default:
		return nil, fmt.Errorf("received unknown status code: %s", res.Status)
	}
}

func getCriminalSearch(searchType string, body []byte) (CriminalSearch, error) {
	switch searchType {
	case "national":
		var criminalSearch *NationalCriminalSearch
		if err := json.Unmarshal(body, &criminalSearch); err != nil {
			return nil, fmt.Errorf("couldn't unmarshal body into struct: %s", err)
		}
		return criminalSearch, nil
	case "county":
		var criminalSearch *CountyCriminalSearch
		if err := json.Unmarshal(body, &criminalSearch); err != nil {
			return nil, fmt.Errorf("couldn't unmarshal body into struct: %s", err)
		}
		return criminalSearch, nil
	case "state":
		var criminalSearch *StateCriminalSearch
		if err := json.Unmarshal(body, &criminalSearch); err != nil {
			return nil, fmt.Errorf("couldn't unmarshal body into struct: %s", err)
		}
		return criminalSearch, nil
	default:
		return nil, fmt.Errorf("invalid search type provided")
	}
}
