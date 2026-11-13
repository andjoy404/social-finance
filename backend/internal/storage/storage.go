package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	_ "golang.org/x/image/webp"
)

const (
	DefaultBucketName  = "social-finance-payment-proofs"
	DefaultMaxFileSize = 5 * 1024 * 1024 // 5 MB

	// Image decode safety limits (checked before full decode to mitigate
	// decompression bombs and excessive memory use).
	//
	// Limits are chosen conservatively for profile/community photos:
	//   - 12 MP max => decoded RGBA image ~48 MB worst case (w*h*4), plus the
	//     original compressed buffer (<= 5 MB) and thumbnail output (~<1 MB).
	//   - 4096 px max dimension covers high-resolution photos while preventing
	//     extreme aspect-ratio/pixel-bomb inputs.
	// Combined with a bounded concurrency semaphore below, concurrent uploads do
	// not allocate unbounded decode memory.
	MaxImageDimension = 4096
	MaxImagePixels    = 12 * 1024 * 1024 // 12 megapixels

	// maxConcurrentImageDecodes bounds parallel decode+thumbnail work to cap
	// peak memory when multiple uploads run concurrently.
	maxConcurrentImageDecodes = 2

	thumbnailSize        = 200
	thumbnailJPEGQuality = 82
)

var imageDecodeSem = make(chan struct{}, maxConcurrentImageDecodes)

var (
	ErrNotFound       = errors.New("storage: object not found")
	ErrFileTooLarge   = errors.New("storage: file exceeds maximum allowed size (5MB)")
	ErrInvalidFormat  = errors.New("storage: unsupported image format, only JPEG, PNG, and WebP are allowed")
	ErrEmptyFile      = errors.New("storage: file is empty")
	ErrImageTooLarge  = errors.New("storage: image dimensions exceed allowed limits")
	ErrImageDecode    = errors.New("storage: unable to decode image")
	ErrImageCorrupted = errors.New("storage: image data is invalid or corrupted")
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
	UploadCommunityImage(ctx context.Context, rtID, category, filename string, r io.Reader, size int64) (*ObjectMetadata, error)
	UploadCommunityImageWithThumb(ctx context.Context, rtID, category, filename string, r io.Reader, size int64, makeThumb bool) (*ImageUploadResult, error)
	GetCommunityImage(ctx context.Context, key string) (*ObjectReader, error)
	DeleteCommunityImage(ctx context.Context, key string) error
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
	client      *minio.Client
	bucketName  string
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

func (s *S3Storage) UploadCommunityImage(ctx context.Context, rtID, category, filename string, r io.Reader, size int64) (*ObjectMetadata, error) {
	if category != "activity" && category != "announcement" && category != "banner" && category != "profile" && category != "avatar" && category != "resident" {
		category = "general"
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

	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("generate object key random component: %w", err)
	}
	randomHex := hex.EncodeToString(randomBytes)

	now := time.Now()
	key := fmt.Sprintf("community/%s/%s/%d_%s%s", rtID, category, now.UnixNano(), randomHex, ext)

	uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err = s.client.PutObject(uploadCtx, s.bucketName, key, fullReader, size, minio.PutObjectOptions{
		ContentType: contentType,
		UserMetadata: map[string]string{
			"original-name": sanitizedName,
			"rt-id":         rtID,
			"category":      category,
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

func (s *S3Storage) UploadCommunityImageWithThumb(ctx context.Context, rtID, category, filename string, r io.Reader, size int64, makeThumb bool) (*ImageUploadResult, error) {
	if category != "activity" && category != "announcement" && category != "banner" && category != "profile" && category != "avatar" && category != "resident" {
		category = "general"
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

	// 128-bit cryptographic random component for unguessable object keys.
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("generate object key random component: %w", err)
	}
	randomHex := hex.EncodeToString(randomBytes)

	now := time.Now()
	baseKey := fmt.Sprintf("community/%s/%s/%d_%s", rtID, category, now.UnixNano(), randomHex)

	origKey := baseKey + ext
	origMetadata := &ObjectMetadata{
		Key:          origKey,
		OriginalName: sanitizedName,
		ContentType:  contentType,
		Size:         size,
		UploadedAt:   now,
	}

	// Read entire body into a buffer so we can both decode and re-upload.
	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(io.LimitReader(fullReader, size)); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	origBody := body.Bytes()
	if int64(len(origBody)) != size {
		return nil, fmt.Errorf("read body: expected %d bytes, got %d", size, len(origBody))
	}

	// Bound concurrent decode+thumbnail work to limit peak memory usage.
	imageDecodeSem <- struct{}{}
	defer func() { <-imageDecodeSem }()

	// Check dimensions/pixel count before full decode to mitigate decompression bombs,
	// then fully decode with EXIF orientation handling.
	img, err := decodeAndValidateImage(origBody)
	if err != nil {
		return nil, err
	}

	// Upload original (full resolution, unmodified bytes) image first.
	if _, err := s.client.PutObject(ctx, s.bucketName, origKey, bytes.NewReader(origBody), int64(len(origBody)), minio.PutObjectOptions{
		ContentType: contentType,
		UserMetadata: map[string]string{
			"original-name": sanitizedName,
			"rt-id":         rtID,
			"category":      category,
		},
	}); err != nil {
		return nil, fmt.Errorf("upload object: %w", err)
	}

	var thumbKey string
	if makeThumb {
		thumbKey = baseKey + "_thumb.jpg"
		thumbImg, err := s.makeThumbnail(img)
		if err != nil {
			if remErr := s.client.RemoveObject(ctx, s.bucketName, origKey, minio.RemoveObjectOptions{}); remErr != nil {
				slog.Warn("thumbnail cleanup failed",
					"operation", "delete_original_after_thumbnail_create_failure",
					"object_key", origKey,
					"rt_id", rtID,
					"error", remErr,
				)
			}
			return nil, fmt.Errorf("create thumbnail: %w", err)
		}
		if _, err := s.client.PutObject(ctx, s.bucketName, thumbKey, bytes.NewReader(thumbImg), int64(len(thumbImg)), minio.PutObjectOptions{
			ContentType: "image/jpeg",
			UserMetadata: map[string]string{
				"original-name": sanitizedName,
				"rt-id":         rtID,
				"category":      category,
			},
		}); err != nil {
			if remErr := s.client.RemoveObject(ctx, s.bucketName, origKey, minio.RemoveObjectOptions{}); remErr != nil {
				slog.Warn("thumbnail cleanup failed",
					"operation", "delete_original_after_thumbnail_upload_failure",
					"object_key", origKey,
					"rt_id", rtID,
					"error", remErr,
				)
			}
			return nil, fmt.Errorf("upload thumbnail: %w", err)
		}
	}

	return &ImageUploadResult{
		OriginalKey: origKey,
		ThumbKey:    thumbKey,
		Original:    *origMetadata,
	}, nil
}

// decodeAndValidateImage checks dimensions/pixel count before full decode to
// mitigate decompression bombs, then fully decodes with EXIF orientation applied.
func decodeAndValidateImage(data []byte) (image.Image, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrImageDecode, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, ErrImageCorrupted
	}
	if cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension {
		return nil, ErrImageTooLarge
	}
	if int64(cfg.Width)*int64(cfg.Height) > MaxImagePixels {
		return nil, ErrImageTooLarge
	}
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrImageDecode, err)
	}
	return img, nil
}

// makeThumbnailJPEG produces a 200x200 JPEG thumbnail, handling transparency by
// compositing onto a white background.
//
// Large images are center-cropped to fill 200x200 (imaging.Fill). Images smaller
// than 200x200 in either dimension are not upscaled: they are scaled down only
// to fit within 200x200 preserving aspect ratio, then centered on a 200x200
// white canvas to avoid blur from excessive upscaling.
func makeThumbnailJPEG(img image.Image) ([]byte, error) {
	if !isOpaque(img) {
		bounds := img.Bounds()
		bg := imaging.New(bounds.Dx(), bounds.Dy(), color.White)
		img = imaging.Overlay(bg, img, image.Pt(0, 0), 1.0)
	}

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	var thumb image.Image
	if w > 0 && h > 0 && (w < thumbnailSize || h < thumbnailSize) {
		fitted := imaging.Fit(img, thumbnailSize, thumbnailSize, imaging.Lanczos)
		canvas := imaging.New(thumbnailSize, thumbnailSize, color.White)
		fb := fitted.Bounds()
		x := (thumbnailSize - fb.Dx()) / 2
		y := (thumbnailSize - fb.Dy()) / 2
		thumb = imaging.Paste(canvas, fitted, image.Pt(x, y))
	} else {
		thumb = imaging.Fill(img, thumbnailSize, thumbnailSize, imaging.Center, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, thumb, imaging.JPEG, imaging.JPEGQuality(thumbnailJPEGQuality)); err != nil {
		return nil, fmt.Errorf("encode thumbnail jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

// makeThumbnail produces a 200x200 JPEG thumbnail, handling transparency by
// compositing onto a white background.
func (s *S3Storage) makeThumbnail(img image.Image) ([]byte, error) {
	return makeThumbnailJPEG(img)
}

// isOpaque reports whether the image is fully opaque.
func isOpaque(img image.Image) bool {
	if img == nil {
		return false
	}
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return false
}

type ImageUploadResult struct {
	OriginalKey string
	ThumbKey    string
	Original    ObjectMetadata
}

func (s *S3Storage) GetCommunityImage(ctx context.Context, key string) (*ObjectReader, error) {
	if strings.Contains(key, "..") || !strings.HasPrefix(key, "community/") {
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

func (s *S3Storage) DeleteCommunityImage(ctx context.Context, key string) error {
	if strings.Contains(key, "..") || !strings.HasPrefix(key, "community/") {
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

	// 128-bit cryptographic random component for unguessable object keys.
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("generate object key random component: %w", err)
	}
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
