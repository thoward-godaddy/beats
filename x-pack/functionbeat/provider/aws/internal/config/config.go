package config

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"os"
)

const fileName = "functionbeat.yml"

func errCheck(err error) {
	if err != nil {
		panic(err)
	}
}

func fileExists(fileName string) bool {
	info, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		return false
	}

	return !info.IsDir()
}

func writeConfig(content []byte) {
	fmt.Println("Writing FunctionBeat configuration to disk")
	err := os.WriteFile(fileName, content, 0444)
	errCheck(err)
	fmt.Println("FunctionBeat configuration saved to disk")
}

func getConfigFromASM(secretId string) {
	fmt.Println("Fetching FunctionBeat configuration from ASM")
	sess := session.Must(session.NewSession())
	svc := secretsmanager.New(sess)
	result, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{SecretId: &secretId})

	errCheck(err)
	writeConfig([]byte(*result.SecretString))
}

func getConfigFromS3(bucketName string, bucketKey string) {
	fmt.Println("Fetching FunctionBeat configuration from S3")
	sess := session.Must(session.NewSession())
	buffer := &aws.WriteAtBuffer{}
	downloader := s3manager.NewDownloader(sess)
	_, err := downloader.Download(buffer, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(bucketKey),
	})

	errCheck(err)
	writeConfig(buffer.Bytes())
}

func Load() {
	if fileExists(fileName) {
		return
	}

	secretConfigId := os.Getenv("FB_CONFIG_SECRET_ID")
	s3ConfigBucketName := os.Getenv("FB_CONFIG_S3_BUCKET_NAME")
	s3ConfigBucketKey := os.Getenv("FB_CONFIG_S3_BUCKET_KEY")

	if len(secretConfigId) > 0 && len(s3ConfigBucketName) > 0 {
		panic(fmt.Errorf("can only load config from S3 or ASM. Not both"))
	}

	if len(secretConfigId) > 0 {
		getConfigFromASM(secretConfigId)
		return
	}

	if len(s3ConfigBucketName) > 0 {
		if len(s3ConfigBucketKey) == 0 {
			panic(fmt.Errorf("bucket Key must be provided"))
		}

		getConfigFromS3(s3ConfigBucketName, s3ConfigBucketKey)
		return
	}

	panic(fmt.Errorf("failed to find or load functiobeat configuration"))
}
