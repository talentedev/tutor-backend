package checkr

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gobuffalo/envy"
	"gitlab.com/learnt/api/config"
)

func Test_Candidate_Create_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() err: %v", err)
		}

		// return a standard candidate
		json, err := os.Open("./testdata/candidate.json")
		if err != nil {
			t.Fatal(err)
		}
		defer json.Close()
		io.Copy(w, json)
	}))
	defer ts.Close()

	config.GetConfig().Set("service.checkr.key", envy.Get("CHECKR_API_KEY", ""))
	cd := &Candidate{}
	cd, err := createCandidate(ts.URL, cd)
	if err != nil {
		t.Fatal(err)
	}

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
}

func Test_Candidate_Retrieve_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() err: %v", err)
		}

		// return a standard candidate
		json, err := os.Open("./testdata/candidate.json")
		if err != nil {
			t.Fatal(err)
		}
		defer json.Close()
		io.Copy(w, json)
	}))
	defer ts.Close()

	config.GetConfig().Set("service.checkr.key", envy.Get("CHECKR_API_KEY", ""))
	cd, err := retrieveCandidate(ts.URL, "e44aa283528e6fde7d542194")
	if err != nil {
		t.Fatal(err)
	}

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
}

func Test_Candidate_List_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() err: %v", err)
		}

		// return a standard candidate
		json, err := os.Open("./testdata/candidates.json")
		if err != nil {
			t.Fatal(err)
		}
		defer json.Close()
		io.Copy(w, json)
	}))
	defer ts.Close()

	config.GetConfig().Set("service.checkr.key", envy.Get("CHECKR_API_KEY", ""))
	cdl, err := listCandidates(ts.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if got, exp := len(cdl.Candidates), 2; got != exp {
		t.Fatalf("unexpected canidate count.  got %d, exp %d", got, exp)
	}

	cd := cdl.Candidates[0]

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
}
