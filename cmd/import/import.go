package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/learnt/api/pkg/store"

	"github.com/pkg/errors"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// Database configuration
const (
	HOST     = "localhost"
	DATABASE = "test"
	USER     = "learnt"
	PASSWORD = "learnt"
	// COMMENT: Uncomment if you prefer to use the lingo strong
	// URL      = "mongodb://nerdly:nerdly@host1.composedb.com:33719,host2.composedb.com:33719/compose?authSource=our-db"
	URL = "mongodb://localhost:27017/learnt"
)

var session *mgo.Session

func collection(name string) *mgo.Collection {
	return session.DB(DATABASE).C(name)
}

func universities() {
	var c = collection("universities")

	err := c.EnsureIndex(mgo.Index{
		Name: "name",
		Key:  []string{"name"},
	})

	if err != nil {
		panic(errors.Wrap(err, "error ensuring index for universities"))
	}

	// Import universities
	jsonfile, err := filepath.Abs("data/universities.json")

	if err != nil {
		panic(errors.Wrap(err, "error reading universities file"))
	}

	data, err := ioutil.ReadFile(jsonfile)
	if err != nil {
		panic(errors.Wrap(err, "error reading universities from file"))
	}

	universities := make([]store.University, 0)

	if err := json.Unmarshal(data, &universities); err != nil {
		panic(errors.Wrap(err, "error reading universities from json"))
	}

	var toInsert = make([]interface{}, len(universities))
	for index := range universities {
		universities[index].ID = bson.NewObjectId()
		toInsert[index] = universities[index]
	}

	if errIns := c.Insert(toInsert...); errIns != nil {
		panic(errIns)
	}
}

// Country is a country
type Country struct {
	ID        bson.ObjectId `bson:"_id" json:"_id"`
	Code      string        `bson:"code" json:"code"`
	Name      string        `bson:"name" json:"name"`
	Native    string        `bson:"native" json:"native"`
	Phone     string        `bson:"phone" json:"phone"`
	Continent string        `bson:"continent" json:"continent"`
	Capital   string        `bson:"capital" json:"capital"`
	Currency  string        `bson:"currency" json:"currency"`
	Languages string        `bson:"languages" json:"languages"`
}

// City is a city
type City struct {
	ID      bson.ObjectId `bson:"_id" json:"_id"`
	Country bson.ObjectId `bson:"country" json:"country"`
	Name    string        `bson:"name" json:"name"`
}

func readRemoteJSON(url string, data interface{}) {

	resp, err := http.Get(url)
	if err != nil {
		panic(errors.Wrap(err, "error making GET request"))
	}

	jsondata, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(errors.Wrap(err, "error reading response body"))
	}

	if err := json.Unmarshal(jsondata, &data); err != nil {
		fmt.Println(string(jsondata))
		panic(errors.Wrap(err, "Fail to unmarshal json"))
	}
}

/**
 * - https://github.com/lutangar/cities.json
 * - https://raw.githubusercontent.com/annexare/Countries/master/data/countries.json
 */
func countriesAndCities() {
	var data map[string]interface{}
	var countries = make([]Country, 0)

	readRemoteJSON("https://raw.githubusercontent.com/annexare/Countries/master/data/countries.json", &data)

	for code, country := range data {
		countryData := country.(map[string]interface{})
		c := Country{
			ID:        bson.NewObjectId(),
			Code:      code,
			Name:      countryData["name"].(string),
			Native:    countryData["native"].(string),
			Phone:     countryData["phone"].(string),
			Continent: countryData["continent"].(string),
			Capital:   countryData["capital"].(string),
			Currency:  countryData["currency"].(string),
		}

		var langs []string
		for _, lang := range countryData["languages"].([]interface{}) {
			langs = append(langs, lang.(string))
		}
		c.Languages = strings.Join(langs, ",")

		countries = append(countries, c)
	}

	countryByCode := func(code string) *Country {
		for _, c := range countries {
			if c.Code == code {
				return &c
			}
		}
		return nil
	}

	var citiesRaw []map[string]string
	var cities = make([]City, 0)

	readRemoteJSON("https://raw.githubusercontent.com/lutangar/cities.json/master/cities.json", &citiesRaw)

	for _, item := range citiesRaw {
		country := countryByCode(item["country"])
		if country == nil {
			fmt.Println("Country not found:", item["country"], "for city name:", item["name"])
			continue
		}

		city := City{
			ID:      bson.NewObjectId(),
			Country: country.ID,
			Name:    item["name"],
		}

		cities = append(cities, city)
	}

	c1 := collection("countries")
	c2 := collection("cities")

	c1.DropCollection()
	c2.DropCollection()

	for _, country := range countries {
		fmt.Println("Import country: ", country.Name)
		c1.Insert(country)
	}

	for _, city := range cities {
		fmt.Println("Import city: ", city.Name)
		c2.Insert(city)
	}

	fmt.Println("Countries:", len(countries))
	fmt.Println("Cities:", len(cities))
}

func subjects() {
	var c = collection("subjects")
	c.DropCollection()

	err := c.EnsureIndex(mgo.Index{
		Name: "subject",
		Key:  []string{"subject"},
	})

	if err != nil {
		panic(errors.Wrap(err, "error ensuring index for subjects"))
	}

	// Import subjects
	jsonfile, err := filepath.Abs("data/subjects.json")
	if err != nil {
		panic(errors.Wrap(err, "error reading subjects.json file"))
	}

	data, err := ioutil.ReadFile(jsonfile)
	if err != nil {
		panic(errors.Wrap(err, "error reading subjects from file"))
	}

	subjects := make([]store.Subject, 0)

	if err := json.Unmarshal(data, &subjects); err != nil {
		panic(errors.Wrap(err, "error reading subjects from json"))
	}

	var toInsert = make([]interface{}, len(subjects))
	for index := range subjects {
		if subjects[index].ID.String() == `ObjectIdHex("")` {
			subjects[index].ID = bson.NewObjectId()
		}
		fmt.Println("Importing Subject: ", subjects[index].Name, subjects[index].ID)
		toInsert[index] = subjects[index]
	}

	if errIns := c.Insert(toInsert...); errIns != nil {
		panic(errIns)
	}
}

func users() {
	var c = collection("users")
	c.DropCollection()

	// Import users
	jsonfile, err := filepath.Abs("data/users.json")
	if err != nil {
		panic(errors.Wrap(err, "error reading users.json file"))
	}

	data, err := ioutil.ReadFile(jsonfile)
	if err != nil {
		panic(errors.Wrap(err, "error reading users from file"))
	}

	users := make([]store.UserMgo, 0)

	if err := json.Unmarshal(data, &users); err != nil {
		panic(errors.Wrap(err, "error reading users from json"))
	}

	var toInsert = make([]interface{}, len(users))
	for index, user := range users {
		fmt.Println("Import user: ", user.Username)
		toInsert[index] = users[index]
	}

	if errIns := c.Insert(toInsert...); errIns != nil {

	}
}

func transactions() {
	var c = collection("transactions")
	c.DropCollection()

	// Import transactions
	jsonfile, err := filepath.Abs("data/transactions.json")
	if err != nil {
		panic(errors.Wrap(err, "error reading transactions.json file"))
	}

	data, err := ioutil.ReadFile(jsonfile)
	if err != nil {
		panic(errors.Wrap(err, "error reading transactions from file"))
	}

	transactions := make([]store.TransactionMgo, 0)

	if err := json.Unmarshal(data, &transactions); err != nil {
		panic(errors.Wrap(err, "error reading transactions from json"))
	}

	var toInsert = make([]interface{}, len(transactions))
	for index, txn := range transactions {
		fmt.Println("Import transaction: ", txn.Reference)
		toInsert[index] = transactions[index]
	}

	if errIns := c.Insert(toInsert...); errIns != nil {

	}
}

func settings() {
	var c = collection("settings")
	c.DropCollection()

	// Import settings
	jsonfile, err := filepath.Abs("data/settings.json")
	if err != nil {
		panic(errors.Wrap(err, "error reading settings.json file"))
	}

	data, err := ioutil.ReadFile(jsonfile)
	if err != nil {
		panic(errors.Wrap(err, "error reading settings from file"))
	}

	settings := make([]interface{}, 0)

	if err := json.Unmarshal(data, &settings); err != nil {
		panic(errors.Wrap(err, "error reading settings from json"))
	}

	fmt.Println("Importing Settings ", settings)

	if errIns := c.Insert(settings...); errIns != nil {
		panic(errors.Wrap(err, "error inserting settings data"))
	}
}

func lessons() {
	var c = collection("lessons")
	c.DropCollection()

	var subject *store.Subject

	if err := collection("subjects").Find(bson.M{}).One(&subject); err != nil {
		panic(errors.Wrap(err, "error finding subject"))
	}

	fmt.Println("Using Subject ID: ", subject.ID)

	// Import lesson
	jsonfile, err := filepath.Abs("data/lessons.json")
	if err != nil {
		panic(errors.Wrap(err, "error reading lesson.json file"))
	}

	data, err := ioutil.ReadFile(jsonfile)
	if err != nil {
		panic(errors.Wrap(err, "error reading lessons from file"))
	}

	lesson := make([]store.LessonMgo, 0)

	if err := json.Unmarshal(data, &lesson); err != nil {
		panic(errors.Wrap(err, "error reading lessons from json"))
	}

	var toInsert = make([]interface{}, len(lesson))
	for index, txn := range lesson {
		fmt.Println("Import lession: ", txn.ID)
		fmt.Println(subject.ID)
		txn.Subject = subject.ID
		// set the dates to now so that we always show data for today
		txn.StartsAt = time.Now().Add(time.Hour * time.Duration(index))
		txn.EndsAt = time.Now().Add(time.Hour * time.Duration((index + 1)))
		txn.CreatedAt = time.Now()
		fmt.Println(txn.CreatedAt)
		toInsert[index] = txn
	}

	if errIns := c.Insert(toInsert...); errIns != nil {
		panic(errors.Wrap(err, "error inserting lesson data"))
	}
}

func main() {

	var err error
	uri := &store.URI{
		Database: DATABASE,
		Hosts:    []string{HOST},
		//		Password: PASSWORD,
		//		User:     USER,
		SSL:         false,
		Certificate: "ssl/client.pem",
		//Options:  []string{"ssl=true", "sslInvalidHostNameAllowed=true"},
		URL: URL,
	}
	s := &store.Session{
		URI:     uri,
		Timeout: 25,
	}
	if err = s.LoadCertificate(); err != nil {
		fmt.Printf("Fail to load certificate to %s\n. Error: %v", uri.Certificate, err)
		os.Exit(1)
	}
	session, err = s.Open()
	if err != nil {
		fmt.Printf("Fail to connect to %s\n. Error: %v", HOST, err.Error())
		os.Exit(1)
	}

	fmt.Println("Connected to", HOST)

	subjects()
	users()
	lessons()
	settings()
	transactions()
	universities()
	// countriesAndCities()
	defer session.Close()
}
