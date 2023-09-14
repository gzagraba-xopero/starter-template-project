package upload

import (
	"bufio"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"s3upload/pkg/config"
	"s3upload/pkg/io"
	"strings"
)

type s3upload struct {
	s3          *s3.S3
	uploader    *s3manager.Uploader
	config      *config.S3Config
	connected   bool
	hiddenPaths map[string]bool
}

func (s *s3upload) Shutdown() error {
	return nil
}

type uploadTrackers struct {
	filesProcessed int64
	filesUploaded  int64
	filesSkipped   int64
	uploadErrors   map[string]error
}

func (u uploadTrackers) String() string {
	return fmt.Sprintf("uploaded: %d; skipped: %d, errors:%d", u.filesUploaded, u.filesSkipped, len(u.uploadErrors))
}

func (s *s3upload) UploadFrom(fromDir string) error {

	var err error

	if !s.connected {
		if err = s.Connect(); err != nil {
			return err
		}
	}

	if !filepath.IsAbs(fromDir) {
		fromDir, err = filepath.Abs(fromDir)
		if err != nil {
			return err
		}
	}

	var parent string
	if i := strings.LastIndex(fromDir, "/"); i == -1 {
		parent = fromDir
	} else {
		parent = fromDir[i:]
	}

	fmt.Printf("Uploading from %s (parent content is : %s)\n", fromDir, parent)

	tracked := uploadTrackers{
		uploadErrors: map[string]error{},
	}

	err = filepath.WalkDir(fromDir, func(path string, d fs.DirEntry, err error) error {

		if d.IsDir() {
			s.updateDirFilters(path)
			return nil
		}

		tracked.filesProcessed++

		if s.ignored(path, d) {
			tracked.filesSkipped++
			return nil
		}

		key := strings.TrimPrefix(path, fromDir)
		if key == "" {
			return nil
		}

		key = filepath.Join(s.config.BucketDest, parent, key)

		file, err := os.OpenFile(path, os.O_RDONLY, fs.ModePerm)
		if err != nil {
			return nil
		}

		defer io.CloseSilentWithErrorHandler(file, func(source interface{}, err error) { fmt.Printf("Error closing file %s: %s\n", path, err.Error()) })

		ext := filepath.Ext(key)
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "binary/octet-stream"
		}

		input := &s3manager.UploadInput{
			Bucket:      &s.config.BucketName,
			Key:         &key,
			ContentType: &mimeType,
			Body:        bufio.NewReader(file),
		}

		_, filename := filepath.Split(path)

		fmt.Printf("%s -> [S3:%s/%s]%s\n", filename, s.config.BucketEndpoint, s.config.BucketName, key)
		_, err = s.uploader.Upload(input)

		tracked.filesUploaded++

		return nil
	})

	println(tracked.String())

	return err
}

func (s s3upload) ignored(path string, d fs.DirEntry) bool {

	dir, filename := filepath.Split(path)

	if filename == ".env" {
		return false
	}

	return s.hiddenPaths[dir] || isDotPrefixed(filename)

}

func isDotPrefixed(name string) bool {
	return strings.HasPrefix(name, ".")
}

func (s *s3upload) createBucket() error {
	_, err := s.s3.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(s.config.BucketName),
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *s3upload) Connect() error {

	if s.connected {
		return nil
	}

	_, err := s.s3.HeadBucket(&s3.HeadBucketInput{
		Bucket: &s.config.BucketName,
	})
	if err != nil {
		if _, ok := err.(awserr.Error); ok {
			return s.createBucket()
		}
		return err
	}

	s.connected = true
	return nil
}

func (s *s3upload) updateDirFilters(path string) {
	_, name := filepath.Split(path)
	if isDotPrefixed(name) {
		s.hiddenPaths[path+"/"] = true
	}
}

func NewS3Upload(config config.S3Config) Upload {

	awsConfig := aws.NewConfig().
		WithRegion(config.Region).
		WithS3ForcePathStyle(true).
		WithEndpoint(config.BucketEndpoint).
		WithCredentials(credentials.NewEnvCredentials())

	awsSession := session.Must(session.NewSession(awsConfig))
	s3Client := s3.New(awsSession)
	uploader := s3manager.NewUploader(awsSession)

	var this = &s3upload{
		s3:          s3Client,
		uploader:    uploader,
		config:      &config,
		hiddenPaths: make(map[string]bool),
	}

	return this
}
