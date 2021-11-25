package checkr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

const ssnTracesURL = "https://api.checkr.com/v1/ssn_traces"

// SSNAddress represents the address object associated to an SSN trace.
type SSNAddress struct {
	Address
	FromDate string `json:"from_date"`
	ToDate   string `json:"to_date"`
}

// SSNTrace represents a list of all the addresses, name aliases and phone numbers of the
// candidate that have been recorded by credit agencies in the last 7 years. It is attached to a report.
type SSNTrace struct {
	ID     string         `json:"id"`
	Object ResourceObject `json:"object"`
	URI    string         `json:"uri"`

	Status Status `json:"status"`

	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`

	TurnaroundTime int `json:"turnaround_time"`

	SSN       string       `json:"ssn"`
	Addresses []SSNAddress `json:"addresses"`
	Aliases   []struct {
		FirstName  string `json:"first_name" url:"first_name,omitempty"`
		MiddleName string `json:"middle_name" url:"middle_name,omitempty"`
		LastName   string `json:"last_name" url:"last_name,omitempty"`
	}
}

// Retrieve makes a GET request to retrieve an existing SSN trace.
func retrieveSSNTrace(id string) (*SSNTrace, error) {
	res, err := get(fmt.Sprintf("%s/%s", ssnTracesURL, id))
	if err != nil {
		return nil, fmt.Errorf("couldn't get ssn trace: %s", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read response body: %s", err)
	}

	switch res.StatusCode {
	case 200, 201:
		var ssnTrace *SSNTrace
		if err := json.Unmarshal(body, &ssnTrace); err != nil {
			return nil, fmt.Errorf("couldn't unmarshal body into struct: %s", err)
		}
		return ssnTrace, nil
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
