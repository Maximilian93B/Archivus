package supabase

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/archivus/archivus/internal/domain/services"
	"github.com/google/uuid"
	supabase "github.com/nedpals/supabase-go"
)

type StorageService struct {
	client     *supabase.Client
	bucketName string
}

type Config struct {
	URL    string
	APIKey string
	Bucket string
}

func NewStorageService(config Config) (*StorageService, error) {
	client := supabase.CreateClient(config.URL, config.APIKey)
	if client == nil {
		return nil, fmt.Errorf("failed to create Supabase client")
	}

	return &StorageService{
		client:     client,
		bucketName: config.Bucket,
	}, nil
}

func (s *StorageService) Store(ctx context.Context, params services.StorageParams) (string, error) {
	// Generate unique file path
	fileExt := filepath.Ext(params.Filename)
	fileName := fmt.Sprintf("%s/%s%s", params.TenantID.String(), uuid.New().String(), fileExt)

	// Read the file content
	content, err := io.ReadAll(params.FileReader)
	if err != nil {
		return "", fmt.Errorf("failed to read file content: %w", err)
	}

	// Upload to Supabase Storage using nedpals client
	fileOptions := &supabase.FileUploadOptions{
		ContentType: params.ContentType,
		Upsert:      false,
	}

	response := s.client.Storage.From(params.BucketName).Upload(fileName, bytes.NewReader(content), fileOptions)
	if response.Key == "" {
		return "", fmt.Errorf("failed to upload file to Supabase: %s", response.Message)
	}

	return fileName, nil
}

func (s *StorageService) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	// Download file from Supabase Storage
	content, err := s.client.Storage.From(s.bucketName).Download(path)
	if err != nil {
		return nil, fmt.Errorf("failed to download file from Supabase: %w", err)
	}

	return io.NopCloser(bytes.NewReader(content)), nil
}

func (s *StorageService) Delete(ctx context.Context, path string) error {
	// Delete file from Supabase Storage
	response := s.client.Storage.From(s.bucketName).Remove([]string{path})
	if response.Key == "" {
		return fmt.Errorf("failed to delete file from Supabase: %s", response.Message)
	}

	return nil
}

func (s *StorageService) GeneratePresignedURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
	// Generate signed URL for temporary access
	expirySeconds := int(expiry.Seconds())

	signedURL := s.client.Storage.From(s.bucketName).CreateSignedUrl(path, expirySeconds)
	if signedURL.SignedUrl == "" {
		return "", fmt.Errorf("failed to generate presigned URL")
	}

	return signedURL.SignedUrl, nil
}

func (s *StorageService) GetPublicURL(bucketName, filePath string) string {
	// Get public URL for the file - this method might not return exactly what we expect
	// For now, we'll construct a basic public URL or return the signed URL
	signedURL := s.client.Storage.From(bucketName).CreateSignedUrl(filePath, 3600) // 1 hour expiry
	return signedURL.SignedUrl
}
