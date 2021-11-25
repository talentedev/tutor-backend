package bgcheck

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// ErrNotReady is returned when the provider indicates the report has been started but is not yet ready.
var ErrNotReady = NewStatusErrorf(http.StatusUnprocessableEntity, "The background check is not ready yet.")

func createCandidate(endpoint string, cd *Candidate) (*Candidate, error) {

	jsonB, err := json.Marshal(cd)
	if err != nil {
		return nil, fmt.Errorf("couldn't marshal the candidate to JSON for API call: %w", err)
	}

	res, err := postJSON(endpoint, jsonB)
	if err != nil {
		return nil, fmt.Errorf("couldn't post to create new candidate: %w", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read response body: %w", err)
	}

	if res.StatusCode == 422 {
		return nil, ErrNotReady
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

func retrieveReport(endpoint string, id string) (*Report, error) {
	endpoint = fmt.Sprintf(endpoint, id)
	res, err := get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("couldn't get report: %w", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read response body: %w", err)
	}

	if res.StatusCode >= 300 {
		return nil, NewStatusError(res.StatusCode, body)
	}

	var report *Report
	if err := json.Unmarshal(body, &report); err != nil {
		return nil, NewStatusErrorf(res.StatusCode, "couldn't unmarshal body into struct: %s", err)
	}

	return report, nil
}
