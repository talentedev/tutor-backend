package twilio

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
)

type Response struct {
	Sid         string  `json:"sid"`
	DateCreated string  `json:"date_created"`
	DateUpdate  string  `json:"date_updated"`
	DateSent    string  `json:"date_sent"`
	AccountSid  string  `json:"account_sid"`
	To          string  `json:"to"`
	From        string  `json:"from"`
	NumMedia    string  `json:"num_media"`
	Body        string  `json:"body"`
	Status      string  `json:"status"`
	Direction   string  `json:"direction"`
	ApiVersion  string  `json:"api_version"`
	Price       *string `json:"price,omitempty"`
	Url         string  `json:"uri"`
}

// Optional SMS parameters
var (
	// https://www.twilio.com/docs/sms/api/message-resource
	// Settings for what Twilio should do with addresses in message logs
	SmsAddressRetentionObfuscate = &Option{"AddressRetention", "obfuscate"}
	SmsAddressRetentionRetain    = &Option{"AddressRetention", "retain"}
	// Settings for what Twilio should do with message content in message logs
	SmsContentRetentionDiscard = &Option{"ContentRetention", "discard"}
	SmsContentRetentionRetain  = &Option{"ContentRetention", "retain"}
)

func (twilio *Twilio) SMS(from, to, body string, opts ...*Option) (smsResponse *Response, exception *Exception, err error) {
	return twilio.SendSMS(from, to, body, "", "", opts...)
}

func (twilio *Twilio) SendSMSWithSID(from, to, body, applicationSid string, opts ...*Option) (smsResponse *Response, exception *Exception, err error) {
	return twilio.SendSMS(from, to, body, "", applicationSid, opts...)
}

func (twilio *Twilio) SendSMSWithCallback(from, to, body, statusCallback string, opts ...*Option) (smsResponse *Response, exception *Exception, err error) {
	return twilio.SendSMS(from, to, body, statusCallback, "", opts...)
}

// See http://www.twilio.com/docs/api/rest/sending-sms for more information.
func (twilio *Twilio) SendSMS(from, to, body, statusCallback, applicationSid string, opts ...*Option) (smsResponse *Response, exception *Exception, err error) {
	formValues := initValues(to, body, nil, statusCallback, applicationSid)
	formValues.Set("From", from)

	for _, opt := range opts {
		if opt != nil {
			formValues.Set(opt.Key, opt.Value)
		}
	}

	smsResponse, exception, err = twilio.sendMessage(formValues)
	return
}

func (twilio *Twilio) MMS(from, to, body string, mediaUrl []string) (smsResponse *Response, exception *Exception, err error) {
	return twilio.SendMMS(from, to, body, mediaUrl, "", "")
}

func (twilio *Twilio) SendMMS(from, to, body string, mediaUrl []string, statusCallback, applicationSid string) (smsResponse *Response, exception *Exception, err error) {
	formValues := initValues(to, body, mediaUrl, statusCallback, applicationSid)
	formValues.Set("From", from)

	smsResponse, exception, err = twilio.sendMessage(formValues)
	return
}

func (twilio *Twilio) sendMessage(formValues url.Values) (smsResponse *Response, exception *Exception, err error) {
	twilioUrl := twilio.BaseUrl + "/Accounts/" + twilio.AccountSid + "/Messages.json"

	res, err := twilio.post(formValues, twilioUrl)
	if err != nil {
		return smsResponse, exception, err
	}
	defer res.Body.Close()

	responseBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return smsResponse, exception, err
	}

	if res.StatusCode != http.StatusCreated {
		exception = new(Exception)
		err = json.Unmarshal(responseBody, exception)

		return smsResponse, exception, err
	}

	smsResponse = new(Response)
	err = json.Unmarshal(responseBody, smsResponse)
	return smsResponse, exception, err
}

func initValues(to, body string, mediaUrl []string, statusCallback, applicationSid string) url.Values {
	formValues := url.Values{}

	formValues.Set("To", to)
	formValues.Set("Body", body)

	if len(mediaUrl) > 0 {
		for _, value := range mediaUrl {
			formValues.Add("MediaUrl", value)
		}
	}

	if statusCallback != "" {
		formValues.Set("StatusCallback", statusCallback)
	}

	if applicationSid != "" {
		formValues.Set("ApplicationSid", applicationSid)
	}

	return formValues
}
