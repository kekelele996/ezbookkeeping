package storage

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// S3ObjectStorage represents S3-compatible object storage
type S3ObjectStorage struct {
	s3Client *s3.Client
	s3Config *settings.S3Config
	rootPath string
}

// NewS3ObjectStorage returns an S3-compatible object storage
func NewS3ObjectStorage(config *settings.Config, pathPrefix string) (*S3ObjectStorage, error) {
	s3Config := config.S3Config

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(s3Config.Region),
		awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			s3Config.AccessKeyID,
			s3Config.SecretAccessKey,
			s3Config.SessionToken,
		)),
	}

	if s3Config.SkipTLSVerify {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		loadOptions = append(loadOptions, awsconfig.WithHTTPClient(&http.Client{Transport: transport}))
	}

	ctx := context.Background()
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)

	if err != nil {
		return nil, err
	}

	s3Client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = s3Config.UsePathStyle

		if s3Config.Endpoint != "" {
			options.BaseEndpoint = aws.String(getS3Endpoint(s3Config))
		}
	})

	storage := &S3ObjectStorage{
		s3Client: s3Client,
		s3Config: s3Config,
		rootPath: s3Config.RootPath,
	}

	storage.rootPath = storage.getFinalPath(pathPrefix)
	storage.rootPath = strings.ReplaceAll(storage.rootPath, "\\", "/")

	_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s3Config.Bucket),
	})

	if err != nil {
		if !isS3BucketNotFoundError(err) {
			return nil, err
		}

		createBucketInput := &s3.CreateBucketInput{
			Bucket: aws.String(s3Config.Bucket),
		}

		createBucketInput.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(s3Config.Region),
		}

		_, err = s3Client.CreateBucket(ctx, createBucketInput)

		if err != nil {
			return nil, err
		}
	}

	return storage, nil
}

// Exists returns whether the file exists
func (s *S3ObjectStorage) Exists(ctx core.Context, path string) (bool, error) {
	output, err := s.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.s3Config.Bucket),
		Key:    aws.String(s.getFinalPath(path)),
	})

	if err != nil {
		if isS3ObjectNotFoundError(err) {
			return false, nil
		}

		return false, err
	}

	return !aws.ToBool(output.DeleteMarker), nil
}

// Read returns the object instance according to specified the file path
func (s *S3ObjectStorage) Read(ctx core.Context, path string) (ObjectInStorage, bool, error) {
	output, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.s3Config.Bucket),
		Key:    aws.String(s.getFinalPath(path)),
	})

	if err != nil {
		if isS3ObjectNotFoundError(err) {
			return nil, false, nil
		}

		return nil, false, err
	}

	defer output.Body.Close()

	if aws.ToBool(output.DeleteMarker) {
		return nil, false, nil
	}

	data, err := io.ReadAll(output.Body)

	if err != nil {
		return nil, false, err
	}

	return newByteSliceObject(data), true, nil
}

// Save returns whether save the object instance successfully
func (s *S3ObjectStorage) Save(ctx core.Context, path string, object ObjectInStorage) error {
	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.s3Config.Bucket),
		Key:    aws.String(s.getFinalPath(path)),
		Body:   object,
	})

	return err
}

// Delete returns whether delete the object according to specified the file path successfully
func (s *S3ObjectStorage) Delete(ctx core.Context, path string) error {
	_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.s3Config.Bucket),
		Key:    aws.String(s.getFinalPath(path)),
	})

	return err
}

// List returns metadata of all objects under the specified prefix
func (s *S3ObjectStorage) List(ctx core.Context, prefix string) ([]ObjectMeta, error) {
	listPrefix := s.getListPrefix(prefix)
	rootPathPrefix := s.getListPrefix("")
	objects := make([]ObjectMeta, 0)
	var continuationToken *string

	for {
		output, err := s.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.s3Config.Bucket),
			Prefix:            aws.String(listPrefix),
			ContinuationToken: continuationToken,
		})

		if err != nil {
			return nil, err
		}

		for i := 0; i < len(output.Contents); i++ {
			object := output.Contents[i]

			if aws.ToString(object.Key) == listPrefix {
				continue
			}

			relativePath := strings.TrimPrefix(aws.ToString(object.Key), rootPathPrefix)

			if relativePath == "" || strings.HasSuffix(relativePath, "/") {
				continue
			}

			objects = append(objects, ObjectMeta{
				Path:         relativePath,
				Size:         aws.ToInt64(object.Size),
				LastModified: aws.ToTime(object.LastModified),
			})
		}

		if !aws.ToBool(output.IsTruncated) {
			break
		}

		continuationToken = output.NextContinuationToken
	}

	return objects, nil
}

func (s *S3ObjectStorage) getListPrefix(prefix string) string {
	listPrefix := s.getFinalPath(prefix)

	if listPrefix != "" && !strings.HasSuffix(listPrefix, "/") {
		listPrefix = listPrefix + "/"
	}

	return listPrefix
}

func (s *S3ObjectStorage) getFinalPath(path string) string {
	rootPath := s.rootPath

	if len(rootPath) > 0 && rootPath[len(rootPath)-1] != '/' {
		rootPath = rootPath + "/"
	}

	if len(rootPath) > 0 && rootPath[0] == '/' {
		rootPath = rootPath[1:]
	}

	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	path = strings.ReplaceAll(path, "\\", "/")

	return rootPath + path
}

func getS3Endpoint(config *settings.S3Config) string {
	if strings.Contains(config.Endpoint, "://") {
		return config.Endpoint
	}

	if config.UseSSL {
		return "https://" + config.Endpoint
	}

	return "http://" + config.Endpoint
}

func isS3BucketNotFoundError(err error) bool {
	var apiError smithy.APIError

	if errors.As(err, &apiError) {
		switch apiError.(type) {
		case *types.NotFound:
			return true
		case *types.NoSuchBucket:
			return true
		default:
			return false
		}
	}

	return false
}

func isS3ObjectNotFoundError(err error) bool {
	var apiError smithy.APIError

	if errors.As(err, &apiError) {
		switch apiError.(type) {
		case *types.NotFound:
			return true
		case *types.NoSuchKey:
			return true
		default:
			return false
		}
	}

	return false
}
