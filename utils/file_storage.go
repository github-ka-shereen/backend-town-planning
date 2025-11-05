package utils

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

type FileStorage interface {
	UploadFile(file multipart.File, fileName string) (string, error)
	UploadFileFromReader(src io.Reader, fileName string) (string, error)
	DownloadFile(filePath string) (io.ReadCloser, error)
	DeleteFile(filePath string) error
	FileExists(filePath string) (bool, error)
}

type LocalFileStorage struct {
	uploadPath string
}

func NewLocalFileStorage(uploadPath string) *LocalFileStorage {
	return &LocalFileStorage{uploadPath: uploadPath}
}

// UploadFile handles multipart file uploads (existing method)
func (s *LocalFileStorage) UploadFile(file multipart.File, fileName string) (string, error) {
	filePath := filepath.Join(s.uploadPath, fileName)

	if _, err := os.Stat(s.uploadPath); os.IsNotExist(err) {
		if err := os.MkdirAll(s.uploadPath, 0755); err != nil {
			return "", fmt.Errorf("failed to create upload directory: %w", err)
		}
	}

	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		// Clean up on error
		os.Remove(filePath)
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	return filePath, nil
}

// UploadFileFromReader handles file uploads from any io.Reader (new method)
func (s *LocalFileStorage) UploadFileFromReader(src io.Reader, fileName string) (string, error) {
	filePath := filepath.Join(s.uploadPath, fileName)

	// Ensure upload directory exists
	if _, err := os.Stat(s.uploadPath); os.IsNotExist(err) {
		if err := os.MkdirAll(s.uploadPath, 0755); err != nil {
			return "", fmt.Errorf("failed to create upload directory: %w", err)
		}
	}

	// Create the file
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy the content
	if _, err := io.Copy(dst, src); err != nil {
		// Clean up on error
		os.Remove(filePath)
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	return filePath, nil
}

// DownloadFile retrieves a file for reading
func (s *LocalFileStorage) DownloadFile(filePath string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.uploadPath, filePath)
	
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	
	return file, nil
}

// DeleteFile removes a file from storage
func (s *LocalFileStorage) DeleteFile(filePath string) error {
	fullPath := filepath.Join(s.uploadPath, filePath)
	
	// Check if file exists first
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to delete
	}
	
	err := os.Remove(fullPath)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	
	return nil
}

// FileExists checks if a file exists in storage
func (s *LocalFileStorage) FileExists(filePath string) (bool, error) {
	fullPath := filepath.Join(s.uploadPath, filePath)
	
	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}
	
	return true, nil
}