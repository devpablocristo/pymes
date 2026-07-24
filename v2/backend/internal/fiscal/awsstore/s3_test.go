package awsstore

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

func TestS3StoreCreatesImmutableIntegrityCheckedObjects(t *testing.T) {
	client := &memoryS3{objects: make(map[string]memoryS3Object)}
	store, err := NewS3Store(client, "fiscal-private", "pymes-v2")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	object := fiscal.ImmutableObject{
		Key:         "fiscal/org/voucher/pdf",
		ContentType: "application/pdf",
		Body:        []byte("%PDF immutable"),
	}
	if err := store.PutImmutable(context.Background(), object); err != nil {
		t.Fatal(err)
	}
	stored := client.objects["pymes-v2/fiscal/org/voucher/pdf"]
	if stored.serverSideEncryption != s3types.ServerSideEncryptionAes256 ||
		stored.ifNoneMatch != "*" {
		t.Fatalf("S3 protection = %q, %q", stored.serverSideEncryption, stored.ifNoneMatch)
	}
	if err := store.PutImmutable(context.Background(), object); err != nil {
		t.Fatalf("idempotent PutImmutable() error = %v", err)
	}
	got, err := store.Get(context.Background(), object.Key)
	if err != nil || !bytes.Equal(got.Body, object.Body) ||
		got.ContentType != object.ContentType || got.SHA256 == "" {
		t.Fatalf("Get() = %+v, %v", got, err)
	}
	conflict := object
	conflict.Body = []byte("different")
	if err := store.PutImmutable(context.Background(), conflict); err == nil ||
		!strings.Contains(err.Error(), "different bytes") {
		t.Fatalf("conflicting PutImmutable() error = %v", err)
	}
	if _, err := store.Get(context.Background(), "../private"); err == nil {
		t.Fatal("S3 logical-key traversal succeeded")
	}
}

func TestS3StoreRejectsTamperedMetadata(t *testing.T) {
	client := &memoryS3{objects: make(map[string]memoryS3Object)}
	store, err := NewS3Store(client, "fiscal-private", "")
	if err != nil {
		t.Fatal(err)
	}
	object := fiscal.ImmutableObject{
		Key: "fiscal/object", ContentType: "application/xml", Body: []byte("<ok/>"),
	}
	if err := store.PutImmutable(context.Background(), object); err != nil {
		t.Fatal(err)
	}
	stored := client.objects[object.Key]
	stored.metadata["sha256"] = strings.Repeat("0", 64)
	client.objects[object.Key] = stored
	if _, err := store.Get(context.Background(), object.Key); err == nil ||
		!strings.Contains(err.Error(), "integrity") {
		t.Fatalf("tampered Get() error = %v", err)
	}
}

type memoryS3Object struct {
	body                 []byte
	contentType          string
	metadata             map[string]string
	ifNoneMatch          string
	serverSideEncryption s3types.ServerSideEncryption
}

type memoryS3 struct {
	objects map[string]memoryS3Object
}

func (client *memoryS3) PutObject(
	_ context.Context,
	input *s3.PutObjectInput,
	_ ...func(*s3.Options),
) (*s3.PutObjectOutput, error) {
	key := aws.ToString(input.Key)
	if _, exists := client.objects[key]; exists {
		return nil, &smithy.GenericAPIError{
			Code: "PreconditionFailed", Message: "conditional create failed",
		}
	}
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	metadata := make(map[string]string, len(input.Metadata))
	for key, value := range input.Metadata {
		metadata[key] = value
	}
	client.objects[key] = memoryS3Object{
		body:                 body,
		contentType:          aws.ToString(input.ContentType),
		metadata:             metadata,
		ifNoneMatch:          aws.ToString(input.IfNoneMatch),
		serverSideEncryption: input.ServerSideEncryption,
	}
	return &s3.PutObjectOutput{}, nil
}

func (client *memoryS3) GetObject(
	_ context.Context,
	input *s3.GetObjectInput,
	_ ...func(*s3.Options),
) (*s3.GetObjectOutput, error) {
	object, exists := client.objects[aws.ToString(input.Key)]
	if !exists {
		return nil, &smithy.GenericAPIError{Code: "NoSuchKey", Message: "missing"}
	}
	metadata := make(map[string]string, len(object.metadata))
	for key, value := range object.metadata {
		metadata[key] = value
	}
	return &s3.GetObjectOutput{
		Body:        io.NopCloser(bytes.NewReader(object.body)),
		ContentType: aws.String(object.contentType),
		Metadata:    metadata,
	}, nil
}

func (client *memoryS3) HeadBucket(
	context.Context,
	*s3.HeadBucketInput,
	...func(*s3.Options),
) (*s3.HeadBucketOutput, error) {
	return &s3.HeadBucketOutput{}, nil
}
