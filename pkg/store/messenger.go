package store

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/logger"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// The message typess
const (
	TypeText         messageType = "text"
	TypeFile         messageType = "file"
	TypeNotification messageType = "notification"
)

// MessageDataType is an emun for message types
type MessageDataType byte

// List of data types
const (
	DataTypeReschedule MessageDataType = iota + 1
)

func GetMessengerCollection(name string) *mgo.Collection {
	return GetCollection(fmt.Sprintf("msg_%s", name))
}

// Valid checks if the data type is withint the range of types
func (mdt MessageDataType) Valid() bool {
	return mdt == DataTypeReschedule
}

// MessageData holds the message data
type MessageData struct {
	Type    MessageDataType `json:"type" bson:"type"`
	Title   string          `json:"title" bson:"title"`
	Content string          `json:"content" bson:"content"`
	Data    interface{}     `json:"data" bson:"data"`
}

type (
	messageType string

	// Messages is used to add functions on for interacting with the database
	Messages struct{}

	// Message holds a message
	Message struct {
		ID       bson.ObjectId   `json:"_id" bson:"_id"`
		Sender   bson.ObjectId   `json:"sender" bson:"sender"`
		Thread   bson.ObjectId   `json:"thread" bson:"thread"`
		Type     messageType     `json:"type" bson:"type"`
		Time     time.Time       `json:"time" bson:"time"`
		Seen     []bson.ObjectId `json:"seen" bson:"seen"`
		Users    []bson.ObjectId `json:"users" bson:"users"`
		Approved bool            `json:"approved" bson:"approved"`
		Body     interface{}     `json:"body" bson:"body"`
		Data     *MessageData    `json:"data" bson:"data"`
	}

	ThreadParticipantProfile struct {
		FirstName string  `json:"first_name" bson:"first_name"`
		LastName  string  `json:"last_name" bson:"last_name"`
		Avatar    *Upload `json:"avatar" bson:"avatar"`
	}

	ThreadParticipant struct {
		ID      bson.ObjectId            `json:"_id" bson:"_id"`
		Profile ThreadParticipantProfile `json:"profile" bson:"profile"`
	}

	// MessageDto is a Message with IDs looked up and filled out
	MessageDto struct {
		ID     bson.ObjectId     `json:"_id" bson:"_id"`
		Sender ThreadParticipant `json:"sender,omitempty"`
		Thread bson.ObjectId     `json:"thread" bson:"thread"`
		Type   messageType       `json:"type" bson:"type"`
		Time   time.Time         `json:"time" bson:"time"`
		Seen   []bson.ObjectId   `json:"seen" bson:"seen"`
		Users  []bson.ObjectId   `json:"users" bson:"users"`
		Body   interface{}       `json:"body" bson:"body"`
		Data   *MessageData      `json:"data" bson:"data"`
	}
)

// GetMessages gets the type the holds all message functions
func GetMessages() *Messages {
	return &Messages{}
}

// GetDto fills in the sender to go from a Message to MessageDTO
func (m *Message) GetDto(sender ThreadParticipant) *MessageDto {
	return &MessageDto{
		ID:     m.ID,
		Thread: m.Thread,
		Sender: sender,
		Type:   m.Type,
		Time:   m.Time,
		Seen:   m.Seen,
		Users:  m.Users,
		Body:   m.Body,
		Data:   m.Data,
	}
}

// Valid checks if a message type is valid
func (mt messageType) Valid() bool {
	return mt == TypeText || mt == TypeFile || mt == TypeNotification
}

// MESSAGES

// NewMessage creates a message struct
func (m *Messages) NewMessage(sender *UserMgo, threadID bson.ObjectId, kind messageType, body interface{}, data *MessageData) *Message {
	return &Message{
		ID:     bson.NewObjectId(),
		Sender: sender.ID,
		Thread: threadID,
		Type:   kind,
		Body:   body,
		Data:   data,
		Time:   time.Now(),
		Seen:   make([]bson.ObjectId, 0),
		Users:  make([]bson.ObjectId, 0),
	}
}

// MarkAsRead marks a message red for a user
func (m *Messages) MarkAsRead(messages []string, user *UserMgo) (err error) {
	ids := make([]bson.ObjectId, 0)

	for _, id := range messages {
		ids = append(ids, bson.ObjectIdHex(id))
	}

	if _, err = GetMessengerCollection("messages").UpdateAll(
		bson.M{"_id": bson.M{"$in": ids}},
		bson.M{"$addToSet": bson.M{
			"seen": user.ID,
		}},
	); err != nil {
		return errors.Wrap(err, "could not make messages as read")
	}
	return nil
}

// GetUserCount gets a count of all messages of a user
func (m *Messages) Count(u *UserMgo) (int, error) {

	query := bson.M{
		"sender": bson.M{"$ne": u.ID},
		"seen":   bson.M{"$ne": u.ID},
		"users":  u.ID,
	}

	if config.GetConfig().GetBool("messenger.require_approval") {
		query["approved"] = true
	}

	n, err := GetCollection("messages").Find(query).Count()
	if err != nil {
		return 0, errors.Wrap(err, "could not get user count")
	}

	return n, nil
}

func getMessageAggregate(items ...bson.M) []bson.M {
	std := []bson.M{
		lookup("users", "sender", "_id", "sender"),
		unwind("$sender"),
		{"$project": bson.M{
			"type":   1,
			"thread": 1,
			"body":   1,
			"data":   1,
			"time":   1,
			"seen":   1,
			"users":  1,
			"sender": "$sender",
		}},

		{"$sort": bson.M{"time": -1}},
	}

	for _, item := range items {
		std = append(std, item)
	}

	return std

}

// WithID finds a message by id
func (m *Messages) WithID(id bson.ObjectId) (*MessageDto, error) {
	var message *MessageDto
	topipe := getMessageAggregate(bson.M{"$match": bson.M{"_id": id}})
	if err := GetMessengerCollection("messages").Pipe(topipe).One(&message); err != nil {
		return nil, errors.Wrap(err, "could not find message by id")

	}
	return message, nil
}

// CountForThread Count all messages from a thread
func (m *Messages) CountForThread(thread *ThreadDto) (total int64) {
	count, _ := GetMessengerCollection("messages").Find(bson.M{"thread": thread.ID}).Count()
	return int64(count)
}

// ForThread Get all messages from a thread
func (m *Messages) ForThread(thread *ThreadDto, skip, limit int64) (messages []MessageDto) {
	defer func() {
		if r := recover(); r != nil {
			logger.Get().Errorf("recovered panic in ForThread: %v", r)
			debug.PrintStack()
		}
	}()

	var topipe = getMessageAggregate(
		bson.M{"$match": bson.M{"thread": thread.ID}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	if skip == 0 && limit == 0 {
		topipe = getMessageAggregate()
	}

	if err := GetMessengerCollection("messages").Pipe(topipe).All(&messages); err != nil {
		panic(err)
	}

	return
}

// MESSAGE

// Flag flags a message to be reviewed
func (m *MessageDto) Flag(user UserMgo, reason string) error {
	if err := GetMessengerCollection("flags").Insert(bson.M{
		"_id":    bson.NewObjectId(),
		"user":   user.ID,
		"reason": reason,
		"time":   time.Now(),
	}); err != nil {
		return errors.Wrap(err, "could not save flagging of message")
	}
	return nil

}

type (
	// ThreadMeta is the meta part of a thread
	ThreadMeta map[string]interface{}

	// Threads allows functions for threads
	Threads struct{}

	// Thread is a thread as stored in the db
	Thread struct {
		ID           bson.ObjectId   `json:"_id" bson:"_id"`
		Name         string          `json:"name" bson:"name"`
		Creator      bson.ObjectId   `json:"creator" bson:"creator"`
		LastMessage  *bson.ObjectId  `json:"message,omitempty" bson:"message,omitempty"`
		Participants []bson.ObjectId `json:"participants" bson:"participants"`
		Deleted      []string        `json:"deleted,omitempty" bson:"deleted,omitempty"`
		Archived     []string        `json:"archived,omitempty" bson:"archived,omitempty"`
		// Last message time for order purposes
		MessageTime time.Time   `json:"message_time,omitempty" bson:"message_time,omitempty"`
		Meta        *ThreadMeta `json:"meta,omitempty" bson:"meta,omitempty"`
		Time        time.Time   `json:"time" bson:"time"`
	}

	// ThreadDto is a Thread converted to fill in IDs for types
	ThreadDto struct {
		ID           bson.ObjectId       `json:"_id" bson:"_id"`
		Name         string              `json:"name" bson:"name"`
		Creator      ThreadParticipant   `json:"creator" bson:"creator"`
		LastMessage  *MessageDto         `json:"message,omitempty" bson:"message,omitempty"`
		Participants []ThreadParticipant `json:"participants" bson:"participants"`
		Deleted      []string            `json:"deleted,omitempty" bson:"deleted,omitempty"`
		Archived     []string            `json:"archived,omitempty" bson:"archived,omitempty"`
		// Last message time for order purposes
		MessageTime time.Time   `json:"message_time,omitempty" bson:"message_time,omitempty"`
		Meta        *ThreadMeta `json:"meta,omitempty" bson:"meta,omitempty"`
		Time        time.Time   `json:"time" bson:"time"`
	}

	// Review holds the information about reviewed flagged threads
	Review struct {
		UserID string    `json:"userId" bson:"userId"`
		Time   time.Time `json:"time" bson:"time"`
	}
	// ThreadFlag contains the info from when a user flags a thread
	ThreadFlag struct {
		ID       bson.ObjectId `json:"_id" bson:"_id"`
		Time     time.Time     `json:"time" bson:"time"`
		ThreadID string        `json:"threadId" bson:"threadId"`
		UserID   string        `json:"userId" bson:"userId"`
		Reason   string        `json:"reason" bson:"reason"`
		Reviews  []Review      `json:"reviews" bson:"reviews"`
	}
)

// GetThreads returns a threads with the associated functions
func GetThreads() *Threads {
	return &Threads{}
}

func getThreadsAggregate(items ...bson.M) (all []bson.M) {

	all = []bson.M{

		lookup("users", "creator", "_id", "creator"),
		unwind("$creator"),

		lookup("messages", "message", "_id", "messageObject"),
		unwind("$messageObject"),

		lookup("users", "messageObject.sender", "_id", "messageSender"),
		unwind("$messageSender"),

		lookup("users", "participants", "_id", "participants"),

		{
			"$project": bson.M{
				"_id":  1,
				"name": 1,

				"message._id":    "$messageObject._id",
				"message.sender": "$messageSender",
				"message.thread": "$messageObject.thread",
				"message.type":   "$messageObject.type",
				"message.time":   "$messageObject.time",
				"message.seen":   "$messageObject.seen",
				"message.users":  "$messageObject.users",
				"message.body":   "$messageObject.body",

				"creator": "$creator",
				"participants": bson.M{
					"$map": bson.M{
						"input": "$participants",
						"as":    "participant",
						"in": bson.M{
							"_id":     "$$participant._id",
							"profile": "$$participant.profile",
						},
					},
				},
				"deleted":      1,
				"archived":     1,
				"time":         1,
				"meta":         1,
				"message_time": 1,
			},
		},
	}

	for _, item := range all {
		items = append(items, item)
	}

	return items

}

// THREADS

// Fetch gets all the threads of a user
func (t *Threads) Fetch(user UserMgo, archived bool) ([]ThreadDto, error) {

	getArchivedQuery := func() bson.M {

		if archived {
			return bson.M{"archived": user.ID}
		}

		return bson.M{
			"$or": []bson.M{
				{"archived": bson.M{"$ne": user.ID}},
				{"archived": bson.M{"$exists": false}},
			},
		}
	}

	query := bson.M{
		"participants": user.ID,
		"$and": []bson.M{
			{
				"$or": []bson.M{
					{"deleted": bson.M{"$ne": user.ID}},
					{"deleted": bson.M{"$exists": false}},
				},
			},
			getArchivedQuery(),
		},
	}

	match := bson.M{
		"$match": query,
	}

	sort := bson.M{
		"$sort": bson.M{
			"message_time": -1,
		},
	}

	var threads []ThreadDto
	topipe := getThreadsAggregate(match, sort)
	if err := GetMessengerCollection("threads").Pipe(topipe).All(&threads); err != nil {
		return nil, errors.Wrap(err, "could not fetch threads")
	}

	return threads, nil
}

// Create makes a new thread
func (t *Threads) Create(createThread *Thread) (*ThreadDto, error) {
	createThread.ID = bson.NewObjectId()
	createThread.Time = time.Now()

	if createThread.Name == "" {
		if err := createThread.SetDefaultName(); err != nil {
			return nil, err
		}
	}

	if len(createThread.Participants) < 2 {
		return nil, errors.New("two participants required")
	}

	if err := GetMessengerCollection("threads").Insert(createThread); err != nil {
		return nil, errors.Wrap(err, "could not insert thread into db")
	}

	thread, err := t.WithID(createThread.ID)
	if err != nil {
		return nil, err
	}

	return thread, nil
}

// ParticipantsThread finds threads that includes all participants
func (t *Threads) ParticipantsThread(participants []ThreadParticipant) (thread *ThreadDto, err error) {
	var ids = make([]bson.ObjectId, len(participants))

	for i, p := range participants {
		ids[i] = p.ID
	}

	q := GetMessengerCollection("threads").Find(bson.M{"participants": bson.M{"$all": ids}})
	if err = q.One(&thread); err != nil {
		return nil, errors.Wrap(err, "could not get participants of thread")
	}
	thread.Participants = participants

	return thread, nil
}

// WithID looks up a thread by the ID
func (t *Threads) WithID(id bson.ObjectId) (*ThreadDto, error) {
	var thread *ThreadDto
	topipe := getThreadsAggregate(bson.M{"$match": bson.M{"_id": id}})
	if err := GetMessengerCollection("threads").Pipe(topipe).One(&thread); err != nil {
		return nil, errors.New("could not get the thread")
	}
	return thread, nil
}

// THREAD

// SetDefaultName sets the threadname
func (thread *Thread) SetDefaultName() error {
	var users []string
	dbUsers, err := getUsers(thread.Participants)
	if err != nil {
		return err
	}

	for _, user := range dbUsers {
		users = append(users, user.Profile.FirstName)
	}
	thread.Name = strings.Join(users, ", ")
	return nil
}

// AddMessage saves a message onto a thead
func (thread *ThreadDto) AddMessage(message *Message) error {

	thread.LastMessage = message.GetDto(ThreadParticipant{
		ID: message.Sender,
	})
	thread.MessageTime = message.Time

	users := make([]bson.ObjectId, 0)
	for _, p := range thread.Participants {
		users = append(users, p.ID)
	}
	message.Users = users
	message.Time = time.Now()

	errup := GetMessengerCollection("threads").UpdateId(thread.ID, bson.M{
		"$set": bson.M{
			"message":      message.ID,
			"message_time": message.Time,
		},
	})

	if errup != nil {
		return errup
	}

	if err := GetMessengerCollection("messages").Insert(message); err != nil {
		return errors.Wrap(err, "could not inset message")
	}
	return nil
}

func undoAction(threadID bson.ObjectId, list *[]string, name string, user UserMgo) error {

	var deleted []string
	for _, u := range *list {
		if u != user.ID.Hex() {
			deleted = append(deleted, u)
		}
	}

	list = &deleted

	if err := GetMessengerCollection("threads").UpdateId(threadID, bson.M{
		"$set": bson.M{name: deleted},
	}); err != nil {
		return errors.Wrap(err, "could not undo action")
	}

	return nil

}

// UndeleteFor undeletes a thread for a user
func (thread *ThreadDto) UndeleteFor(user UserMgo) (err error) {

	if !thread.HasParticipant(user) {
		return errors.New("cannot undelete thread because user is not a participant of this thread")
	}

	if !thread.IsDeletedFor(user) {
		return errors.New("cannot undelete thread because it not deleted for user")
	}

	return undoAction(thread.ID, &thread.Deleted, "deleted", user)
}

// UnarchiveFor unarchives a thread for a user
func (thread *Thread) UnarchiveFor(user UserMgo) (err error) {
	return undoAction(thread.ID, &thread.Archived, "archived", user)
}

// DeleteFor deletes a thread for a user
func (thread *ThreadDto) DeleteFor(user UserMgo) error {

	//TODO: Check if has messages are only from user. Then delete it permanently
	//TODO: Check if thread messages are not seen by others. Then delete it permanently

	if !thread.HasParticipant(user) {
		return errors.New("You're not allowed to delete this thread since you are not in participants list")
	}

	if thread.IsDeletedFor(user) {
		return errors.New("This thread is already deleted. It will be permanently deleted when all participants delete it")
	}

	if thread.Deleted == nil {
		thread.Deleted = make([]string, 0)
	}

	thread.Deleted = append(thread.Deleted, user.ID.Hex())

	if len(thread.Deleted) == len(thread.Participants) {
		// Permanently delete thread and messages
		if _, err := GetMessengerCollection("messages").RemoveAll(bson.M{"thread": thread.ID}); err != nil {
			return errors.Wrap(err, "could not remove messaged related to threads")
		}

		if err := GetMessengerCollection("threads").RemoveId(thread.ID); err != nil {
			return errors.Wrap(err, "could not remove thread")
		}
		return nil
	}

	if err := GetMessengerCollection("threads").UpdateId(thread.ID, bson.M{
		"$set": bson.M{"deleted": thread.Deleted},
	}); err != nil {
		return errors.Wrap(err, "could not mark thread as deleted")
	}
	return nil
}

// ArchiveFor archives a thread for a user
func (thread *ThreadDto) ArchiveFor(user UserMgo) error {

	if !thread.HasParticipant(user) {
		return errors.New("You're not allowed to archive this thread since you are not in participants list")
	}

	if thread.IsArchivedFor(user) {
		return errors.New("thread is already archived")
	}

	if thread.Archived == nil {
		thread.Archived = make([]string, 0)
	}

	thread.Archived = append(thread.Archived, user.ID.Hex())

	if err := GetMessengerCollection("threads").UpdateId(thread.ID, bson.M{
		"$set": bson.M{"archived": thread.Archived},
	}); err != nil {
		return errors.Wrap(err, "could not set thread as archived")
	}
	return nil
}

// IsDeletedFor checks if this thread is deleted for user
func (thread *ThreadDto) IsDeletedFor(user UserMgo) bool {

	for _, participant := range thread.Deleted {
		if participant == user.ID.Hex() {
			return true
		}
	}

	return false
}

// IsArchivedFor checks if this thread is deleted for user
func (thread *ThreadDto) IsArchivedFor(user UserMgo) bool {

	for _, participant := range thread.Archived {
		if participant == user.ID.Hex() {
			return true
		}
	}

	return false
}

// HasParticipant checks if this thread has this participant
func (thread *ThreadDto) HasParticipant(user UserMgo) bool {

	for _, participant := range thread.Participants {
		if participant.ID == user.ID {
			return true
		}
	}

	return false
}

// ParticipantsIDS checks if this thread has this participant
func (thread *ThreadDto) ParticipantsIDS() (participants []string) {

	participants = make([]string, len(thread.Participants))

	for i, participant := range thread.Participants {
		participants[i] = participant.ID.Hex()
	}

	return
}

// Flag will flag a thread for an admin to review
func (thread *ThreadDto) Flag(user UserMgo, reason string) (*ThreadFlag, error) {
	tf := ThreadFlag{
		ID:       bson.NewObjectId(),
		Time:     time.Now(),
		ThreadID: thread.ID.Hex(),
		UserID:   user.ID.Hex(),
		Reason:   reason,
	}

	if err := GetMessengerCollection("flaggedThreads").Insert(tf); err != nil {
		return nil, errors.Wrap(err, "could not insert flag")
	}

	return &tf, nil
}

// ListFlaggedThreads will list all flagged threads
func ListFlaggedThreads(all bool) (threads []ThreadFlag, err error) {
	if all {
		err = GetMessengerCollection("flaggedThreads").Find(bson.D{}).All(&threads)
	} else {
		query := bson.M{"reviews.0": bson.M{"$exists": false}}
		err = GetMessengerCollection("flaggedThreads").Find(query).All(&threads)
	}
	return
}

// GetFlaggedThread returns the flagged thread that matches the passed in id
func GetFlaggedThread(id bson.ObjectId) (ThreadFlag, error) {
	tf := ThreadFlag{}
	err := GetMessengerCollection("flaggedThreads").FindId(id).One(&tf)
	return tf, err
}

//FlaggedThreadAddReview will add a reviewer to a thread
func FlaggedThreadAddReview(id bson.ObjectId, userID string) error {
	r := Review{UserID: userID, Time: time.Now()}
	update := bson.M{"$push": bson.M{"reviews": r}}
	return GetMessengerCollection("flaggedThreads").UpdateId(id, update)
}

func unwind(field string) bson.M {
	return bson.M{"$unwind": bson.M{
		"path":                       field,
		"preserveNullAndEmptyArrays": true,
	}}
}

func lookup(from, localField, foreignField, as string) bson.M {
	return bson.M{"$lookup": bson.M{
		"from":         from,
		"localField":   localField,
		"foreignField": foreignField,
		"as":           as,
	}}
}
