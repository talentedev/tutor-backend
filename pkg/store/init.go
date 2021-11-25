package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/logger"
	"gopkg.in/mgo.v2"
)

var session *mgo.Session

func Seed() {

	seedUri := os.Getenv("SEED_URI")
	seedDb := os.Getenv("SEED_DB")
	seedDbLocal := os.Getenv("SEED_DB_LOCAL")
	seedUriSSL := os.Getenv("SEED_URI_SSL")

	if seedUri != "" && seedDb != "" {

		fmt.Printf("Are you sure you want to drop all collections and sync from: %s\nTo confirm type 's' then enter, to skip this operation just press enter: ", seedUri)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		fmt.Println(response)

		if strings.TrimSuffix(response, "\n") != "s" {
			return
		}

		fail := func(err error) {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		remote, err := SessionFrom(&URI{URL: seedUri, SSL: seedUriSSL == "true"}, config.GetConfig().GetInt("storage.timeout"))
		if err != nil {
			fail(err)
		}
		defer remote.Close()

		fmt.Println("Connected to remote...")

		remoteDb := remote.DB(seedDb)
		colsRemote, err := remoteDb.CollectionNames()
		if err != nil {
			fail(err)
		}

		fmt.Printf("Found %d collections.\n", len(colsRemote))

		if len(colsRemote) == 0 {
			os.Exit(0)
		}

		db := session.DB(seedDbLocal)
		cols, err := db.CollectionNames()
		if err != nil {
			fail(err)
		}

		for _, col := range cols {

			fmt.Printf("Removing local collection %s...", col)

			if err := db.C(col).DropCollection(); err == nil {
				fmt.Printf(" ✓")
			}

			fmt.Println("")
		}

		fmt.Println("Fetching data from remote...")

		for _, col := range colsRemote {
			data := make([]interface{}, 0)
			remoteDb.C(col).Find(nil).All(&data)
			fmt.Printf("Copy %d records from remote/%s...", len(data), col)
			db.C(col).Insert(data...)
			fmt.Println(" ✓")
		}

		os.Exit(0)
	}
}

// Init ensures the database is properly set up
func Init() {

	var err error
	if session, err = NewSession(); err != nil {
		panic(errors.Wrap(err, "error created NewSession()"))
	}

	Seed()

	indexesToEnsure := map[string][]mgo.Index{
		"users": {
			{
				Unique: true,
				Name:   "auth",
				Key:    []string{"username", "approval"},
			},
			{
				Unique: false,
				Name:   "loc",
				Key:    []string{"$2dsphere:location.position"},
			},
		},

		"files": {
			{
				Key: []string{"-created_at"},
			},
		},

		"lessons": {
			{
				Key: []string{"-starts_at"},
			},
		},

		"reviews": {
			{
				Key: []string{"user", "time", "approved"},
			},
		},

		"rooms": {
			{
				Name:   "lesson",
				Unique: true,
				Key:    []string{"lesson"},
			},
		},

		"settings": {
			{
				Key: []string{"ui"},
			},
			{
				Unique: true,
				Key:    []string{"name"},
			},
		},

		"subjects": {
			{
				Unique: true,
				Key:    []string{"subject"},
			},
		},

		"universities": {
			{
				Name:   "univ_unique",
				Unique: true,
				Key:    []string{"country", "name"},
			},
		},

		"transactions": {
			{
				Name: "trindx",
				Key:  []string{"user", "amount"},
			},
		},
	}

	for collection, indexes := range indexesToEnsure {
		c := GetCollection(collection)
		for _, index := range indexes {
			if err := c.EnsureIndex(index); err != nil {
				logger.Get().Errorf("Could not ensure index '%+v' in collectin '%s': %v\n", index, collection, err)
			}
		}
	}
}

// GetCollection ...
// TODO: Remove this function and use Collection (or rename it) and use the last one instead
// The goal is centralize all data access in one layer
func GetCollection(name string) *mgo.Collection {
	return session.DB(config.GetConfig().GetString("storage.database")).C(name)
}

// Destroy drops the database.
// TODO: Remove this function and use DropDB (or rename it) and use the last one instead
// The goal is centralice all data access in one layer
func Destroy() (err error) {
	if os.Getenv("ENV") != "testing" {
		panic("Database can be dropped only in testing environment")
	}

	return session.DB(config.GetConfig().GetString("storage.database")).DropDatabase()
}

// GetUser tries to get the user based on the context.
func GetUser(c *gin.Context) (user *UserMgo, exist bool) {
	if u, exist := c.Get("user"); exist {
		return u.(*UserMgo), true
	}

	return nil, false
}

// PrintExplaination should how mongo uses indexes queries
func PrintExplaination(name string, query *mgo.Query) {
	var explaination map[string]interface{}
	if err := query.Explain(&explaination); err != nil {
		log.Println("failed explaintion", name, err)
	}

	b, err := json.Marshal(explaination)
	if err != nil {
		log.Println("could not marshall explaintion", name, err)
	}
	log.Printf("explaination-%s:\n%s\n", name, string(b))
}
