package config

import (
	"strings"
	"testing"
	"time"
)

// taken from dummy_config.yml
// this should be enough to test string, int, bool and time.Duration types
var config = `
app:
  listen: 0.0.0.0:5050
  data: ./uploads
  search_miles: 50
  env: next
  debug: true

storage:
  uri: mongodb://localhost:27017/tutoring
  database: tutoring
  user:
  password:
  ssl: false
  certificate:
  timeout: 5

security:
  token: ]

websocket:
  origins: 

payments:
  commission: 30
  student_referral_reward: 10
  student_signup_reward: 10
  tutor_referral_reward: 50

mail:
  smtp:
    host: smtp.google.com
    user: test@gmail.com
    pass: test@gmail.com
  from:
    email: hello@ggg.io
    name: tutoring

lessons:
  advance_duration: 1m

service:
  google:
    key: 
    secret: 

dummy:
  dummy_token: test
`

func TestLoadConfigFromReader(t *testing.T) {
	in := strings.NewReader(config)
	_, err := LoadConfigFromReader(in, "yaml")
	if err != nil {
		t.Fatal("Could not load config file:", err)
	}
}

func TestLoadConfig(t *testing.T) {
	_, err := LoadConfig()
	if err != nil {
		t.Fatal("Could not load config file:", err)
	}
}

func TestConfigValues(t *testing.T) {
	c, _ := LoadConfig("dummy_config.yml")

	// test app portion
	if c.App.Listen != "0.0.0.0:5050" {
		t.Errorf("error reading app.Listen: got %s", c.App.Listen)
	}

	if c.App.Data != "./uploads" {
		t.Errorf("error reading app.Data: got %s", c.App.Data)
	}

	if c.App.Env != "next" {
		t.Errorf("error reading app.Env: got %s", c.App.Env)
	}

	if c.App.SearchMiles != 50 {
		t.Errorf("error reading app.SearchMiles: got %d", c.App.SearchMiles)
	}

	if !c.App.Debug {
		t.Errorf("error reading app.Debug: got %v", c.App.Debug)
	}

	// test storage portion
	if c.Storage.Uri != "mongodb://localhost:27017/tutoring" {
		t.Errorf("error reading storage.Uri: got %s", c.Storage.Uri)
	}

	if c.Storage.Database != "tutoring" {
		t.Errorf("error reading storage.Database: got %s", c.Storage.Database)
	}

	if c.Storage.User != "" {
		t.Errorf("error reading storage.User: got %s", c.Storage.User)
	}

	if c.Storage.Password != "" {
		t.Errorf("error reading storage.Password: got %s", c.Storage.Password)
	}

	if c.Storage.SSL {
		t.Errorf("error reading storage.SSL: got %v", c.Storage.SSL)
	}

	if c.Storage.Certificate != "" {
		t.Errorf("error reading storage.Certificate: got %s", c.Storage.Certificate)
	}

	if c.Storage.Timeout != 5 {
		t.Errorf("error reading storage.Timeout: got %d", c.Storage.Timeout)
	}

	// test security portion
	if c.Security.Token != "F41EF9AE1428F3A8" {
		t.Errorf("error reading security.Token: got %s", c.Security.Token)
	}

	// test websocket portion
	if c.Websocket.Origins != "" {
		t.Errorf("error reading websocket.Origins: got %s", c.Websocket.Origins)
	}

	// test payments portion
	if c.Payments.Commission != 30 {
		t.Errorf("error reading payments.Origins: got %d", c.Payments.Commission)
	}

	if c.Payments.StudentReferralReward != 10 {
		t.Errorf("error reading payments.StudentReferralReward: got %d", c.Payments.StudentReferralReward)
	}

	if c.Payments.StudentSignupReward != 10 {
		t.Errorf("error reading payments.StudentSignupReward: got %d", c.Payments.StudentSignupReward)
	}

	if c.Payments.TutorReferralReward != 50 {
		t.Errorf("error reading payments.TutorReferralReward: got %d", c.Payments.TutorReferralReward)
	}

	// test mail portion
	if c.Mail.SMTP.Host != "smtp.google.com" {
		t.Errorf("error reading mail.SMTP.Host: got %s", c.Mail.SMTP.Host)
	}

	if c.Mail.SMTP.User != "test@gmail.com" {
		t.Errorf("error reading mail.SMTP.User: got %s", c.Mail.SMTP.User)
	}

	if c.Mail.SMTP.Pass != "test@gmail.com" {
		t.Errorf("error reading mail.SMTP.Pass: got %s", c.Mail.SMTP.Pass)
	}

	if c.Mail.From.Email != "hello@tutoring.io" {
		t.Errorf("error reading mail.From.Email: got %s", c.Mail.From.Email)
	}

	if c.Mail.From.Name != "tutoring" {
		t.Errorf("error reading mail.From.Name: got %s", c.Mail.From.Name)
	}

	// test lessons portion
	td := 1 * time.Minute
	d, err := c.Lessons.ParseAdvanceDuration()
	if err != nil {
		t.Fatal("Could not load config file:", err)
	}
	if d != td {
		t.Errorf("error reading lessons.AdvanceDuration: got %s", d)
	}

	// test service portion
	if c.Service.Google.Key != "sdsd8j.apps.googleusercontent.com" {
		t.Errorf("error reading service.Google.Key: got %s", c.Service.Google.Key)
	}

	if c.Service.Google.Secret != "" {
		t.Errorf("error reading service.Google.secret: got %s", c.Service.Google.Secret)
	}

	// test dummy config portion
	if c.GetString("dummy.dummy_token") != "test" {
		t.Errorf("error reading dummy.dummy_token: got %s", c.GetString("dummy.dummy_token"))
	}

	// test that you can still pull up config values the old way
	if c.GetString("app.env") != c.App.Env {
		t.Errorf("error reading app.Env: got %s", c.GetString("app.env"))
	}
}
