package notifications

import (
	"github.com/olebedev/emitter"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"
	"gopkg.in/mgo.v2/bson"
)


func InitNotifications() {
	utils.Bus().On("TUTOR_LOGGED_IN", func(event *emitter.Event) {
		data := event.Args[0].(map[string]string)
		id := data["id"]
		if id == "" {
			return
		}

		var tutor *store.UserMgo
		if err := store.GetCollection("users").Find(bson.M{"_id": bson.ObjectIdHex(id)}).One(&tutor); err != nil {
			return
		}

		NotifyFollowers(tutor)
	})
}
