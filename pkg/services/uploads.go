package services

import (
	"bytes"
	"crypto/md5"
	"crypto/subtle"
	b64 "encoding/base64"
	"fmt"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	sess "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/s3"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

const (
	tempScale      = time.Minute
	uploadLifetime = tempScale * 60
	flushInterval  = tempScale
)

type uploads struct {
	stop chan int
	// do we need them in memory?
	temps map[string]*store.Upload
	mux   sync.Mutex
}

var (
	conf  *config.Config           = config.GetConfig()
	creds *credentials.Credentials = credentials.NewStaticCredentials(
		conf.GetString("service.amazon.key"),
		conf.GetString("service.amazon.secret"),
		"",
	)
	s3Config *aws.Config = aws.NewConfig().WithRegion("us-east-1").WithCredentials(creds)
	Uploads  *uploads
	client   *s3.S3
)

func init() {
	Uploads = new(uploads)
	Uploads.stop = make(chan int)
	Uploads.temps = make(map[string]*store.Upload)

	Uploads.TempFlush()

	client = s3.New(s3Config)

	ticker := time.NewTicker(15 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				Uploads.TempFlush()
			}
		}
	}()
}

func session() (*sess.Session, error) {
	return sess.NewSession(s3Config)
}

func (up *uploads) Delete(upload *store.Upload) (err error) {
	return removeS3Object(upload)
}

func (up *uploads) TempFlush() {
	go func() {
		for {
			select {
			case <-up.stop:
				return
			case <-time.After(flushInterval):
				if len(up.temps) == 0 {
					continue
				}

				up.mux.Lock()
				for id, u := range up.temps {
					if time.Now().After(*u.Expire) {
						logger.Get().Infof("Upload %s expired", u.ID.Hex())
						go removeS3Object(up.temps[id])
						delete(up.temps, id)
					}
				}
				up.mux.Unlock()
			}
		}
	}()
}

func (up *uploads) GetTempUploads() (uploads map[string]*store.Upload) {
	return up.temps
}

func (up *uploads) Valid(u *store.Upload) (exist bool) {
	u, exist = up.temps[u.ID.Hex()]
	return
}

func (up *uploads) Get(id bson.ObjectId) (upload *store.Upload, err error) {
	u, exist := up.temps[id.Hex()]
	if !exist {
		return nil, errors.New("Upload missing")
	}

	return u, nil
}

func (up *uploads) GetAndApprove(id bson.ObjectId) (upload *store.Upload, err error) {
	u, exist := up.temps[id.Hex()]
	if !exist {
		return nil, errors.New("Upload missing")
	}

	if err := up.Approve(u); err != nil {
		return nil, err
	}

	return u, err
}

func (up *uploads) Approve(upload *store.Upload) (err error) {
	if !up.Valid(upload) {
		return errors.New("upload is missing or expired")
	}

	delete(up.temps, upload.ID.Hex())

	return
}

func (up *uploads) upload(user *store.UserMgo, context, name string, file *multipart.File, download bool) (upload *store.Upload, err error) {
	buffer, err := ioutil.ReadAll(*file)
	if err != nil {
		core.PrintError(err, "upload")
		debug.PrintStack()
		return
	}
	mimeType := mime.TypeByExtension(filepath.Ext(name))
	if mimeType == "" {
		mimeType = http.DetectContentType(buffer)
	}
	data := bytes.NewReader(buffer)
	id := bson.NewObjectId()

	url := fmt.Sprintf("%s/%s", context, id.Hex())

	checksum := hash(buffer)

	var uid bson.ObjectId
	expire := time.Now().Add(uploadLifetime)
	if user == nil {
		uid = bson.NewObjectId()
	} else {
		uid = user.ID
	}
	upload = createUpload(id, uid, name, data.Size(), mimeType, url, context, &expire, checksum)

	go func() {
		if err = uploadedFile(url, name, mimeType, data, checksum, download); err != nil {
			core.PrintError(err, "upload")
			debug.PrintStack()
			up.mux.Lock()
			up.temps[upload.ID.Hex()].State = store.UploadFailed
			up.mux.Unlock()
			return
		}

		up.mux.Lock()
		up.temps[upload.ID.Hex()].State = store.UploadSucceeded
		up.mux.Unlock()
	}()

	return
}

func (up *uploads) Upload(user *store.UserMgo, context, name string, file *multipart.File, download bool) (upload *store.Upload, err error) {
	upload, err = up.upload(user, context, name, file, download)
	if err != nil {
		return
	}
	up.temps[upload.ID.Hex()] = upload

	return
}

func (up *uploads) UploadS3(user *store.UserMgo, context, name string, file *multipart.File, download bool) (upload *store.Upload, err error) {
	return up.upload(user, context, name, file, download)
}

func (up *uploads) RemoveFile(context string, id bson.ObjectId) error {
	return client.DeleteObject(conf.GetString("service.amazon.bucket"), fmt.Sprintf("%s/%s", context, id.Hex()))
}

func (up *uploads) FetchAndMove(f *store.Upload, context string) (upload *store.Upload, err error) {
	path, err := filepath.Abs(conf.App.Temp)
	if err != nil {
		return nil, err
	}

	file, err := ioutil.TempFile(path, "learnt")
	if err != nil {
		return nil, fmt.Errorf("Unable to open temporary file: %v", err)
	}

	if err = fetchFile(file, f.URL); err != nil {
		return nil, err
	}

	buffer, err := ioutil.ReadAll(file)
	data := bytes.NewReader(buffer)
	url := fmt.Sprintf("%s/%s", context, f.ID.Hex())
	checksum := hash(buffer)

	if err = compareChecksum(checksum, f.Checksum); err != nil {
		return
	}

	go uploadedFile(url, f.Name, f.Mime, data, f.Checksum, false)

	// keep the id but don't update temp[id], otherwise it'll be flushed.
	upload = createUpload(f.ID, f.UploadedBy, f.Name, data.Size(), f.Mime, url, context, nil, f.Checksum)
	if err := file.Close(); err != nil {
		return nil, err
	}

	if err := os.Remove(file.Name()); err != nil {
		return nil, err
	}
	return
}

func compareChecksum(checksum1, checksum2 string) error {
	sDec1, _ := b64.StdEncoding.DecodeString(checksum1)
	sDec2, _ := b64.StdEncoding.DecodeString(checksum2)

	if subtle.ConstantTimeCompare(sDec1, sDec2) == 0 {
		return fmt.Errorf("file failed to match checksum.")
	}

	return nil
}

func hash(buf []byte) string {
	hasher := md5.New()
	hasher.Write(buf)
	return b64.StdEncoding.EncodeToString(hasher.Sum(nil))
}

func createUpload(id, uid bson.ObjectId, name string, size int64, mime, url, context string, expiration *time.Time, checksum string) *store.Upload {
	return &store.Upload{
		ID:         id,
		Name:       name,
		Size:       size,
		Mime:       mime,
		URL:        url,
		Context:    context,
		Expire:     expiration,
		Checksum:   checksum,
		UploadedBy: uid,
	}
}

func fetchFile(file *os.File, url string) error {
	s, err := session()
	if err != nil {
		return fmt.Errorf("Unable to create S3 session: %v", err)
	}
	downloader := s3manager.NewDownloader(s)

	_, err = downloader.Download(file, s3.GetObjectInput(conf.GetString("service.amazon.bucket"), url))

	if err != nil {
		return fmt.Errorf("Unable to download file %s: %v", url, err)
	}

	return nil
}

func uploadedFile(url, name, mime string, data *bytes.Reader, checksum string, download bool) (err error) {
	return client.PutObject(conf.GetString("service.amazon.bucket"), url, name, mime, data, checksum, download)
}

func removeS3Object(upload *store.Upload) (err error) {
	return client.DeleteObject(conf.GetString("service.amazon.bucket"), fmt.Sprintf("%s/%s", upload.Context, upload.ID.Hex()))
}
