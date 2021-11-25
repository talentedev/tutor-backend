package store

import (
	"testing"
)

func dbSetup(t *testing.T) {
	uri := &URI{
		URL: "mongodb://mongo:27017/nerdly?gssapiServiceName=mongodb",
	}
	dialSession, err := SessionFrom(uri, 15)
	if err != nil {
		t.Skip("Database not available")
	}
	dialSession.Close()
	Init()
}

func addUser(t *testing.T, u *UserMgo) {
	if err := u.SaveNew(); err != nil {
		t.Fatal("Could not add user: ", err)
	}
}

func removeUser(t *testing.T, u *UserMgo) {
	if err := GetCollection("users").RemoveId(u.ID); err != nil {
		t.Fatal("Could not remove user: ", err)
	}
}
