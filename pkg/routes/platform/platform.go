package platform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/ws"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/now"
	"gopkg.in/mgo.v2/bson"
)

const collectionName = "settings"

type Setting struct {
	Type        string
	Name        string
	Description string
	Value       interface{}
	UI          bool
}

func (s Setting) Int() int {

	value, isInt := s.Value.(int)

	if !isInt {
		panic(fmt.Sprintf("Setting %s is not int", s.Name))
	}

	return value
}

func (s Setting) Bool() bool {

	value, isBool := s.Value.(bool)

	if !isBool {
		panic(fmt.Sprintf("Setting %s is not bool", s.Name))
	}

	return value
}

func (s Setting) String() string {

	value, isString := s.Value.(string)

	if !isString {
		panic(fmt.Sprintf("Setting %s is not string", s.Name))
	}

	return value
}

var settingsCache []Setting = make([]Setting, 0)
var settingsCacheExpire time.Time

// Get setting from platform
func GetSetting(name string, valueDefault interface{}) *Setting {

	if time.Now().After(settingsCacheExpire) {
		store.GetCollection(collectionName).Find(bson.M{}).All(&settingsCache)
		settingsCacheExpire = time.Now().Add(time.Minute)
	}

	for _, setting := range settingsCache {
		if setting.Name == name {
			return &setting
		}
	}

	return &Setting{Name: name + ".__DEFAULT__", Value: valueDefault}
}

func getSettings(c *gin.Context) {
	var settings = make([]interface{}, 0)
	store.GetCollection(collectionName).Find(bson.M{}).All(&settings)
	c.JSON(http.StatusOK, settings)
}

func getUISettings(c *gin.Context) {
	var settings = make([]interface{}, 0)
	err := store.GetCollection(collectionName).Pipe([]bson.M{
		{"$match": bson.M{"ui": bson.M{"$exists": true, "$eq": true}}},
		{
			"$project": bson.M{
				"name":  1,
				"value": 1,
			},
		},
	}).All(&settings)

	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	c.JSON(http.StatusOK, settings)
}

func updateSettings(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	if !user.HasRole(store.RoleRoot) {
		c.JSON(
			http.StatusUnauthorized,
			core.NewErrorResponse(
				"Only root account is authorized to change platform settings",
			),
		)
		return
	}

	var request = make(map[string]interface{}, 0)

	if err := c.BindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	for prop, value := range request {
		store.GetCollection(collectionName).Update(
			bson.M{"name": prop},
			bson.M{"$set": bson.M{"value": value}},
		)
	}

	getSettings(c)
}

func getUploads(c *gin.Context) {
	c.JSON(http.StatusOK, services.Uploads.GetTempUploads())
}

func isLoggedAdmin(c *gin.Context) {
	auth.Middleware(c)
	if !c.IsAborted() {
		auth.IsAdminMiddleware(c)
	}
}

func getStats(c *gin.Context) {
	_, exist := store.GetUser(c)
	if !exist {
		return
	}

	users := services.NewUsers()

	pendingTutors, _ := users.PendingTutors()
	unverifiedTutors, _ := users.RequiresVerification()
	currentRunningLessons, _ := services.GetLessons().GetCurrentRunningLessons(0, 10000)

	c.JSON(http.StatusOK, map[string]interface{}{
		"users": map[string]interface{}{
			"tutors":     users.Count(store.RoleTutor),
			"students":   users.Count(store.RoleStudent),
			"admins":     users.Count(store.RoleAdmin),
			"root":       users.Count(store.RoleRoot),
			"pending":    len(pendingTutors),
			"unverified": len(unverifiedTutors),
		},
		"socket":         ws.GetEngine().Hub.Info(),
		"vcr":            services.VCRInstance().Stats(),
		"currentLessons": currentRunningLessons.Length,
	})
}

type Subject struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

type Location struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type FooterLinksUpdatePayload struct {
	Subjects  []Subject  `json:"subjects"`
	Locations []Location `json:"locations"`
}

func updateFooterLinks(c *gin.Context) {

	_, exist := store.GetUser(c)
	if !exist {
		return
	}

	r := FooterLinksUpdatePayload{}

	if err := c.BindJSON(&r); err != nil {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	data, err := json.Marshal(r)
	if err != nil {
		c.JSON(
			500,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	_, err = store.GetCollection(collectionName).Upsert(
		bson.M{"name": "footer_links"},
		bson.M{"$set": bson.M{
			"value": string(data),
			"type":  "string",
			"ui":    true,
		}},
	)

	if err != nil {
		c.JSON(
			500,
			core.NewErrorResponse(
				err.Error(),
			),
		)
	}
}

var Build string = ""
var Version string = ""

func getStatus(c *gin.Context) {

	status := map[string]interface{}{
		"build": Build,
	}

	if Version != "n/a" {
		status["version"] = Version
	}

	c.JSON(200, status)
}

func getCreditsSummary(c *gin.Context) {

	from, err := time.Parse(time.RFC3339Nano, c.Query("from"))
	if err != nil {
		from = now.New(time.Now()).BeginningOfYear()
	}

	to, err := time.Parse(time.RFC3339Nano, c.Query("to"))
	if err != nil {
		to = now.New(time.Now()).EndOfYear()
	}

	var transactions []*store.TransactionDto
	transactions = services.GetTransactions().GetCreditSummaryTransactions(from, to)

	c.JSON(http.StatusOK, transactions)
}

// Setup adds the platform routes to the router
func Setup(g *gin.RouterGroup, version string, build string) {

	Build = build
	Version = version

	g.GET("/status", getStatus)
	g.GET("/settings", isLoggedAdmin, getSettings)
	g.GET("/settings/ui", getUISettings)
	g.PUT("/settings", isLoggedAdmin, updateSettings)
	g.PUT("/footer-links", isLoggedAdmin, updateFooterLinks)
	g.GET("/uploads", isLoggedAdmin, getUploads)
	g.GET("/stats", isLoggedAdmin, getStats)
	g.GET("/credits-summary", isLoggedAdmin, getCreditsSummary)

	// Put required settings
	store.GetCollection(collectionName).Insert(

		// Ui is using this to show beta content
		Setting{Name: "pre-beta", Value: true, UI: true, Description: "In pre beta users are allowed only to register."},
	)
}
