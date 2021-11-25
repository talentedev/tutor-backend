package services

import (
	"os"
	"testing"
	"time"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

func dbSetup(t *testing.T) {
	config.GetConfig().Set("storage.database", "testing")
	dialSession, err := store.NewSession()
	if err != nil {
		t.Skip("Database not available")
	}
	defer dialSession.Close()
	store.Init()
}

func happyTutor() *store.UserMgo {
	return &store.UserMgo{
		ID:             bson.NewObjectId(),
		Username:       time.Now().Format(time.RFC3339Nano),
		ApprovalStatus: store.ApprovalStatusApproved,
		Role:           store.RoleTutor,
		Profile:        store.Profile{Avatar: &store.Upload{ID: bson.NewObjectId()}},
		Tutoring: &store.Tutoring{
			Meet: store.MeetInPerson,
			Rate: 10,
			Subjects: []store.TutoringSubject{store.TutoringSubject{
				ID:      bson.NewObjectId(),
				Subject: bson.NewObjectId(),
			}},
			Degrees: []store.TutoringDegree{store.TutoringDegree{
				ID:         bson.NewObjectId(),
				University: bson.NewObjectId(),
			}},
			Availability: &store.Availability{
				Recurrent: []*store.AvailabilitySlot{
					{
						ID:   bson.NewObjectId(),
						From: time.Now().Add(23 * time.Hour),
						To:   time.Now().Add(26 * time.Hour),
					},
				},
			},
		},
		Payments: &store.Payments{ConnectID: "id"},
	}
}

func cleanupUser(t *testing.T, u *store.UserMgo) {
	if err := store.GetCollection("users").RemoveId(u.ID); err != nil {
		t.Error("could not remove user:", err)
	}
}

func TestMain(m *testing.M) {
	c := m.Run()
	os.Exit(c)
}

func TestMeetLocation(t *testing.T) {
	// This should check that it can find andresses within 50 miles
	testAddr := "87 lafayette street, new york, ny 10013"
	tests := []struct {
		name        string
		address     string
		online      bool
		matchLocal  bool
		matchOnline bool
	}{
		{name: "local in range", matchLocal: true, matchOnline: false, address: "278 Spring St, New York, NY 10013"},
		{name: "local in range GPS", matchLocal: true, matchOnline: false, address: "40.723987,-74.002640"},
		{name: "local out of range (california)", matchLocal: false, matchOnline: false, address: "92780"},
		{name: "local in range and online", matchLocal: true, matchOnline: true, address: "278 Spring St, New York, NY 10013", online: true},
		{name: "online", matchLocal: false, matchOnline: true, online: true},
		{name: "no locations info", matchLocal: true, matchOnline: true},
	}

	config.GetConfig().Set("storage.search_miles", 50)
	config.GetConfig().Set("storage.database", "testing")
	dbSetup(t)

	localTutor := happyTutor()
	localTutor.Username = "local:" + localTutor.Username
	c, err := AddressToCoordinates(testAddr)
	if err != nil {
		t.Fatal(err)
	}
	localTutor.Location = &store.UserLocation{
		Position: &store.GeoJSON{
			Type: "Point", Coordinates: c,
		},
	}
	if err := localTutor.SaveNew(); err != nil {
		t.Fatal("could not create needed for test:", err)
	}
	defer cleanupUser(t, localTutor)

	onlineTutor := happyTutor()
	onlineTutor.Username = "online:" + localTutor.Username
	onlineTutor.Tutoring.Meet = store.MeetOnline
	if err := onlineTutor.SaveNew(); err != nil {
		t.Fatal("could not create needed for test:", err)
	}
	defer cleanupUser(t, onlineTutor)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := GetSearch()
			s.Clear()

			if test.address != "" {
				c, err := AddressToCoordinates(test.address)
				if err != nil {
					t.Fatal(err)
				}
				s.MeetLocation(c)
			}

			matchedUsers, err := s.Do(test.address != "", test.online)
			if err != nil {
				t.Fatal(err)
			}

			matchLocal := false
			matchOnline := false
			for _, u := range matchedUsers {
				if u.ID == localTutor.ID {
					matchLocal = true
				}
				if u.ID == onlineTutor.ID {
					matchOnline = true
				}
			}

			if matchOnline != test.matchOnline {
				t.Error("matchOnline did not match:", matchOnline)
			}
			if matchLocal != test.matchLocal {
				t.Error("matchLocal did not match:", matchLocal)
			}
		})
	}
}

func TestSort(t *testing.T) {
	config.GetConfig().Set("storage.database", "testing")
	dbSetup(t)

	onlineTutor := happyTutor()
	onlineTutor.Username = "online:" + onlineTutor.Username
	onlineTutor.Online = 1
	if err := onlineTutor.SaveNew(); err != nil {
		t.Fatal("could not create needed for test:", err)
	}
	defer cleanupUser(t, onlineTutor)

	offlineTutor := happyTutor()
	offlineTutor.Username = "offline:" + offlineTutor.Username
	if err := offlineTutor.SaveNew(); err != nil {
		t.Fatal("could not create needed for test:", err)
	}
	defer cleanupUser(t, offlineTutor)

	s := GetSearch()
	s.Clear()
	matched, err := s.Do(false, false)
	if err != nil {
		t.Fatal(err)
	}

	previousOnlineStatus := uint8(1)
	for _, v := range matched {
		if previousOnlineStatus < uint8(v.Online) {
			t.Fatal("non was after online")
		}
	}
}
