package utils

import (
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

type FileStorage interface {
	UploadFile(file multipart.File, fileName string) (string, error)
}

type LocalFileStorage struct {
	uploadPath string
}

func NewLocalFileStorage(uploadPath string) *LocalFileStorage {
	return &LocalFileStorage{uploadPath: uploadPath}
}

func (s *LocalFileStorage) UploadFile(file multipart.File, fileName string) (string, error) {
	// fileName already contains full unique name with extension
	filePath := filepath.Join(s.uploadPath, fileName)

	if _, err := os.Stat(s.uploadPath); os.IsNotExist(err) {
		if err := os.MkdirAll(s.uploadPath, 0755); err != nil {
			return "", err
		}
	}

	dst, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}

	return filePath, nil
}
