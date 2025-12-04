package storage

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
)

// FileType represents allowed file types
type FileType string

const (
	FileTypeImage    FileType = "image"
	FileTypeDocument FileType = "document"
	FileTypeVideo    FileType = "video"
)

// FileValidator validates uploaded files
type FileValidator struct {
	MaxSize      int64           // in bytes
	AllowedTypes map[string]bool // allowed extensions
	FileType     FileType
}

// Magic numbers for file type detection
var magicNumbers = map[string][]byte{
	// Images
	".jpg":  {0xFF, 0xD8, 0xFF},
	".jpeg": {0xFF, 0xD8, 0xFF},
	".png":  {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
	".gif":  {0x47, 0x49, 0x46, 0x38},
	".webp": {0x52, 0x49, 0x46, 0x46}, // RIFF (WebP container)
	".bmp":  {0x42, 0x4D},

	// Documents
	".pdf":  {0x25, 0x50, 0x44, 0x46},
	".docx": {0x50, 0x4B, 0x03, 0x04}, // ZIP format (Office Open XML)
	".xlsx": {0x50, 0x4B, 0x03, 0x04},
	".zip":  {0x50, 0x4B, 0x03, 0x04},

	// Video
	".mp4": {0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70}, // ftyp
	".avi": {0x52, 0x49, 0x46, 0x46},                         // RIFF
	".mov": {0x00, 0x00, 0x00, 0x14, 0x66, 0x74, 0x79, 0x70},
}

// NewImageValidator creates a validator for image files
func NewImageValidator(maxSizeMB int) *FileValidator {
	return &FileValidator{
		MaxSize: int64(maxSizeMB * 1024 * 1024),
		AllowedTypes: map[string]bool{
			".jpg":  true,
			".jpeg": true,
			".png":  true,
			".gif":  true,
			".webp": true,
			".bmp":  true,
		},
		FileType: FileTypeImage,
	}
}

// NewDocumentValidator creates a validator for document files
func NewDocumentValidator(maxSizeMB int) *FileValidator {
	return &FileValidator{
		MaxSize: int64(maxSizeMB * 1024 * 1024),
		AllowedTypes: map[string]bool{
			".pdf":  true,
			".doc":  true,
			".docx": true,
			".xls":  true,
			".xlsx": true,
			".txt":  true,
		},
		FileType: FileTypeDocument,
	}
}

// ValidateFile validates an uploaded file
func (v *FileValidator) ValidateFile(file multipart.File, header *multipart.FileHeader) error {
	// Check file size
	if header.Size > v.MaxSize {
		return fmt.Errorf("file too large: max %d MB allowed", v.MaxSize/(1024*1024))
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !v.AllowedTypes[ext] {
		return fmt.Errorf("file type not allowed: %s", ext)
	}

	// Verify actual file content matches extension (magic number check)
	if err := v.verifyFileContent(file, ext); err != nil {
		return err
	}

	// Reset file pointer
	file.Seek(0, io.SeekStart)

	return nil
}

// verifyFileContent checks if the file content matches the expected type
func (v *FileValidator) verifyFileContent(file multipart.File, ext string) error {
	magic, exists := magicNumbers[ext]
	if !exists {
		// If we don't have a magic number for this type, skip content verification
		return nil
	}

	// Read the beginning of the file
	buffer := make([]byte, len(magic))
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	// Reset file pointer
	file.Seek(0, io.SeekStart)

	// Compare magic numbers
	if n < len(magic) {
		return fmt.Errorf("file too small to determine type")
	}

	// Special case for WebP (need more bytes)
	if ext == ".webp" {
		buffer2 := make([]byte, 12)
		file.Read(buffer2)
		file.Seek(0, io.SeekStart)

		// WebP: RIFF....WEBP
		if !bytes.Equal(buffer2[0:4], []byte{0x52, 0x49, 0x46, 0x46}) ||
			!bytes.Equal(buffer2[8:12], []byte{0x57, 0x45, 0x42, 0x50}) {
			return fmt.Errorf("file content does not match WebP format")
		}
		return nil
	}

	if !bytes.Equal(buffer[:len(magic)], magic) {
		return fmt.Errorf("file content does not match expected type %s (possible extension spoofing)", ext)
	}

	return nil
}

// SanitizeFilename removes potentially dangerous characters from filename
func SanitizeFilename(filename string) string {
	// Remove path traversal attempts
	filename = filepath.Base(filename)

	// Remove or replace dangerous characters
	replacer := strings.NewReplacer(
		"..", "",
		"/", "",
		"\\", "",
		":", "",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "",
		"\x00", "", // null byte
	)

	return replacer.Replace(filename)
}

// ValidateImageFile is a convenience function for validating image files
func ValidateImageFile(file multipart.File, header *multipart.FileHeader) error {
	validator := NewImageValidator(5) // 5MB max
	return validator.ValidateFile(file, header)
}

// ValidateDocumentFile is a convenience function for validating document files
func ValidateDocumentFile(file multipart.File, header *multipart.FileHeader) error {
	validator := NewDocumentValidator(10) // 10MB max
	return validator.ValidateFile(file, header)
}
