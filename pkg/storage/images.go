package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ImageStorage provides file system operations for images.
type ImageStorage struct {
	basePath string
}

// ImageStorageConfig configures the image storage.
type ImageStorageConfig struct {
	BasePath string `json:"base_path"`
}

// DefaultImageStorageConfig returns default image storage configuration.
func DefaultImageStorageConfig() *ImageStorageConfig {
	return &ImageStorageConfig{
		BasePath: "./storage",
	}
}

// NewImageStorage creates a new image storage instance.
func NewImageStorage(cfg *ImageStorageConfig) (*ImageStorage, error) {
	if cfg == nil {
		cfg = DefaultImageStorageConfig()
	}

	// Create directory structure
	dirs := []string{
		filepath.Join(cfg.BasePath, "reference"),
		filepath.Join(cfg.BasePath, "generated"),
		filepath.Join(cfg.BasePath, "cache"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return &ImageStorage{
		basePath: cfg.BasePath,
	}, nil
}

// ImageInfo contains metadata about an image.
type ImageInfo struct {
	Path         string    `json:"path"`
	Filename     string    `json:"filename"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	Size         int64     `json:"size"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Checksum     string    `json:"checksum"`
	CreatedAt    time.Time `json:"created_at"`
}

// UploadReference uploads a reference image for a job.
func (s *ImageStorage) UploadReference(jobID string, file multipart.File, header *multipart.FileHeader) (*ImageInfo, error) {
	return s.uploadImage("reference", jobID, file, header)
}

// UploadGenerated saves a generated image for a job.
func (s *ImageStorage) UploadGenerated(jobID string, data []byte, filename string) (*ImageInfo, error) {
	if filename == "" {
		filename = fmt.Sprintf("%s.png", uuid.New().String())
	}

	dir := filepath.Join(s.basePath, "generated", jobID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return s.getImageInfo(filePath, filename)
}

// uploadImage handles the common upload logic.
func (s *ImageStorage) uploadImage(category, jobID string, file multipart.File, header *multipart.FileHeader) (*ImageInfo, error) {
	// Validate file type
	mimeType, err := s.detectMimeType(file)
	if err != nil {
		return nil, fmt.Errorf("failed to detect file type: %w", err)
	}
	if !s.isValidImageType(mimeType) {
		return nil, fmt.Errorf("unsupported image type: %s", mimeType)
	}

	// Reset file position after reading for mime detection
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	// Create job directory
	dir := filepath.Join(s.basePath, category, jobID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = s.mimeToExtension(mimeType)
	}
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	filePath := filepath.Join(dir, filename)

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy file content and calculate checksum
	hash := sha256.New()
	writer := io.MultiWriter(dst, hash)

	size, err := io.Copy(writer, file)
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	checksum := hex.EncodeToString(hash.Sum(nil))

	// Get image dimensions
	width, height, err := s.getImageDimensions(filePath)
	if err != nil {
		// Non-fatal: dimensions might not be available for all formats
		width, height = 0, 0
	}

	return &ImageInfo{
		Path:         filePath,
		Filename:     filename,
		OriginalName: header.Filename,
		MimeType:     mimeType,
		Size:         size,
		Width:        width,
		Height:       height,
		Checksum:     checksum,
		CreatedAt:    time.Now(),
	}, nil
}

// GetReference retrieves a reference image.
func (s *ImageStorage) GetReference(jobID, filename string) (string, error) {
	path := filepath.Join(s.basePath, "reference", jobID, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("reference image not found: %s", filename)
	}
	return path, nil
}

// GetGenerated retrieves a generated image.
func (s *ImageStorage) GetGenerated(jobID, filename string) (string, error) {
	path := filepath.Join(s.basePath, "generated", jobID, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("generated image not found: %s", filename)
	}
	return path, nil
}

// ListReferenceImages lists all reference images for a job.
func (s *ImageStorage) ListReferenceImages(jobID string) ([]*ImageInfo, error) {
	return s.listImages("reference", jobID)
}

// ListGeneratedImages lists all generated images for a job.
func (s *ImageStorage) ListGeneratedImages(jobID string) ([]*ImageInfo, error) {
	return s.listImages("generated", jobID)
}

// listImages lists images in a category for a job.
func (s *ImageStorage) listImages(category, jobID string) ([]*ImageInfo, error) {
	dir := filepath.Join(s.basePath, category, jobID)

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []*ImageInfo{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var images []*ImageInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !s.isImageFile(entry.Name()) {
			continue
		}

		info, err := s.getImageInfo(filepath.Join(dir, entry.Name()), entry.Name())
		if err != nil {
			continue // Skip files we can't read
		}
		images = append(images, info)
	}

	return images, nil
}

// DeleteJobImages deletes all images for a job.
func (s *ImageStorage) DeleteJobImages(jobID string) error {
	categories := []string{"reference", "generated"}
	for _, category := range categories {
		dir := filepath.Join(s.basePath, category, jobID)
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete %s images: %w", category, err)
		}
	}
	return nil
}

// CacheModel stores a model file in cache.
func (s *ImageStorage) CacheModel(modelName string, data []byte) (string, error) {
	dir := filepath.Join(s.basePath, "cache", modelName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Store with hash-based filename
	hash := sha256.Sum256(data)
	filename := hex.EncodeToString(hash[:]) + ".bin"
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write cache file: %w", err)
	}

	return path, nil
}

// GetCachePath returns the cache directory for a model.
func (s *ImageStorage) GetCachePath(modelName string) string {
	return filepath.Join(s.basePath, "cache", modelName)
}

// Helper methods

func (s *ImageStorage) detectMimeType(file multipart.File) (string, error) {
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}
	return http.DetectContentType(buffer[:n]), nil
}

func (s *ImageStorage) isValidImageType(mimeType string) bool {
	validTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
		"image/bmp":  true,
		"image/tiff": true,
	}
	return validTypes[mimeType]
}

func (s *ImageStorage) mimeToExtension(mimeType string) string {
	extensions := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
		"image/webp": ".webp",
		"image/bmp":  ".bmp",
		"image/tiff": ".tiff",
	}
	if ext, ok := extensions[mimeType]; ok {
		return ext
	}
	return ".bin"
}

func (s *ImageStorage) isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true,
		".gif": true, ".webp": true, ".bmp": true, ".tiff": true,
	}
	return imageExts[ext]
}

func (s *ImageStorage) getImageDimensions(path string) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}

	return config.Width, config.Height, nil
}

func (s *ImageStorage) getImageInfo(path, filename string) (*ImageInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Calculate checksum
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))

	// Reset for dimension reading
	file.Seek(0, io.SeekStart)
	width, height, _ := s.getImageDimensions(path)

	// Detect MIME type
	file.Seek(0, io.SeekStart)
	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	mimeType := http.DetectContentType(buffer[:n])

	return &ImageInfo{
		Path:      path,
		Filename:  filename,
		MimeType:  mimeType,
		Size:      stat.Size(),
		Width:     width,
		Height:    height,
		Checksum:  checksum,
		CreatedAt: stat.ModTime(),
	}, nil
}

// PreprocessImage preprocesses an image for vision model input.
// Returns the preprocessed image data and metadata.
func (s *ImageStorage) PreprocessImage(path string, maxWidth, maxHeight int) ([]byte, *ImageInfo, error) {
	// For now, just read the original file
	// TODO: Add actual preprocessing (resize, format conversion) when needed
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read image: %w", err)
	}

	info, err := s.getImageInfo(path, filepath.Base(path))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get image info: %w", err)
	}

	return data, info, nil
}

// SaveFromURL downloads an image from URL and saves it.
func (s *ImageStorage) SaveFromURL(category, jobID, url string) (*ImageInfo, error) {
	// TODO: Implement URL download when needed
	return nil, fmt.Errorf("URL download not implemented")
}
