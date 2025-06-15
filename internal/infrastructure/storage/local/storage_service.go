package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/archivus/archivus/internal/domain/services"
	"github.com/google/uuid"
)

type StorageService struct {
	basePath string
}

func NewStorageService(basePath string) *StorageService {
	return &StorageService{
		basePath: basePath,
	}
}

func (s *StorageService) Store(ctx context.Context, params services.StorageParams) (string, error) {
	// Create tenant directory if it doesn't exist
	tenantDir := filepath.Join(s.basePath, params.TenantID.String())
	if err := os.MkdirAll(tenantDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create tenant directory: %w", err)
	}

	// Generate unique filename
	fileExt := filepath.Ext(params.Filename)
	fileName := fmt.Sprintf("%s%s", uuid.New().String(), fileExt)
	filePath := filepath.Join(tenantDir, fileName)

	// Create the file
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy content to file
	_, err = io.Copy(file, params.FileReader)
	if err != nil {
		return "", fmt.Errorf("failed to write file content: %w", err)
	}

	// Return relative path from base
	relativePath := filepath.Join(params.TenantID.String(), fileName)
	return relativePath, nil
}

func (s *StorageService) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.basePath, path)

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

func (s *StorageService) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(s.basePath, path)

	err := os.Remove(fullPath)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

func (s *StorageService) GeneratePresignedURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
	// For local storage, we can't generate presigned URLs
	// Return a simple URL that the application can serve
	return fmt.Sprintf("/api/v1/documents/download?path=%s", path), nil
}

func (s *StorageService) GetPublicURL(bucketName, filePath string) string {
	// For local storage, return a URL that the application can serve
	return fmt.Sprintf("/api/v1/files/%s", filePath)
}
