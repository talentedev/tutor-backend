package checkr

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gobuffalo/envy"
	"gitlab.com/learnt/api/config"
)

func Test_API_Endpoint_Failures(t *testing.T) {
	t.Parallel()

	// create our statusInterface to get the status code
	type se interface {
		StatusCode() int
	}

	// endpoints we need to test.
	endpoints := []struct {
		call func(string) error
	}{
		{
			call: func(url string) error {
				cd := &Candidate{}
				_, err := createCandidate(url, cd)
				return err
			},
		},
		{
			call: func(url string) error {
				_, err := retrieveCandidate(url, "1234")
				return err
			},
		},
		{
			call: func(url string) error {
				_, err := listCandidates(url, nil, nil)
				return err
			},
		},
		{
			call: func(url string) error {
				r := Report{}
				_, err := createReport(url, &r)
				return err
			},
		},
		{
			call: func(url string) error {
				_, err := retrieveReport(url, "")
				return err
			},
		},
	}

	for _, endpoint := range endpoints {
		tests := []struct {
			name       string
			body       string
			statusCode int
		}{
			{
				name:       "status ok - invalid json",
				statusCode: http.StatusOK,
				body:       `junk`,
			},

			{
				name:       "not found",
				statusCode: http.StatusNotFound,
				body:       `{"error":"not found"}`,
			},
			{
				name:       "not found",
				statusCode: http.StatusInternalServerError,
			},
			{
				name:       "not found",
				statusCode: 600,
			},
		}
		for _, test := range tests {
			test := test // capture range variable
			call := endpoint.call
			t.Run(test.name, func(st *testing.T) {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(test.statusCode)
					w.Write([]byte(test.body))
				}))
				defer ts.Close()
				config.GetConfig().Set("service.checkr.key", envy.Get("CHECKR_API_KEY", ""))
				err := call(ts.URL)
				if err == nil {
					st.Fatal("expected error, got none")
				}
				// All of these errors should return status errors to ensure we can
				// inspect it further if needed
				if _, ok := err.(se); !ok {
					st.Errorf("expected typed error of statusError")
				}
			})
		}
	}
}
