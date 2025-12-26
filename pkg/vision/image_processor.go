package vision

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw" // For high-quality resizing
)

// ImageProcessor provides image preprocessing capabilities.
type ImageProcessor struct {
	maxWidth  int
	maxHeight int
	quality   int // JPEG quality (1-100)
}

// ImageProcessorConfig configures the image processor.
type ImageProcessorConfig struct {
	MaxWidth  int `json:"max_width"`   // Maximum width for preprocessing
	MaxHeight int `json:"max_height"`  // Maximum height for preprocessing
	Quality   int `json:"quality"`     // JPEG quality (1-100)
}

// DefaultImageProcessorConfig returns default configuration.
func DefaultImageProcessorConfig() *ImageProcessorConfig {
	return &ImageProcessorConfig{
		MaxWidth:  2048,
		MaxHeight: 2048,
		Quality:   85,
	}
}

// NewImageProcessor creates a new image processor.
func NewImageProcessor(cfg *ImageProcessorConfig) *ImageProcessor {
	if cfg == nil {
		cfg = DefaultImageProcessorConfig()
	}
	return &ImageProcessor{
		maxWidth:  cfg.MaxWidth,
		maxHeight: cfg.MaxHeight,
		quality:   cfg.Quality,
	}
}

// ProcessedImage contains the processed image data and metadata.
type ProcessedImage struct {
	Data      []byte `json:"data"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Format    string `json:"format"`
	OriginalWidth  int `json:"original_width"`
	OriginalHeight int `json:"original_height"`
	Resized   bool   `json:"resized"`
}

// Process reads and processes an image file.
func (p *ImageProcessor) Process(path string) (*ProcessedImage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	return p.ProcessReader(file, filepath.Ext(path))
}

// ProcessReader processes an image from a reader.
func (p *ImageProcessor) ProcessReader(reader io.Reader, format string) (*ProcessedImage, error) {
	// Read all data first (needed for re-reading)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	// Decode image
	img, imgFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	result := &ProcessedImage{
		OriginalWidth:  origWidth,
		OriginalHeight: origHeight,
		Format:         imgFormat,
	}

	// Check if resizing is needed
	if origWidth > p.maxWidth || origHeight > p.maxHeight {
		img = p.resize(img, p.maxWidth, p.maxHeight)
		bounds = img.Bounds()
		result.Resized = true
	}

	result.Width = bounds.Dx()
	result.Height = bounds.Dy()

	// Re-encode the image
	var buf bytes.Buffer
	switch strings.ToLower(imgFormat) {
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: p.quality})
		result.Format = "jpeg"
	case "png":
		err = png.Encode(&buf, img)
		result.Format = "png"
	default:
		// Default to PNG for other formats
		err = png.Encode(&buf, img)
		result.Format = "png"
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	result.Data = buf.Bytes()
	return result, nil
}

// resize resizes an image while maintaining aspect ratio.
func (p *ImageProcessor) resize(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate new dimensions
	ratio := float64(width) / float64(height)
	newWidth := maxWidth
	newHeight := int(float64(maxWidth) / ratio)

	if newHeight > maxHeight {
		newHeight = maxHeight
		newWidth = int(float64(maxHeight) * ratio)
	}

	// Create new image
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// Use high-quality resizing
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, xdraw.Over, nil)

	return dst
}

// ResizeToFit resizes an image to fit within the specified dimensions.
func (p *ImageProcessor) ResizeToFit(data []byte, maxWidth, maxHeight int) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	resized := p.resize(img, maxWidth, maxHeight)

	var buf bytes.Buffer
	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, resized, &jpeg.Options{Quality: p.quality})
	default:
		err = png.Encode(&buf, resized)
	}

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ConvertToJPEG converts an image to JPEG format.
func (p *ImageProcessor) ConvertToJPEG(data []byte, quality int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	if quality <= 0 {
		quality = p.quality
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ConvertToPNG converts an image to PNG format.
func (p *ImageProcessor) ConvertToPNG(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// CropToAspectRatio crops an image to the specified aspect ratio.
func (p *ImageProcessor) CropToAspectRatio(data []byte, aspectWidth, aspectHeight int) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	targetRatio := float64(aspectWidth) / float64(aspectHeight)
	currentRatio := float64(width) / float64(height)

	var cropRect image.Rectangle
	if currentRatio > targetRatio {
		// Image is wider, crop width
		newWidth := int(float64(height) * targetRatio)
		offset := (width - newWidth) / 2
		cropRect = image.Rect(offset, 0, offset+newWidth, height)
	} else {
		// Image is taller, crop height
		newHeight := int(float64(width) / targetRatio)
		offset := (height - newHeight) / 2
		cropRect = image.Rect(0, offset, width, offset+newHeight)
	}

	// Create cropped image
	dst := image.NewRGBA(image.Rect(0, 0, cropRect.Dx(), cropRect.Dy()))
	draw.Draw(dst, dst.Bounds(), img, cropRect.Min, draw.Src)

	var buf bytes.Buffer
	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: p.quality})
	default:
		err = png.Encode(&buf, dst)
	}

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// GetDimensions returns the dimensions of an image.
func (p *ImageProcessor) GetDimensions(data []byte) (width, height int, err error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// AddBorder adds a border to an image.
func (p *ImageProcessor) AddBorder(data []byte, borderWidth int, borderColor color.Color) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	newWidth := bounds.Dx() + 2*borderWidth
	newHeight := bounds.Dy() + 2*borderWidth

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// Fill with border color
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			dst.Set(x, y, borderColor)
		}
	}

	// Draw original image in center
	draw.Draw(dst, image.Rect(borderWidth, borderWidth, borderWidth+bounds.Dx(), borderWidth+bounds.Dy()),
		img, bounds.Min, draw.Over)

	var buf bytes.Buffer
	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: p.quality})
	default:
		err = png.Encode(&buf, dst)
	}

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ValidateImage checks if data is a valid image.
func (p *ImageProcessor) ValidateImage(data []byte) error {
	_, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid image data: %w", err)
	}
	return nil
}

// GetFormat returns the format of an image.
func (p *ImageProcessor) GetFormat(data []byte) (string, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	return format, nil
}
