package fetch

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Origin should be safe to use concurrently
type Origin interface {
	Get(name string) (int, io.Reader, error)
}

type S3Origin struct {
	client *s3.Client
	ctx    context.Context
	bucket string
}

func (o *S3Origin) Get(name string) (int, io.Reader, error) {
	result, err := o.client.GetObject(o.ctx, &s3.GetObjectInput{
		Bucket: aws.String(o.bucket),
		Key:    aws.String(name),
	})
	if err != nil {
		// TODO: not found
		log.Printf("error calling get object on s3 client: %v\n", err)
	}
	return int(*result.ContentLength), result.Body, nil
}

type DirOrigin struct {
	path string
}

func (o *DirOrigin) Get(name string) (int, io.Reader, error) {
	f, err := os.Open(o.path + "/" + name)
	if err != nil {
		return 0, nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		return 0, nil, err
	}
	size := int(stat.Size())
	return size, f, nil
}
