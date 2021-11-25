package checkr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
)

type InvitationStatus string

func (s InvitationStatus) String() string {
	return string(s)
}

const (
	InvitationPending   InvitationStatus = "pending"
	InvitationCompleted InvitationStatus = "completed"
	InvitationExpired   InvitationStatus = "expired"
)

// https://docs.checkr.com/#section/Webhooks/Invitation-events
type Invitation struct {
	ID            string           `json:"id"`
	Object        ResourceObject   `json:"object"`
	URI           string           `json:"uri"`
	InvitationURL string           `json:"invitation_url"`
	Status        InvitationStatus `json:"status"`

	CreatedAt   string  `json:"created_at"`
	ExpiresAt   *string `json:"expires_at"`
	CompletedAt *string `json:"completed_at"`
	DeletedAt   *string `json:"deleted_at"`

	Package     Package `json:"package"`
	CandidateID string  `json:"candidate_id"`
	ReportID    string  `json:"report_id"`
}

/*

DEFINITION
POST https://api.checkr.com/v1/invitations

CREATE A TEST INVITATION
$ curl -X POST https://api.checkr.com/v1/invitations \
        -u YOUR_TEST_API_KEY: \
        -d package=driver_pro \
        -d candidate_id=CANDIDATE_ID

*/

func createInvitation(endpoint string, ckr *Invitation) (*Invitation, error) {
	values := url.Values{}
	values.Set("package", ckr.Package.String())
	values.Set("candidate_id", ckr.CandidateID)

	res, err := postForm(endpoint, values)
	if err != nil {
		return nil, fmt.Errorf("couldn't post to create new invitation: %s", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read response body: %s", err)
	}

	if res.StatusCode >= 300 {
		return nil, NewStatusError(res.StatusCode, body)
	}

	var invitation *Invitation
	if err := json.Unmarshal(body, &invitation); err != nil {
		return nil, NewStatusErrorf(res.StatusCode, "couldn't unmarshal body into struct: %s", err)
	}

	return invitation, nil
}
