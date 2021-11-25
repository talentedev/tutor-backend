package checkr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

const sexOffenderSearchesURL = "https://api.checkr.com/v1/sex_offender_searches"

// SexOffenderSearchRecord represents the record associated to a sex offender search.
type SexOffenderSearchRecord struct {
	Registry          string `json:"registry"`
	FullName          string `json:"full_name"`
	Age               string `json:"age"`
	DateOfBirth       string `json:"dob"`
	RegistrationStart string `json:"registration_start"`
	RegistrationEnd   string `json:"registration_end"`
}

// SexOffenderSearch represents an instant multi-state sex offender registry search.
type SexOffenderSearch struct {
	ID     string         `json:"id"`
	Object ResourceObject `json:"object"`
	URI    string         `json:"uri"`

	Status Status `json:"status"`

	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	TurnaroundTime int        `json:"turnaround_time"`

	Records []SexOffenderSearchRecord `json:"records"`
}

func retrieveSexOffender(id string) (*SexOffenderSearch, error) {
	res, err := get(fmt.Sprintf("%s/%s", sexOffenderSearchesURL, id))
	if err != nil {
		return nil, fmt.Errorf("couldn't get sex offender search: %s", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read response body: %s", err)
	}

	switch res.StatusCode {
	case 200, 201:
		var sexOffenderSearch *SexOffenderSearch
		if err := json.Unmarshal(body, &sexOffenderSearch); err != nil {
			return nil, fmt.Errorf("couldn't unmarshal body into struct: %s", err)
		}
		return sexOffenderSearch, nil
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
