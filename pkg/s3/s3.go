package s3

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/pkg/errors"
	"io/ioutil"
)

var ErrInvalidParameter = errors.New("invalid_parameter")

type S3 struct {
	s3 s3iface.S3API
}

type file struct {
	content       []byte
	contentType   string
	contentLength int64
}

// New returns a new S3.
func New(config *aws.Config) *S3 {
	return &S3{s3: s3.New(session.New(), config)}
}

// CreateBucket creates an s3 bucket.
func (s S3) CreateBucket(name string) error {

	if name == "" {
		return errors.Wrap(ErrInvalidParameter, "name")
	}

	in := &s3.CreateBucketInput{
		Bucket: aws.String(name),
	}

	_, err := s.s3.CreateBucket(in)
	if err != nil {
		return errors.Wrapf(err, "unable to create bucket %s", name)
	}

	return nil

}

// GetObject fetches an object from a given path.
func (s S3) GetObject(bucket, path string) ([]byte, error) {

	if bucket == "" {
		return nil, errors.Wrap(ErrInvalidParameter, "bucket")
	}
	if path == "" {
		return nil, errors.Wrap(ErrInvalidParameter, "path")
	}

	in := GetObjectInput(bucket, path)

	out, err := s.s3.GetObject(in)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get object bucket %s/%s", bucket, path)
	}

	return ioutil.ReadAll(out.Body)
}

func GetObjectInput(bucket, path string) *s3.GetObjectInput {
	return &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	}
}

// PutObject uploads an object to a bucket.
func (s S3) PutObject(bucket, name, fileName, mime string, data *bytes.Reader, checksum string, download bool) error {
	if bucket == "" {
		return errors.Wrap(ErrInvalidParameter, "bucket")
	}
	if name == "" {
		return errors.Wrap(ErrInvalidParameter, "name")
	}

	in := PutObjectInput(bucket, name, mime, data, checksum)

	if download {
		in.ContentDisposition = aws.String(fmt.Sprintf("attachment; filename=%s;", fileName))
	}

	_, err := s.s3.PutObject(in)
	if err != nil {
		return errors.Wrapf(err, "unable to put object %s", name)
	}

	return nil
}

func PutObjectInput(bucket, name, mime string, data *bytes.Reader, checksum string) *s3.PutObjectInput {
	return &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(name),
		ContentType:   aws.String(mime),
		Body:          data,
		ContentLength: aws.Int64(data.Size()),
		ContentMD5:    aws.String(checksum),
	}
}

func (s S3) DeleteObject(bucket, name string) error {
	if bucket == "" {
		return errors.Wrap(ErrInvalidParameter, "bucket")
	}
	if name == "" {
		return errors.Wrap(ErrInvalidParameter, "name")
	}

	in := DeleteObjectInput(bucket, name)

	_, err := s.s3.DeleteObject(in)
	if err != nil {
		return errors.Wrapf(err, "unable to put object %s", name)
	}

	return nil
}

func DeleteObjectInput(bucket, name string) *s3.DeleteObjectInput {
	return &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(name),
	}
}
