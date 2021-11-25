package intercom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

type Role string
type Response []byte

const (
	RoleUser = Role("user")
	RoleLead = Role("lead")
)

type CustomAttributes struct {
	Profile string `json:"profile,omitempty"`
}

type NewContact struct {
	Role             Role              `json:"role"`
	Email            string            `json:"email"`
	FirstName        string            `json:"first_name,omitempty"`
	LastName         string            `json:"last_name,omitempty"`
	Phone            string            `json:"phone,omitempty"`
	ExternalId       string            `json:"external_id,omitempty"`
	Avatar           string            `json:"avatar,omitempty"`
	SignedUpAt       int64             `json:"signed_up_at,omitempty"`
	LastSeenAt       int64             `json:"last_seen_at,omitempty"`
	CustomAttributes *CustomAttributes `json:"custom_attributes,omitempty"`
}

type Tag struct {
	Type string `json:"type"`
	Id   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type TagAddressableList struct {
	Type       string `json:"type"`
	Data       []Tag  `json:"data"`
	Url        string `json:"url,omitempty"`
	TotalCount int    `json:"total_count,omitempty"`
	HasMore    bool   `json:"has_more,omitempty"`
}

type Contact struct {
	Type             string             `json:"type,omitempty"`
	Id               string             `json:"id"`
	WorkspaceId      string             `json:"workspace_id,omitempty"`
	UserId           string             `json:"user_id"`
	Role             Role               `json:"role"`
	Email            string             `json:"email"`
	Phone            string             `json:"phone"`
	Name             string             `json:"name"`
	Avatar           string             `json:"avatar"`
	OwnerId          string             `json:"owner_id"`
	CreatedAt        int64              `json:"created_at,omitempty"`
	UpdatedAt        int64              `json:"updated_at,omitempty"`
	SignedUpAt       int64              `json:"signed_up_at,omitempty"`
	LastSeenAt       int64              `json:"last_seen_at,omitempty"`
	Tags             TagAddressableList `json:"tags"`
	CustomAttributes *CustomAttributes  `json:"custom_attributes,omitempty"`
	ExternalId       string             `json:"external_id,omitempty"`
}

type SearchContactResponse struct {
	Type       string    `json:"type"`
	Data       []Contact `json:"data"`
	TotalCount int       `json:"total_count"`
}

func sendRequest(method, url string, data interface{}) []byte {
	raw, err := json.Marshal(data)
	if err != nil {
		logger.Get().Errorf("Error marshaling data: %v", err)
		return nil
	}
	request, err := http.NewRequest(method, url, bytes.NewBuffer(raw))
	if err != nil {
		logger.Get().Errorf("Error reading request: %v", err)
		return nil
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	token := config.GetConfig().GetString("service.intercom.token")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	client := http.Client{Timeout: time.Second * 20}
	response, err := client.Do(request)
	if err != nil {
		logger.Get().Errorf("Error reading response: %v", err)
		return nil
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logger.Get().Errorf("Error reading response body: %v", err)
		return nil
	}
	return body
}

func (r *Response) BindWith(object interface{}) {
	if err := json.Unmarshal(*r, object); err != nil {
		logger.Get().Errorf("Error unmarshalling response body: %v", err)
	}
}

type SearchQuery struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

func SearchContact(email string) (searchResponse *SearchContactResponse) {
	data := struct {
		Query SearchQuery `json:"query"`
	}{
		Query: SearchQuery{
			Field:    "email",
			Operator: "=",
			Value:    email,
		},
	}

	response := sendRequest(http.MethodPost, "https://api.intercom.io/contacts/search", data)
	if response == nil {
		logger.Get().Errorf("Failed searching contact")
		return
	}
	if err := json.Unmarshal(response, &searchResponse); err != nil {
		logger.Get().Errorf("Error unmarshalling response %s: %v", string(response), err)
		return
	}
	return
}

func CreateContact(data *Contact, roleGroup string) (contact *Contact) {
	now := time.Now().Unix()
	params := &Contact{
		Role:             data.Role,
		Email:            data.Email,
		Phone:            data.Phone,
		Name:             data.Name,
		SignedUpAt:       now,
		LastSeenAt:       now,
		CustomAttributes: nil,
		ExternalId:       data.ExternalId,
		Avatar:           data.Avatar,
	}
	if data.ExternalId != "" {
		if profileUrl, err := core.AppURL("/admin/%s/%s", roleGroup, data.ExternalId); err != nil {
			logger.PrintStack(logger.ERROR, "error creating app url: %v", err)
		} else {
			params.CustomAttributes = &CustomAttributes{Profile: profileUrl}
		}
	}

	response := sendRequest(http.MethodPost, "https://api.intercom.io/contacts", params)
	if response == nil {
		logger.Get().Errorf("Failed creating contact")
		return
	}
	if err := json.Unmarshal(response, &contact); err != nil {
		logger.Get().Errorf("Error unmarshalling response %s: %v", string(response), err)
		return
	}
	return
}

func UpdateLeadToUser(contact *Contact, user *store.UserMgo) (updatedContact *Contact) {
	params := &Contact{
		Role:             RoleUser,
		Email:            user.GetEmail(),
		Avatar:           user.Avatar(),
		SignedUpAt:       time.Now().Unix(),
		LastSeenAt:       time.Now().Unix(),
		ExternalId:       user.ID.Hex(),
		CustomAttributes: &CustomAttributes{},
	}
	if profileUrl, err := core.AppURL("/admin/tutors/%s", user.ID.Hex()); err != nil {
		logger.PrintStack(logger.ERROR, "error creating app url: %v", err)
	} else {
		params.CustomAttributes.Profile = profileUrl
	}
	response := sendRequest(http.MethodPut, fmt.Sprintf("https://api.intercom.io/contacts/%s", contact.Id), params)
	if response == nil {
		logger.Get().Errorf("Failed to update contact")
		return
	}
	if err := json.Unmarshal(response, &updatedContact); err != nil {
		logger.Get().Errorf("Error unmarshalling response %s: %v", string(response), err)
	}
	return
}

func GetTags() (tags *TagAddressableList) {
	response := sendRequest(http.MethodGet, "https://api.intercom.io/tags", nil)
	if response == nil {
		logger.Get().Errorf("Failed getting tags")
		return
	}
	if err := json.Unmarshal(response, &tags); err != nil {
		logger.Get().Errorf("Error unmarshalling response %s: %v", string(response), err)
	}
	return
}

func GetTag(name string) *Tag {
	tags := GetTags()
	if tags == nil {
		return nil
	}
	for _, tag := range tags.Data {
		if tag.Name == name {
			return &tag
		}
	}
	return nil
}

func TagContact(tagId, contactId string) (tag *Tag) {
	data := struct {
		Id string `json:"id"`
	}{Id: tagId}
	response := sendRequest(http.MethodPost, fmt.Sprintf("https://api.intercom.io/contacts/%s/tags", contactId), data)
	if response == nil {
		logger.Get().Errorf("Failed getting tags")
		return
	}
	if err := json.Unmarshal(response, &tag); err != nil {
		logger.Get().Errorf("Error unmarshalling response %s: %v", string(response), err)
	}
	return
}

func UserCompletedApplication(user *store.UserMgo) *Contact {
	searchRes := SearchContact(user.GetEmail())
	var contact *Contact
	if searchRes.TotalCount > 0 {
		contact = &searchRes.Data[0]
	}

	if contact != nil {
		contact = UpdateLeadToUser(contact, user)
	} else {
		contact = CreateContact(&Contact{
			Role:       RoleUser,
			Email:      user.GetEmail(),
			Name:       user.GetName(),
			Phone:      user.Profile.Telephone,
			ExternalId: user.ID.Hex(),
			Avatar:     user.Avatar(),
			LastSeenAt: time.Now().Unix(),
			SignedUpAt: time.Now().Unix(),
		}, "tutors")
	}

	if err := services.NewUsers().UpdateId(user.ID, bson.M{
		"intercom_id": contact.Id,
	}); err != nil {
		logger.Get().Errorf("Error saving intercom contact id for user %s with id %s: %v", user.ID.Hex(), contact.Id, err)
	}

	tag := GetTag("tutorapplicationcompleted")
	if tag != nil {
		tag = TagContact(tag.Id, contact.Id)
		if tag == nil {
			logger.Get().Errorf("Unable to tag user %s with #tutorapplicationcompleted user", user.ID.Hex())
		}
	} else {
		logger.Get().Errorf("Unable to get id for tag tutorapplicationcompleted")
	}
	return contact
}
