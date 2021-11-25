package store

import (
	"fmt"
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"
	"time"
)

type FilesStore struct{}

func GetFilesStore() *FilesStore { return &FilesStore{} }

func (L *FilesStore) Get(id bson.ObjectId) (file *FilesMgo, exist bool) {
	exist = GetCollection("files").FindId(id).One(&file) == nil
	return
}

func (L *FilesStore) GetActiveFiles() (files []FilesMgo) {
	GetCollection("files").Find(bson.M{"deleted": false}).Sort("-created_at").All(&files)
	return
}

func (L *FilesStore) GetAllUserFiles(user *UserMgo) ([]*FilesMgo, error) {
	var files []*FilesMgo
	if err := GetCollection("files").FindId(bson.M{"$in": user.Files}).Sort("-created_at").All(&files); err != nil {
		return nil, errors.Wrap(err, "couldn't get files from database")
	}

	return files, nil
}

func (L *FilesStore) GetAllUserActiveFiles(user *UserMgo) ([]*FilesMgo, error) {
	var files []*FilesMgo
	query := bson.M{
		"$and": []bson.M{
			bson.M{"_id": bson.M{"$in": user.Files}},
			{"deleted": false},
		},
	}
	if err := GetCollection("files").Find(query).Sort("-created_at").All(&files); err != nil {
		return nil, errors.Wrap(err, "couldn't get files from database")
	}

	return files, nil
}

type UploadState int
const (
	UploadInitiated = UploadState(iota)
	UploadSucceeded
	UploadFailed
)

type Upload struct {
	ID bson.ObjectId `json:"_id" bson:"_id"`

	Name string `json:"name" bson:"name"`
	Mime string `json:"mime" bson:"mime"`
	Size int64  `json:"size" bson:"size"`
	URL  string `json:"url" bson:"url"`

	// Used to identify the type of this request
	// Context might be the folder where upload is stored in s3
	// or the type of the upload
	Context        string        `json:"context" bson:"context"`
	Checksum       string        `json:"checksum" bson:"checksum"`
	UploadedBy     bson.ObjectId `json:"uploaded_by,omitempty" bson:"uploaded_by,omitempty"`
	AddedToLibrary bool          `json:"added_to_library" bson:"added_to_library"`
	Expire         *time.Time    `json:"expire" bson:"expire"`
	State		   UploadState   `json:"state" bson:"state"`
}

func (u *Upload) Href() string {
	return fmt.Sprintf("https://s3.amazonaws.com/nerdly.io/%s", u.URL)
}

type FilesMgo struct {
	ID         bson.ObjectId `json:"_id" bson:"_id"`
	Name       string        `json:"name,omitempty" bson:"name"`
	Context    string        `json:"context" bson:"context"`
	URL        string        `json:"url" bson:"url"`
	Mime       string        `json:"mime" bson:"mime"`
	Size       int64         `json:"size" bson:"size"`
	Checksum   string        `json:"checksum" bson:"checksum"`
	UploadedBy bson.ObjectId `json:"uploaded_by,omitempty" bson:"uploaded_by,omitempty"`
	CreatedAt  time.Time     `json:"created_at" bson:"created_at"`
	Deleted    bool          `json:"deleted" bson:"deleted"`
	DeletedAt  *time.Time    `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

type FilesDto struct {
	ID         bson.ObjectId `json:"_id" bson:"_id"`
	Name       string        `json:"name,omitempty" bson:"name"`
	Context    string        `json:"context" bson:"context"`
	URL        string        `json:"url" bson:"url"`
	Mime       string        `json:"mime" bson:"mime"`
	Size       int64         `json:"size" bson:"size"`
	Checksum   string        `json:"checksum" bson:"checksum"`
	UploadedBy PublicUserDto `json:"uploaded_by" bson:"uploaded_by"`
	CreatedAt  time.Time     `json:"created_at" bson:"created_at"`
	Deleted    bool          `json:"deleted" bson:"deleted"`
	DeletedAt  *time.Time    `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

func (f *FilesMgo) DTO() (*FilesDto, error) {
	var uploader UserMgo
	if err := GetCollection("users").FindId(f.UploadedBy).One(&uploader); err != nil {
		return nil, errors.Wrap(err, "couldn't get uploader")
	}

	return &FilesDto{
		ID:         f.ID,
		Name:       f.Name,
		Context:    f.Context,
		URL:        f.URL,
		Mime:       f.Mime,
		Size:       f.Size,
		Checksum:   f.Checksum,
		UploadedBy: *uploader.ToPublicDto(),
		CreatedAt:  f.CreatedAt,
		Deleted:    f.Deleted,
		DeletedAt:  f.DeletedAt,
	}, nil
}

func (f *FilesMgo) DeleteFile() (err error) {
	now := time.Now()
	f.Deleted = true
	f.DeletedAt = &now
	return GetCollection("files").UpdateId(f.ID, bson.M{"$set": bson.M{"deleted": true, "deleted_at": now}})
}

func (f *FilesMgo) SaveNew() (err error) {
	if !f.ID.Valid() {
		f.ID = bson.NewObjectId()
	}

	return GetCollection("files").Insert(f)
}
