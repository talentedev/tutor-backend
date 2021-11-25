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

func Test_Report_Create_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() err: %v", err)
		}

		// return a standard report
		json, err := os.Open("./testdata/report.json")
		if err != nil {
			t.Fatal(err)
		}
		defer json.Close()
		io.Copy(w, json)
	}))
	defer ts.Close()

	config.GetConfig().Set("service.checkr.key", envy.Get("CHECKR_API_KEY", ""))
	r := &Report{}
	r, err := createReport(ts.URL, r)
	if err != nil {
		t.Fatal(err)
	}

	if r.ID == "" {
		t.Error("blank ID")
	}
	if got, exp := r.Status, ReportStatus("clear"); got != exp {
		t.Errorf("unexpected status.  got %s, exp %s", got, exp)
	}
}

func Test_Report_Retrieve_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() err: %v", err)
		}

		// return a standard report
		json, err := os.Open("./testdata/report.json")
		if err != nil {
			t.Fatal(err)
		}
		defer json.Close()
		io.Copy(w, json)
	}))
	defer ts.Close()

	config.GetConfig().Set("service.checkr.key", envy.Get("CHECKR_API_KEY", ""))
	r := &Report{}
	r, err := retrieveReport(ts.URL, "1234")
	if err != nil {
		t.Fatal(err)
	}

	if r.ID == "" {
		t.Error("blank ID")
	}
	if got, exp := r.Status, ReportStatus("clear"); got != exp {
		t.Errorf("unexpected status.  got %s, exp %s", got, exp)
	}
}
