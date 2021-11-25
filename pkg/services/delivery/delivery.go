package delivery

import (
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/utils/messaging"
	"gitlab.com/learnt/api/pkg/utils/messaging/mail"
	"gitlab.com/learnt/api/pkg/utils/messaging/sms"
	"sync"
)

type UserWithPreferences interface {
	IsReceiveUpdates() bool
	IsReceiveSMSUpdates() bool
}

type UserProvider interface {
	UserWithPreferences
	GetEmail() string
	GetPhoneNumber() string
	GetFirstName() string
}

type Delivery struct {
	smsSender, emailSender messaging.Sender
}

func New(cfg *config.Config) *Delivery {
	return &Delivery{sms.GetSender(cfg), mail.GetSender(cfg)}
}

// Send FIXME: sendMail and sendSMS should have different templates
func (d *Delivery) Send(user UserWithPreferences, template messaging.Tpl, params *messaging.P) (err error) {
	if !user.IsReceiveUpdates() && !user.IsReceiveSMSUpdates() {
		return nil
	}
	conf := config.GetConfig()
	smsSendingEnabled := conf.GetBool("app.enable_sms")

	var errcList []<-chan error

	if user.IsReceiveUpdates() {
		if v, ok := user.(UserProvider); ok {
			errcList = append(errcList, d.sendMail(&mail.User{v.GetEmail(), v.GetFirstName()}, template, params))
		}
	}

	if smsSendingEnabled && user.IsReceiveSMSUpdates() {
		if v, ok := user.(UserProvider); ok {
			errcList = append(errcList, d.sendSMS(&sms.User{v.GetPhoneNumber(), v.GetFirstName()}, template, params))
		}
	}

	return errorPipeline(errcList...)
}

func (d *Delivery) sendMail(user mail.UserProvider, template messaging.Tpl, params *messaging.P) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		err := d.emailSender.Send(user, template, params)
		if err != nil {
			errc <- err
			return
		}
	}()

	return errc
}

func (d *Delivery) sendSMS(user sms.UserProvider, template messaging.Tpl, params *messaging.P) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		err := d.smsSender.Send(user, template, params)
		if err != nil {
			errc <- err
			return
		}
	}()

	return errc
}

func mergeErrors(cs ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	out := make(chan error, len(cs))

	output := func(c <-chan error) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func errorPipeline(errs ...<-chan error) error {
	errc := mergeErrors(errs...)
	for err := range errc {
		if err != nil {
			return err
		}
	}
	return nil
}
