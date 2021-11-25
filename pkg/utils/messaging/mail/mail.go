package mail

import (
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/utils/messaging"
)

type (
	P            = messaging.P
	Tpl          = messaging.Tpl
	Sender       = messaging.Sender
	UserProvider = messaging.UserProvider
)

type User struct {
	Email     string
	FirstName string
}

func (u *User) To() string {
	return u.Email
}

func (u *User) GetFirstName() string {
	return u.FirstName
}

func GetSender(conf *config.Config) Sender {
	return NewMandrillSender(conf)
}

func ParseTemplate(template Tpl, params *P) (out string, err error) {
	return
}
