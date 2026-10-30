package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	DefaultBucketName = "social-finance-payment-proofs"
	DefaultMaxFileSize = 5 * 1024 * 1024 // 5 MB
)

var (
	ErrNotFound      = errors.New("storage: object not found")
	ErrFileTooLarge  = errors.New("storage: file exceeds maximum allowed size (5MB)")
	ErrInvalidFormat = errors.New("storage: unsupported image format, only JPEG, PNG, and WebP are allowed")
	ErrEmptyFile     = errors.New("storage: file is empty")
)

var allowedMimeTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type ObjectMetadata struct {
	Key          string
	OriginalName string
	ContentType  string
	Size         int64
	UploadedAt   time.Time
}

type ObjectReader struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

type Service interface {
	UploadProof(ctx context.Context, rtID, paymentID, filename string, r io.Reader, size int64) (*ObjectMetadata, error)
	GetProof(ctx context.Context, key string) (*ObjectReader, error)
	DeleteProof(ctx context.Context, key string) error
	UploadNotificationImage(ctx context.Context, rtID, eventType, filename string, r io.Reader, size int64) (*ObjectMetadata, error)
	GetNotificationImage(ctx context.Context, key string) (*ObjectReader, error)
	DeleteNotificationImage(ctx context.Context, key string) error
	InitBucket(ctx context.Context) error
}

type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	UseSSL          bool
	Region          string
	MaxFileSize     int64
}

type S3Storage struct {
	client     *minio.Client
	bucketName string
	maxFileSize int64
}

func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	endpoint := cfg.Endpoint
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimRight(endpoint, "/")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create s3 client: %w", err)
	}

	bucketName := cfg.BucketName
	if bucketName == "" {
		bucketName = DefaultBucketName
	}

	maxSize := cfg.MaxFileSize
	if maxSize <= 0 {
		maxSize = DefaultMaxFileSize
	}

	return &S3Storage{
		client:      client,
		bucketName:  bucketName,
		maxFileSize: maxSize,
	}, nil
}

func (s *S3Storage) InitBucket(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	exists, err := s.client.BucketExists(ctx, s.bucketName)
	if err != nil {
		return fmt.Errorf("check bucket %s: %w", s.bucketName, err)
	}

	if !exists {
		err = s.client.MakeBucket(ctx, s.bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("make bucket %s: %w", s.bucketName, err)
		}
	}

	return nil
}

func (s *S3Storage) UploadProof(ctx context.Context, rtID, paymentID, filename string, r io.Reader, size int64) (*ObjectMetadata, error) {
	if size <= 0 {
		return nil, ErrEmptyFile
	}
	if size > s.maxFileSize {
		return nil, ErrFileTooLarge
	}

	// Read first 512 bytes to sniff content type
	header := make([]byte, 512)
	n, err := io.ReadFull(r, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if n == 0 {
		return nil, ErrEmptyFile
	}

	contentType := http.DetectContentType(header[:n])
	// Also detect webp if not recognized by standard http.DetectContentType
	if contentType == "application/octet-stream" && n >= 12 {
		if string(header[0:4]) == "RIFF" && string(header[8:12]) == "WEBP" {
			contentType = "image/webp"
		}
	}

	ext, ok := allowedMimeTypes[contentType]
	if !ok {
		return nil, ErrInvalidFormat
	}

	// Recombine header with remaining body
	fullReader := io.MultiReader(bytes.NewReader(header[:n]), r)

	// Generate safe, unguessable key
	sanitizedName := filepath.Base(filename)
	sanitizedName = strings.ReplaceAll(sanitizedName, "/", "_")
	sanitizedName = strings.ReplaceAll(sanitizedName, "\\", "_")

	randomBytes := make([]byte, 8)
	_, _ = rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)

	now := time.Now()
	key := fmt.Sprintf("proofs/%s/%s_%d_%s%s", rtID, paymentID, now.UnixNano(), randomHex, ext)

	uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err = s.client.PutObject(uploadCtx, s.bucketName, key, fullReader, size, minio.PutObjectOptions{
		ContentType: contentType,
		UserMetadata: map[string]string{
			"original-name": sanitizedName,
			"rt-id":         rtID,
			"payment-id":    paymentID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("put object: %w", err)
	}

	return &ObjectMetadata{
		Key:          key,
		OriginalName: sanitizedName,
		ContentType:  contentType,
		Size:         size,
		UploadedAt:   now,
	}, nil
}

func (s *S3Storage) GetProof(ctx context.Context, key string) (*ObjectReader, error) {
	// Defense against path traversal or arbitrary keys
	if strings.Contains(key, "..") || !strings.HasPrefix(key, "proofs/") {
		return nil, ErrNotFound
	}

	getCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	stat, err := s.client.StatObject(getCtx, s.bucketName, key, minio.StatObjectOptions{})
	if err != nil {
		var errResp minio.ErrorResponse
		if errors.As(err, &errResp) && (errResp.Code == "NoSuchKey" || errResp.StatusCode == http.StatusNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("stat object: %w", err)
	}

	obj, err := s.client.GetObject(ctx, s.bucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}

	return &ObjectReader{
		Body:        obj,
		ContentType: stat.ContentType,
		Size:        stat.Size,
	}, nil
}

func (s *S3Storage) DeleteProof(ctx context.Context, key string) error {
	if strings.Contains(key, "..") || !strings.HasPrefix(key, "proofs/") {
		return ErrNotFound
	}

	delCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := s.client.RemoveObject(delCtx, s.bucketName, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("remove object: %w", err)
	}
	return nil
}

func (s *S3Storage) UploadNotificationImage(ctx context.Context, rtID, eventType, filename string, r io.Reader, size int64) (*ObjectMetadata, error) {
	if eventType != "waste_truck_arrival" && eventType != "manual_push" {
		return nil, errors.New("storage: images only allowed for waste_truck_arrival or manual_push")
	}
	if size <= 0 {
		return nil, ErrEmptyFile
	}
	if size > s.maxFileSize {
		return nil, ErrFileTooLarge
	}

	header := make([]byte, 512)
	n, err := io.ReadFull(r, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if n == 0 {
		return nil, ErrEmptyFile
	}

	contentType := http.DetectContentType(header[:n])
	if contentType == "application/octet-stream" && n >= 12 {
		if string(header[0:4]) == "RIFF" && string(header[8:12]) == "WEBP" {
			contentType = "image/webp"
		}
	}

	ext, ok := allowedMimeTypes[contentType]
	if !ok {
		return nil, ErrInvalidFormat
	}

	fullReader := io.MultiReader(bytes.NewReader(header[:n]), r)

	sanitizedName := filepath.Base(filename)
	sanitizedName = strings.ReplaceAll(sanitizedName, "/", "_")
	sanitizedName = strings.ReplaceAll(sanitizedName, "\\", "_")

	randomBytes := make([]byte, 8)
	_, _ = rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)

	now := time.Now()
	var category string
	if eventType == "waste_truck_arrival" {
		category = "waste-truck"
	} else {
		category = "manual"
	}
	key := fmt.Sprintf("notifications/%s/%s/%d_%s%s", category, rtID, now.UnixNano(), randomHex, ext)

	uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err = s.client.PutObject(uploadCtx, s.bucketName, key, fullReader, size, minio.PutObjectOptions{
		ContentType: contentType,
		UserMetadata: map[string]string{
			"original-name": sanitizedName,
			"rt-id":         rtID,
			"event-type":    eventType,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("put object: %w", err)
	}

	return &ObjectMetadata{
		Key:          key,
		OriginalName: sanitizedName,
		ContentType:  contentType,
		Size:         size,
		UploadedAt:   now,
	}, nil
}

func (s *S3Storage) GetNotificationImage(ctx context.Context, key string) (*ObjectReader, error) {
	if strings.Contains(key, "..") || !strings.HasPrefix(key, "notifications/") {
		return nil, ErrNotFound
	}

	getCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	stat, err := s.client.StatObject(getCtx, s.bucketName, key, minio.StatObjectOptions{})
	if err != nil {
		var errResp minio.ErrorResponse
		if errors.As(err, &errResp) && (errResp.Code == "NoSuchKey" || errResp.StatusCode == http.StatusNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("stat object: %w", err)
	}

	obj, err := s.client.GetObject(ctx, s.bucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}

	return &ObjectReader{
		Body:        obj,
		ContentType: stat.ContentType,
		Size:        stat.Size,
	}, nil
}

func (s *S3Storage) DeleteNotificationImage(ctx context.Context, key string) error {
	if strings.Contains(key, "..") || !strings.HasPrefix(key, "notifications/") {
		return ErrNotFound
	}

	delCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := s.client.RemoveObject(delCtx, s.bucketName, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("remove object: %w", err)
	}
	return nil
}
