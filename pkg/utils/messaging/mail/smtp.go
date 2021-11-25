package mail

import (
	"net/smtp"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
)

type SmtpSender struct{}

func NewSmtpSender(key string) *SmtpSender {
	return &SmtpSender{}
}

func Send(to UserProvider, template Tpl, params *P) error {
	return nil
}

func SendWithSubject(to UserProvider, template Tpl, subject string, params *P) error {

	html, err := ParseTemplate(template, params)

	if err != nil {
		return errors.Wrap(err, "Failed to render email template")
	}

	return smtpSend(to.To(), html)
}

func SendAt(when time.Time, to UserProvider, template Tpl, params *P) error {
	return errors.New("Not implemented")
}

func smtpSend(to, msg string) error {

	cfg := config.GetConfig()

	from := cfg.GetString("mail.from.email")
	user := cfg.GetString("mail.smtp.user")
	pass := cfg.GetString("mail.smtp.pass")
	host := cfg.GetString("mail.smtp.host")

	auth := smtp.PlainAuth("Smtp", user, pass, host)

	return smtp.SendMail(
		host,
		auth,
		from,
		[]string{to},
		[]byte(msg),
	)
}
