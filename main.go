package main

import (
	"github.com/joho/godotenv"
	"os"
	"s3upload/pkg/config"
	"s3upload/pkg/upload"
)

func init() {
	_ = godotenv.Load()
}

func main() {

	uploader := upload.NewS3Upload(config.S3Config{
		Region:         os.Getenv("S3_UPLOAD_REGION"),
		BucketName:     os.Getenv("S3_UPLOAD_BUCKET"),
		BucketEndpoint: os.Getenv("S3_UPLOAD_ENDPOINT"),
		BucketDest:     os.Getenv("S3_UPLOAD_BUCKET_PATH"),
	})

	defer func(u upload.Upload) { _ = u.Shutdown() }(uploader)

	err := uploader.UploadFrom(os.Getenv("S3_UPLOAD_SOURCE_PATH"))

	if err != nil {
		panic(err)
	}
}
