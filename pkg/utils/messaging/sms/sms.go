package sms

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/utils/messaging"

	// "regexp"
	"strings"
)

type (
	P            = messaging.P
	Tpl          = messaging.Tpl
	Sender       = messaging.Sender
	UserProvider = messaging.UserProvider
)

var (
	malformedJsonErr = errors.New("Malformed json file")
	Templates        map[Tpl]map[string]interface{}
)

type User struct {
	Telephone string
	FirstName string
}

func (u *User) To() string {
	return u.Telephone
}

func (u *User) GetFirstName() string {
	return u.FirstName
}

func GetSender(conf *config.Config) Sender {
	return NewTwilioSender(conf)
}

func LoadFile(file string) {
	absPath, err := filepath.Abs(file)
	if err != nil {
		logger.Get().Fatal(err)
	}
	f, err := os.Open(absPath)
	if err != nil {
		logger.Get().Fatal(err)
	}
	dec := json.NewDecoder(f)
	for {
		if err := dec.Decode(&Templates); err == io.EOF {
			break
		} else if err != nil {
			logger.Get().Fatal(err)
		}
	}

	if err := f.Close(); err != nil {
		logger.Get().Fatal(err)
	}
}

func getTemplateBody(template Tpl, params *P) string {
	tmp := getTemplate(template)
	if tmp == nil {
		// ensure you have the corresponding payload template in payload/payload.json
		return ""
	}
	return getMappedTemplateBody(tmp, *params)
}

func getTemplate(template Tpl) map[string]interface{} {
	v, found := Templates[template]
	if found {
		return v
	}

	return nil
}

func getMappedTemplateBody(tmp map[string]interface{}, params P) string {
	copy := make(map[string]interface{})
	for k, v := range tmp {
		copy[k] = v
	}

	if _, ok := copy["variables"]; !ok {
		logger.Get().Fatal(malformedJsonErr)
	}

	if _, ok := copy["body"]; !ok {
		logger.Get().Fatal(malformedJsonErr)
	}

	for _, k := range copy["variables"].([]interface{}) {
		key := strings.ToUpper(k.(string))
		// case checking on params
		val, ok := params[key]
		if !ok {
			val, ok = params[k.(string)]
		}

		if ok {
			copy["body"] = strings.ReplaceAll(copy["body"].(string), strings.ToUpper(k.(string)), val)
			copy["body"] = strings.ReplaceAll(copy["body"].(string), k.(string), val)
			// replacer := regexp.MustCompile("(?i)" + k.(string))
			// copy["body"] = replacer.ReplaceAllString(copy["body"].(string), val)
		}
	}

	return copy["body"].(string)
}
