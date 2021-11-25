package lessons

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

func setupDB(t *testing.T) {
	/* use this if you want to test the next.learnt.io database
	// Lookup or create these credentials on:
	// https://app.compose.com/tutor-the-people/deployments/learnt/mongodb/databases/admin/users
	database := ""
	username := ""
	password := ""
	server := "portal-ssl1863-25.learnt.997166227.composedb.com:33719"
	core.Config.Storage.SSL = true
	core.Config.Storage.URL = fmt.Sprintf("mongodb://%s:%s@%s/%s?authSource=admin", username, password, server, database)
	// */

	dialSession, err := store.NewSession()
	if err != nil {
		t.Skip("Database not available")
	}
	defer dialSession.Close()
	store.Init()
}

func setupGin() (*gin.Context, *httptest.ResponseRecorder) {
	if !testing.Verbose() {
		// set the gin mode to test, this is what actually turns off the log output.. ugg
		gin.SetMode(gin.TestMode)
		gin.DefaultWriter = ioutil.Discard
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

func TestGet(t *testing.T) {
	setupDB(t)
	c, w := setupGin()

	userID := "5c92635bd0330e1fa6c1e9e4"
	c.Set("user", &store.UserMgo{ID: bson.ObjectIdHex(userID)})

	now := time.Now()
	week := time.Hour * 24 * 7
	firstday := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	lastday := firstday.AddDate(0, 1, 0).Add(time.Nanosecond * -1)

	vals := url.Values{}
	vals.Add("state", "all")
	vals.Add("from", firstday.Add(-1*week).Format(time.RFC3339))
	vals.Add("to", lastday.Add(week).Format(time.RFC3339))
	vals.Add("limit", "1")
	uri := "/lessons?" + vals.Encode()

	u, err := url.Parse(uri)
	if err != nil {
		t.Fatal("Could not parse the url", err)
	}
	c.Request = &http.Request{URL: u}

	before := time.Now()
	get(c)
	t.Log("get time:", time.Now().Sub(before))

	if w.Code != 200 {
		b, _ := ioutil.ReadAll(w.Body)
		t.Fatal(w.Code, string(b))
	}

	var pages paginatedObscuredLessons
	if err := json.Unmarshal(w.Body.Bytes(), &pages); err != nil {
		t.Error("could not unmarshal lessons into the right struct", err)
	}
}
