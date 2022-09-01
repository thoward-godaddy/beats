package config

import (
	"encoding/json"
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
	err := os.WriteFile("/tmp/"+fileName, content, 0444)
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
	setAuthEnvVars()
}

// TODO: Make this Generic
func setAuthEnvVars() {
	type EsspDeploymentCredentials struct {
		CloudId       string `json:"deployment_id"`
		CloudAuthUser string `json:"ingestion_user"`
		CloudAuthPass string `json:"ingestion_user_password"`
	}

	var esspDeploymentCredentials EsspDeploymentCredentials

	fmt.Println("Fetching FunctionBeat auth environment variables")
	secretId := "essp_deployment_credentials"
	sess := session.Must(session.NewSession())
	svc := secretsmanager.New(sess)
	result, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{SecretId: &secretId})
	errCheck(err)

	err = json.Unmarshal([]byte(*result.SecretString), &esspDeploymentCredentials)
	errCheck(err)

	_ = os.Setenv("CLOUD_ID", esspDeploymentCredentials.CloudId)
	_ = os.Setenv("CLOUD_AUTH_USER", esspDeploymentCredentials.CloudAuthUser)
	_ = os.Setenv("CLOUD_AUTH_PASS", esspDeploymentCredentials.CloudAuthPass)
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
	setAuthEnvVars()
}

func Load() bool {
	if fileExists(fileName) {
		return false
	}

	if fileExists("/tmp/" + fileName) {
		return true
	}

	secretConfigId := os.Getenv("FB_CONFIG_SECRET_ID")
	s3ConfigBucketName := os.Getenv("FB_CONFIG_S3_BUCKET_NAME")
	s3ConfigBucketKey := os.Getenv("FB_CONFIG_S3_BUCKET_KEY")

	if len(secretConfigId) > 0 && len(s3ConfigBucketName) > 0 {
		panic(fmt.Errorf("can only load config from S3 or ASM. Not both"))
	}

	if len(secretConfigId) > 0 {
		getConfigFromASM(secretConfigId)
		return true
	}

	if len(s3ConfigBucketName) > 0 {
		if len(s3ConfigBucketKey) == 0 {
			panic(fmt.Errorf("bucket Key must be provided"))
		}

		getConfigFromS3(s3ConfigBucketName, s3ConfigBucketKey)
		return true
	}

	panic(fmt.Errorf("failed to find or load functiobeat configuration"))
}
