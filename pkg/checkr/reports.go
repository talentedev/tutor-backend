package checkr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
)

type ReportStatus string

func (s ReportStatus) String() string {
	return string(s)
}

const (
	ReportPending  ReportStatus = "pending"
	ReportComplete ReportStatus = "complete"
	ReportDispute  ReportStatus = "dispute"
	ReportCanceled ReportStatus = "canceled"
)

// Report represents a background check report. Depending on the selected package, a report can include
// the following screenings: SSN trace, sex offender search, national criminal search, county criminal
// searches and motor vehicle report.
// https://docs.checkr.com/#section/Webhooks/Report-events
type Report struct {
	ID     string         `json:"id"`
	Object ResourceObject `json:"object"`
	URI    string         `json:"uri"`

	Status ReportStatus `json:"status"`

	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at"`
	RevisedAt   *string `json:"revised_at"`

	TurnaroundTime int    `json:"turnaround_time"`
	DueTime        string `json:"due_time"`

	Adjudication Adjudication `json:"adjudication"`
	Package      Package      `json:"package"`

	CandidateID              string   `json:"candidate_id"`
	SSNTraceID               string   `json:"ssn_trace_id"`
	SexOffenderSearchID      string   `json:"sex_offender_search_id"`
	NationalCriminalSearchID string   `json:"national_criminal_search_id"`
	CountyCriminalSearchIDs  []string `json:"county_criminal_search_ids"`
	MotorVehicleReportID     string   `json:"motor_vehicle_report_id"`
	StateCriminalSearches    []string `json:"state_criminal_searches"`

	DocumentIDs []string `json:"document_ids"`
	GeoIDs      []string `json:"geo_ids"`
}

/*

DEFINITION
POST https://api.checkr.com/v1/invitations

CREATE A TEST REPORT
$ curl -X POST https://api.checkr.com/v1/reports \
        -u YOUR_TEST_API_KEY: \
        -d package=driver_pro \
        -d candidate_id=CANDIDATE_ID

*/

func createReport(endpoint string, ckr *Report) (*Report, error) {
	values := url.Values{}
	values.Set("package", ckr.Package.String())
	values.Set("candidate_id", ckr.CandidateID)

	res, err := postForm(endpoint, values)
	if err != nil {
		return nil, fmt.Errorf("couldn't post to create new report: %s", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read response body: %s", err)
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

func retrieveReport(endpoint string, id string) (*Report, error) {
	res, err := get(fmt.Sprintf("%s/%s", endpoint, id))
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

	var report *Report
	if err := json.Unmarshal(body, &report); err != nil {
		return nil, NewStatusErrorf(res.StatusCode, "couldn't unmarshal body into struct: %s", err)
	}

	return report, nil
}
