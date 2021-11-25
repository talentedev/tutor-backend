package sms

import (
	"gitlab.com/learnt/api/config"
	m "gitlab.com/learnt/api/pkg/utils/messaging"
	"testing"
)

var test_num = "+12243238312" // https://www.receivesms.co/us-phone-number/3017/

func TestSMS(t *testing.T) {
	conf := config.GetConfig()
	conf.App.Payload = "../../../../payload/payload.json" // hijack this since we use relative path
	sender := NewTwilioSender(conf)
	err := sender.SendTo(test_num, m.TPL_JOIN_INVITATION, &m.P{
		"FULL_NAME": "tester",
		"promo_url": "test url",
	})

	if err != nil {
		t.Fatal(err)
	}
}
