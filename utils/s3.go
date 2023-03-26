package utils

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func CreateS3Uploader() (*s3manager.Uploader, error) {
	// Initialize S3 session
	sess, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewCredentials(
			&credentials.EnvProvider{},
		),
	})

	if err != nil {
		return nil, err
	}

	// create an uploader
	uploader := s3manager.NewUploader(sess)

	return uploader, nil
}
