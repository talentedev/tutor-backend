package dto

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type Profile struct {
	Model  `dto:":Profile"`
	Avatar string `dto:"avatar"`
	Phone  string
}

type NonDto struct {
	NonDtoProp string
}

type User struct {
	Model     `dto:":User"`
	ID        string  `dto:"id"`
	FirstName string  `dto:"first_name"`
	LastName  string  `dto:"last_name"`
	Profile   Profile `dto:"profile"`
	NonDto    *NonDto
	Password  string
	CC        string `dto:"cc=CCS($admin)"`

	_ string `dto:"name=Name()"`
	_ string `dto:"extra=Extra()" dtoif:"$extended"`
}

func (u User) CCS(a bool) string {
	if a {
		return u.CC
	}
	parts := strings.Split(u.CC, "-")
	return parts[len(parts)-1]
}

func (u User) Name() string {
	return u.FirstName + " " + u.LastName
}

func (u User) Extra() string {
	return "extra"
}

func assert(t *testing.T, dto interface{}, expected string) {

	var o interface{}

	err := json.Unmarshal([]byte(expected), &o)

	if err != nil {
		t.Error(err)
		return
	}

	if !reflect.DeepEqual(dto, o) {
		str, err := json.Marshal(dto)
		if err != nil {
			t.Error(err)
			return
		}
		t.Errorf(
			"\n Actual: %s\n\n Expected: %s\n",
			string(str),
			expected,
		)
	}
}

func TestTags(t *testing.T) {

	a := User{
		ID:        "1",
		FirstName: "Jhon",
		LastName:  "Doe",
		Profile: Profile{
			Avatar: "http://avatar",
			Phone:  "555",
		},
		NonDto:   &NonDto{"test"},
		Password: "Password",
		CC:       "5555-5555-5555-1111",
	}

	admin := func() bool {
		return true
	}

	dto := Serialize(
		a,
		Prop("admin", admin),
		Prop("extended", false),
	)

	expected := `
	{
		"__dto": "User",
		"id": "1",
		"first_name":"Jhon",
		"last_name":"Doe",
		"profile": {
			"__dto": "Profile",
			"avatar": "http://avatar"
		},
		"cc": "5555-5555-5555-1111",
		"name": "Jhon Doe"
	}
	`

	assert(t, dto, expected)
}
