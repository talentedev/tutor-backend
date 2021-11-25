package twilio

import (
	"gitlab.com/learnt/api/config"
	"testing"
)

var params map[string]string
var ServiceSid = ""
var SessionSid = ""

func init() {
	conf := config.GetConfig()
	params = make(map[string]string)
	params["SID"] = conf.Service.Twilio.Account
	params["TOKEN"] = conf.Service.Twilio.AccountToken
	params["FROM"] = conf.Service.Twilio.Phone
	params["TO"] = "+12243238312" // https://www.receivesms.co/us-phone-number/3017/
}

func TestSMS(t *testing.T) {
	msg := "Welcome to Learnt"
	twilio := New(params["SID"], params["TOKEN"])
	_, exc, err := twilio.SendSMS(params["FROM"], params["TO"], msg, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if exc != nil {
		t.Fatal(exc)
	}
}

func TestMMS(t *testing.T) {
	msg := "Welcome to Learnt"
	twilio := New(params["SID"], params["TOKEN"])
	file := []string{"https://www.google.com/images/logo.png"}
	_, exc, err := twilio.SendMMS(params["FROM"], params["TO"], msg, file, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if exc != nil {
		t.Fatal(exc)
	}
}

func TestVoice(t *testing.T) {
	callback := NewCallbackParameters("http://example.com")
	twilio := New(params["SID"], params["TOKEN"])
	_, exc, err := twilio.CallWithUrlCallbacks(params["FROM"], params["TO"], callback)
	if err != nil {
		t.Fatal(err)
	}

	if exc != nil {
		t.Fatal(exc)
	}
}
