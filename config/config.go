//Package config initializes a default configuration based on config.sample.yml
// We've implemented a Config struct that embeds viper. We unmarshall the yaml file to this struct to make it easier to access without wild-guessing
// using GetConfig().GetString('variable'), and then doing tedious type-juggling.
// We've embedded viper on this config so it's still possible to do GetConfig().GetString('variable'), etc (especially if you've just wanted
// to quickly try out a new API).
package config

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/linkedin"

	"gitlab.com/learnt/api/pkg/logger"
)

var conf *Config
var oauthConf map[string]*oauth2.Config
var once sync.Once
var mu *sync.RWMutex

type Config struct {
	*viper.Viper           // embed this in case there's new configuration added by someone else and no struct is defined
	App          App       `mapstructure:"app"`
	Storage      Storage   `mapstructure:"storage"`
	Security     Security  `mapstructure:"security"`
	Websocket    Websocket `mapstructure:"websocket"`
	Payments     Payments  `mapstructure:"payments"`
	Mail         Mail      `mapstructure:"mail"`
	Lessons      Lessons   `mapstructure:"lessons"`
	Messenger    Messenger `mapstructure:"messenger"`
	Service      Service   `mapstructure:"service"`
}

type App struct {
	Listen      string `mapstructure:"listen"`
	Data        string `mapstructure:"data"`
	SearchMiles int    `mapstructure:"search_miles"`
	Env         string `mapstructure:"env"`
	Debug       bool   `mapstructure:"debug"`
	Payload     string `mapstructure:"payload"`
	Temp        string `mapstructure:"temp"`
	Jobs        bool   `mapstructure:"jobs"`
}

type Storage struct {
	Uri         string `mapstructure:"uri"`
	Database    string `mapstructure:"database"`
	User        string `mapstructure:"user"`
	Password    string `mapstructure:"password"`
	SSL         bool   `mapstructure:"ssl"`
	Certificate string `mapstructure:"certificate"`
	Timeout     int    `mapstructure:"timeout"`
}

type Security struct {
	Token string `mapstructure:"token"`
}

type Websocket struct {
	Origins string `mapstructure:"origins"`
}

type Payments struct {
	Commission            int `mapstructure:"commission"`
	StudentReferralReward int `mapstructure:"student_referral_reward"`
	StudentSignupReward   int `mapstructure:"student_signup_reward"`
	TutorReferralReward   int `mapstructure:"tutor_referral_reward"`
}

type Mail struct {
	SMTP SMTP `mapstructure:"smtp"`
	From From `mapstructure:"from"`
}

type SMTP struct {
	Host string `mapstructure:"host"`
	User string `mapstructure:"user"`
	Pass string `mapstructure:"pass"`
}

type From struct {
	Email string `mapstructure:"email"`
	Name  string `mapstructure:"name"`
}

type Lessons struct {
	AdvanceDuration string `mapstructure:"advance_duration"`
}

type Service struct {
	Twitter  Twitter  `mapstructure:"twitter"`
	Linkedin Linkedin `mapstructure:"linkedin"`
	Facebook Facebook `mapstructure:"facebook"`
	Google   Google   `mapstructure:"google"`
	Outlook  Outlook  `mapstructure:"outlook"`
	Yahoo    Yahoo    `mapstructure:"yahoo"`
	Amazon   Amazon   `mapstructure:"amazon"`
	Stripe   Stripe   `mapstructure:"stripe"`
	Twilio   Twilio   `mapstructure:"twilio"`
	Mandrill Mandrill `mapstructure:"mandrill"`
	Checkr   Checkr   `mapstructure:"checkr"`
	BGCheck  BGCheck  `mapstructure:"bgcheck"`
}

type Twitter struct {
	Key    string `mapstructure:"key"`
	Secret string `mapstructure:"secret"`
}

type Linkedin struct {
	Key    string `mapstructure:"key"`
	Secret string `mapstructure:"secret"`
}

type Facebook struct {
	Key    string `mapstructure:"key"`
	Secret string `mapstructure:"secret"`
}

type Google struct {
	Key    string `mapstructure:"key"`
	Secret string `mapstructure:"secret"`
}

type Outlook struct {
	Key    string `mapstructure:"key"`
	Secret string `mapstructure:"secret"`
}

type Yahoo struct {
	Key    string `mapstructure:"key"`
	Secret string `mapstructure:"secret"`
}

type Amazon struct {
	Key    string `mapstructure:"key"`
	Secret string `mapstructure:"secret"`
	Bucket string `mapstructure:"bucket"`
}

type Stripe struct {
	Key                     string `mapstructure:"key"`
	Secret                  string `mapstructure:"secret"`
	CorporateChargeCustomer string `mapstructure:"coporate_charge_customer"`
}

type Twilio struct {
	Account      string `mapstructure:"account"`
	AccountToken string `mapstructure:"account_token"`
	Token        string `mapstructure:"token"`
	Secret       string `mapstructure:"secret"`
	Phone        string `mapstructure:"phone"`
}

type Mandrill struct {
	Key  string `mapstructure:"key"`
	User string `mapstructure:"user"`
}

type Checkr struct {
	Key string `mapstructure:"key"`
}

type BGCheck struct {
	Auth    string ` mapstructure:"auth"`
	Package string ` mapstructure:"package"`
}

func (l Lessons) ParseAdvanceDuration() (time.Duration, error) {
	return time.ParseDuration(l.AdvanceDuration)
}

type Messenger struct {
	RequireApproval bool `mapstructure:"require_approval"`
}

func init() {
	if conf == nil {
		var err error
		conf, err = LoadConfig()
		if err != nil {
			log.Fatal(err)
		}
	}

	if oauthConf == nil {
		mu = new(sync.RWMutex)
		loginURL := appURL("/start/redirect")
		oauthConf = make(map[string]*oauth2.Config)
		oauthConf["google"] = &oauth2.Config{
			ClientID:     conf.Service.Google.Key,
			ClientSecret: conf.Service.Google.Secret,
			RedirectURL:  loginURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
			},
			Endpoint: google.Endpoint,
		}

		oauthConf["facebook"] = &oauth2.Config{
			ClientID:     conf.Service.Facebook.Key,
			ClientSecret: conf.Service.Facebook.Secret,
			RedirectURL:  loginURL,
			Scopes: []string{
				"email",
			},
			Endpoint: facebook.Endpoint,
		}

		oauthConf["linkedin"] = &oauth2.Config{
			ClientID:     conf.Service.Linkedin.Key,
			ClientSecret: conf.Service.Linkedin.Secret,
			RedirectURL:  loginURL,
			Scopes: []string{
				"r_emailaddress",
			},
			Endpoint: linkedin.Endpoint,
		}

	}
}

func appURL(endpoint string) string {
	var domain string
	if conf.App.Env == "www" {
		domain = "https://learnt.io"
	}

	if conf.App.Env == "next" {
		domain = "https://next.learnt.io"
	}

	if conf.App.Env == "dev" {
		domain = "https://localhost:4200"
	}
	if len(endpoint) > 1 {
		if endpoint[0] != '/' {
			endpoint = "/" + endpoint
		}
	}

	return fmt.Sprintf(domain + endpoint)
}

func isDebugging() bool {
	if envDebug := os.Getenv("DEBUG"); envDebug != "" {
		return envDebug == "true"
	}

	return false
}

// GetConfig returns the currently initialized app configuration
func GetConfig() *Config {
	return conf
}

func GetOAuthConfig(key string) *oauth2.Config {
	mu.RLock()
	v, _ := oauthConf[key]
	mu.RUnlock()
	return v
}

func New() *Config {
	once.Do(func() {
		conf = &Config{Viper: viper.New()}
	})

	return conf
}

// Load config accepts 0 or 1 filename strings, loads the configuration, and watches the config file for changes.
// If no filename is provided it will attempt to use the filename defined in env variable CONFIG.
// If no env var is defined, it will use the configuration in ../config.sample.yml
func LoadConfig(filename ...string) (*Config, error) {
	conf = New()

	var configFile string = "../config.sample.yml"

	if filename != nil {
		if len(filename) > 1 {
			return nil, errors.New("Multiple config files not allowed")
		}
		configFile = filename[0]
	} else {
		if os.Getenv("CONFIG") != "" {
			configFile = os.Getenv("CONFIG")
		}
	}

	conf.Set("env", os.Getenv("env"))
	conf.SetConfigFile(configFile)
	if err := conf.ReadInConfig(); err != nil {
		return nil, err
	}

	if err := conf.Unmarshal(conf); err != nil {
		return nil, err
	}
	WatchConfig()
	return conf, nil
}

// Loads config file from a given io.Reader and config type/extension (e.g. "yaml")
func LoadConfigFromReader(in io.Reader, t string) (*Config, error) {
	if t == "" {
		return nil, errors.New("file type/extension is required")
	}

	conf = New()
	conf.Set("env", os.Getenv("env"))
	conf.SetConfigType(t)
	if err := conf.ReadConfig(in); err != nil {
		return nil, err
	}

	if err := conf.Unmarshal(conf); err != nil {
		return nil, err
	}
	// WatchConfig() // dont watch changes from an io.Reader as it doesn't have the location which is required by WatchConfig
	return conf, nil
}

func WatchConfig() {
	conf.WatchConfig()
	// https://github.com/gohugoio/hugo/blob/master/watcher/batcher.go
	// https://github.com/spf13/viper/issues/609
	// for some reason this fires twice on a Win machine, and the way some editors save files.
	conf.OnConfigChange(func(e fsnotify.Event) {
		logger.Get().Info("Configuration has been changed...")
		// only re-read if file has been modified
		if err := conf.ReadInConfig(); err != nil {
			if err == nil {
				logger.Get().Error("Reading failed after configuration update: no data was read")
			} else {
				panic(fmt.Sprintf("Reading failed after configuration update: %s \n", err))
			}

			return
		} else {
			logger.Get().Info("Successfully re-read config file...")
		}

	})
}
