package checkr_test

import (
	"testing"

	"github.com/gobuffalo/envy"
	"github.com/kr/pretty"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/checkr"
)

// Test_Service_Integration is an integration test and will actually hit the
// checkr api endpoint. This should be skipped when sent to production
// and unskipped locally only for testing/development
func Test_Service_Integration(t *testing.T) {
	// Uncomment this line to test locally
	t.Skip("skipping integration test for checkr api")
	config.GetConfig().Set("service.checkr.key", envy.Get("CHECKR_API_KEY", ""))
	t.Logf("apiKey: %q", envy.Get("CHECKR_API_KEY", ""))

	api := checkr.New()

	cd := &checkr.Candidate{
		Email:                "john.smith@gmail.com",
		FirstName:            "John",
		MiddleName:           "Alfred",
		LastName:             "Smith",
		SocialSecurityNumber: "111-11-2001",
		ZipCode:              "90210",
		DateOfBirth:          "1970-01-01",
	}
	cd, err := api.CreateCandidate(cd)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(pretty.Sprint(cd))

	if got, exp := cd.FirstName, "John"; got != exp {
		t.Errorf("unexpected first_name.  got %s, exp %s", got, exp)
	}
	if got, exp := cd.MiddleName, "Alfred"; got != exp {
		t.Errorf("unexpected middle_name.  got %s, exp %s", got, exp)
	}
	if got, exp := cd.LastName, "Smith"; got != exp {
		t.Errorf("unexpected last_name.  got %s, exp %s", got, exp)
	}
	if got, exp := cd.Email, "john.smith@gmail.com"; got != exp {
		t.Errorf("unexpected email.  got %s, exp %s", got, exp)
	}
	if cd.ID == "" {
		t.Error("ID was blank.")
	}

	// Create a report
	r := &checkr.Report{
		CandidateID: cd.ID,
		Package:     checkr.PackageTaskerStd,
	}

	r, err = api.CreateReport(r)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(pretty.Sprint(r))

	if r.ID == "" {
		t.Error("blank ID")
	}

	// retrieve the report

}
