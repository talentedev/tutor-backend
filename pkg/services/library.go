package services

import (
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

type Library struct {
	store *store.FilesStore
}

// GetLessons gets an object used to interact with the Lessons db
func GetLibrary() *Library {
	return &Library{
		store: store.GetFilesStore(),
	}
}

func (l *Library) ByID(id bson.ObjectId) (f *store.FilesDto, exist bool) {
	var file *store.FilesMgo
	file, exist = l.store.Get(id)
	if exist {
		f, _ = file.DTO()
	}

	return
}

func (l *Library) ActiveFilesByUser(user *store.UserMgo) ([]*store.FilesDto, error) {
	fs, err := l.store.GetAllUserActiveFiles(user)
	if err != nil {
		return nil, err
	}

	files := make([]*store.FilesDto, len(fs))
	for i, file := range fs {
		files[i], err = file.DTO()
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func (l *Library) FilesByUser(user *store.UserMgo) ([]*store.FilesDto, error) {
	fs, err := l.store.GetAllUserFiles(user)
	if err != nil {
		return nil, err
	}

	files := make([]*store.FilesDto, len(fs))
	for i, file := range fs {
		files[i], err = file.DTO()
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}
