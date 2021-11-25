package checkr

import (
	"time"
)

// ResourceObject is the string representation of the resources
type ResourceObject string

func (o ResourceObject) String() string {
	return string(o)
}

const (
	ObjectWebhook                ResourceObject = "event"
	ObjectPagination             ResourceObject = "list"
	ObjectCandidate              ResourceObject = "candidate"
	ObjectSchool                 ResourceObject = "school"
	ObjectEmployer               ResourceObject = "employer"
	ObjectInvitation             ResourceObject = "invitation"
	ObjectReport                 ResourceObject = "report"
	ObjectPackage                ResourceObject = "package"
	ObjectDocument               ResourceObject = "document"
	ObjectVerification           ResourceObject = "validation"
	ObjectAdverseItem            ResourceObject = "adverse_item"
	ObjectAdverseAction          ResourceObject = "adverse_action"
	ObjectSubscription           ResourceObject = "subscription"
	ObjectGeo                    ResourceObject = "geo"
	ObjectProgram                ResourceObject = "program"
	ObjectSSNTrace               ResourceObject = "ssn_trace"
	ObjectSexOffenderSearch      ResourceObject = "sex_offender_search"
	ObjectGlobalWatchlistSearch  ResourceObject = "global_watchlist_search"
	ObjectNationalCriminalSearch ResourceObject = "national_criminal_search"
	ObjectCountyCriminalSearch   ResourceObject = "county_criminal_search"
	ObjectStateCriminalSearch    ResourceObject = "state_criminal_search"
	ObjectMotorVehicleReport     ResourceObject = "motor_vehicle_report"
	ObjectEducationVerification  ResourceObject = "education_verification"
	ObjectEmploymentVerification ResourceObject = "employment_verification"
)

type Adjudication string

func (a Adjudication) String() string {
	return string(a)
}

const (
	AdjudicationEngaged    Adjudication = "engaged"
	AdjudicationPreAction  Adjudication = "pre_adverse_action"
	AdjudicationPostAction Adjudication = "post_adverse_action"
)

type Package string

func (p Package) String() string {
	return string(p)
}

const (
	PackageTaskerStd Package = "tasker_standard"
	PackageTaskerPro Package = "tasker_pro"
	PackageDriverStd Package = "driver_standard"
	PackageDriverPro Package = "driver_pro"
)

// ResponseError represents the basic error that checkr returns in case of an issue. The error is set as
// an interface because it returns either a string, or a slice of strings, without a hint of the type.
type ResponseError struct {
	Response interface{} `json:"error"`
}

func (e ResponseError) Error() string {
	switch e.Response.(type) {
	case string:
		return e.Response.(string)
	case []interface{}:
		response := ""
		for _, v := range e.Response.([]interface{}) {
			switch v.(type) {
			case string:
				response += v.(string) + ", "
			}
		}
		return response
	default:
		return "unknown error"
	}
}

// Address represents the base address object.
type Address struct {
	Street  string `json:"street"`
	Unit    string `json:"unit"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zipcode"`
	Country string `json:"country,omitempty"` // default: 'US'
}

// Webhook is a checkr webhook
type Webhook struct {
	ID         string      `json:"id"`
	Object     string      `json:"object"`
	Type       string      `json:"type"`
	CreatedAt  time.Time   `json:"created_at"`
	WebhookURL string      `json:"webhook_url"`
	Data       WebhookData `json:"data"`
}

type WebhookData struct {
	Object map[string]interface{} `json:"object"`
}
