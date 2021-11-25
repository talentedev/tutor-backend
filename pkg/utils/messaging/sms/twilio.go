package sms

import (
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/twilio"
)

type ExceptionStatus int

var _ Sender = &TwilioSender{}

const (
	InvalidRequest     ExceptionStatus = 400
	ResourceNotFound   ExceptionStatus = 404
	ServiceUnavailable ExceptionStatus = 503
)

type TwilioSender struct {
	conf   *config.Config
	twilio *twilio.Twilio
}

func NewTwilioSender(conf *config.Config) *TwilioSender {
	LoadFile(conf.App.Payload)
	return &TwilioSender{
		conf,
		twilio.New(conf.Service.Twilio.Account, conf.Service.Twilio.AccountToken).WithAPIKey(conf.Service.Twilio.Token, conf.Service.Twilio.Secret), // ensure fallback authenticate
	}
}

func (ts *TwilioSender) Send(user UserProvider, template Tpl, params *P) error {
	logger.Get().Infof("Sending %s for %s...", template, user.To())

	msg := getTemplateBody(template, params)
	if len(msg) == 0 {
		// no template yet, so just return instead of an error
		return nil
	}

	resp, exc, err := ts.twilio.SMS(ts.conf.Service.Twilio.Phone, user.To(), msg)
	if err != nil {
		return err
	}

	if exc != nil {
		return exception(template, resp, exc)
	}

	logger.Get().Infof("Sent %s to %s@%s", template, resp.To, resp.Sid)
	return nil
}

func (ts *TwilioSender) SendTo(phone string, template Tpl, params *P) error {
	logger.Get().Infof("Sending %s for %s...", template, phone)

	msg := getTemplateBody(template, params)
	if len(msg) == 0 {
		// no template yet, so just return instead of an error
		return nil
	}

	resp, exc, err := ts.twilio.SMS(ts.conf.Service.Twilio.Phone, phone, msg)
	if err != nil {
		return err
	}

	if exc != nil {
		return exception(template, resp, exc)
	}

	logger.Get().Infof("Sent %s to %s@%s", template, resp.To, resp.Sid)
	return nil
}

func exception(template Tpl, resp *twilio.Response, exc *twilio.Exception) (err error) {
	return errors.Errorf(
		"Failed to send %s to %s with reason %s",
		template,
		resp.To,
		exc.Message,
	)
}
