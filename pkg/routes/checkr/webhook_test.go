package checkr

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

func Test_Checkr_Report(t *testing.T) {
	// check verbose, if not, shut down the gin output.
	if !testing.Verbose() {
		// set the gin mode to test, this is what actually turns off the log output.. ugg
		gin.SetMode(gin.TestMode)
		gin.DefaultWriter = ioutil.Discard
	}
	t.Parallel()
	tests := []struct {
		name string
		json string
		mock usersMock
		code int
	}{
		{
			name: "success",
			json: "./testdata/webhook_report.json",
			mock: usersMock{
				ByCandidateIDf: func(string) (*store.UserMgo, error) {
					return &store.UserMgo{
						ID: bson.NewObjectId(),
					}, nil
				},
				SetCheckrDataf: func(bson.ObjectId, *store.UserCheckrData) error {
					return nil
				},
			},
			code: http.StatusOK,
		},
		{
			name: "invalid event",
			json: "./testdata/webhook_report_invalid_event.json",
			code: http.StatusBadRequest,
		},
		{
			name: "invalid report object",
			json: "./testdata/webhook_report_invalid_object.json",
			code: http.StatusUnprocessableEntity,
		},
		{
			name: "invalid json",
			json: "./testdata/webhook_invalid.json",
			code: http.StatusBadRequest,
		},
		{
			name: "db save error",
			json: "./testdata/webhook_report.json",
			mock: usersMock{
				ByCandidateIDf: func(string) (*store.UserMgo, error) {
					return &store.UserMgo{
						ID: bson.NewObjectId(),
					}, nil
				},
				SetCheckrDataf: func(bson.ObjectId, *store.UserCheckrData) error {
					return errors.New("boom")
				},
			},
			code: http.StatusInternalServerError,
		},
	}

	for _, test := range tests {
		test := test // capture the range variable
		t.Run(test.name, func(st *testing.T) {
			st.Parallel()
			// load the test payload
			json, err := os.Open(test.json)
			if err != nil {
				st.Fatal(err)
			}
			defer json.Close()

			// initialize the handler
			h := &handler{
				Users: test.mock,
			}

			// mock the request
			req := httptest.NewRequest("GET", "/checkr_webhook", json)
			res := httptest.NewRecorder()
			router := gin.New()
			router.GET("/checkr_webhook", h.checkrWebhookHandler)

			// execute the route
			router.ServeHTTP(res, req)

			if got, exp := res.Code, test.code; got != exp {
				st.Fatalf("unexpected response code.  got %d, exp %d", got, exp)
			}
		})
	}
}

type usersMock struct {
	ByCandidateIDf func(string) (*store.UserMgo, error)
	SetCheckrDataf func(bson.ObjectId, *store.UserCheckrData) error
}

func (u usersMock) ByCandidateID(id string) (*store.UserMgo, error) {
	return u.ByCandidateIDf(id)
}

func (u usersMock) SetCheckrData(id bson.ObjectId, data *store.UserCheckrData) error {
	return u.SetCheckrDataf(id, data)
}
