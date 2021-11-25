package mail

import (
	"time"

	"strings"

	m "github.com/keighl/mandrill"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/logger"
)

var _ Sender = &MandrillSender{}

const SEND_AT_LAYOUT = "2006-01-02 15:04:05"

type MandrillSender struct {
	client *m.Client
}

func NewMandrillSender(conf *config.Config) *MandrillSender {
	return &MandrillSender{
		m.ClientWithKey(conf.Service.Mandrill.Key),
	}
}

func (ms *MandrillSender) Send(to UserProvider, template Tpl, params *P) error {
	return ms.SendWithSubject(to, template, "", params)
}

func (ms *MandrillSender) SendTo(email string, template Tpl, params *P) error {
	logger.Get().Infof("Sending %s for %s...", template, email)

	cfg := config.GetConfig()

	msg := &m.Message{
		FromEmail: cfg.GetString("mail.from.email"),
		FromName:  cfg.GetString("mail.from.name"),
		MergeVars: []*m.RcptMergeVars{
			m.MapToRecipientVars(email, getVars(nil, params)),
		},
	}

	msg.AddRecipient(email, "", "to")

	responses, err := ms.client.MessagesSendTemplate(
		msg,
		string(template),
		nil,
	)

	if err != nil {
		return errors.Wrap(err, "Failed to send email")
	}

	return verifyResponses(template, responses)
}

func (ms *MandrillSender) SendWithSubject(to UserProvider, template Tpl, subject string, params *P) error {

	logger.Get().Infof("Sending %s for %s...", template, to.To())

	cfg := config.GetConfig()

	msg := &m.Message{
		FromEmail: cfg.GetString("mail.from.email"),
		FromName:  cfg.GetString("mail.from.name"),
		MergeVars: []*m.RcptMergeVars{
			m.MapToRecipientVars(to.To(), getVars(&to, params)),
		},
	}

	msg.AddRecipient(to.To(), to.GetFirstName(), "to")

	responses, err := ms.client.MessagesSendTemplate(
		msg,
		string(template),
		nil,
	)

	if err != nil {
		return errors.Wrap(err, "Failed to send email")
	}

	return verifyResponses(template, responses)
}

func (ms *MandrillSender) SendAt(at time.Time, to UserProvider, template Tpl, params *P) error {

	cfg := config.GetConfig()

	msg := &m.Message{
		FromEmail: cfg.GetString("mail.from.email"),
		FromName:  cfg.GetString("mail.from.name"),
		SendAt:    at.Format(SEND_AT_LAYOUT),
	}

	msg.MergeVars = []*m.RcptMergeVars{
		m.MapToRecipientVars(to.To(), getVars(&to, params)),
	}

	msg.AddRecipient(to.To(), to.GetFirstName(), "to")

	responses, err := ms.client.MessagesSendTemplate(
		msg,
		string(template),
		nil,
	)

	if err != nil {
		return errors.Wrap(err, "Failed to send email")
	}

	return verifyResponses(template, responses)
}

func verifyResponses(template Tpl, responses []*m.Response) (err error) {

	if len(responses) != 1 {
		return errors.Wrap(err, "Failed to send email, response missing")
	}

	r := responses[0]

	if r.Status == "rejected" {
		return errors.Errorf(
			"Failed to send %s to %s with reason %s",
			template,
			r.Email,
			r.RejectionReason,
		)
	}

	logger.Get().Infof("Sent %s to %s@%s", template, r.Email, r.Id)

	return nil
}

func getVars(to *UserProvider, params *P) map[string]string {
	out := make(map[string]string, 0)
	cfg := config.GetConfig()
	var envSubj string
	if strings.Title(cfg.App.Env) != "www" {
		envSubj = strings.Title(cfg.App.Env) + ": "
	}

	if to != nil {
		user := *to
		out["EMAIL"] = user.To()
		out["FIRST_NAME"] = user.GetFirstName()
	}

	for k, v := range *params {
		out[k] = v
	}

	if v, ok := out["SUBJECT"]; ok {
		out["SUBJECT"] = envSubj + v
	}

	return out
}
