package image

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"regexp"
	"strings"

	_ "image/gif"  // Register GIF decoder
	_ "image/jpeg" // Register JPEG decoder
	_ "image/png"  // Register PNG decoder
)

// DataURI represents a parsed data URI
type DataURI struct {
	MediaType string
	Data      string
}

// ParseDataUri parses a data URI and returns its components
func ParseDataUri(dataURI string) *DataURI {
	// Match data:[<mediatype>][;base64],<data>
	re := regexp.MustCompile(`^data:([^;,]+)?(?:;base64)?,(.+)$`)
	matches := re.FindStringSubmatch(dataURI)
	if len(matches) < 3 {
		return nil
	}
	return &DataURI{
		MediaType: matches[1],
		Data:      matches[2],
	}
}

// Image processing constants
const (
	// Target size for images (5MB in base64 is ~3.75MB raw)
	ImageTargetRawSize = 5 * 1024 * 1024 // 5 MB
	// Maximum dimensions
	ImageMaxWidth  = 8000
	ImageMaxHeight = 8000
	// JPEG quality levels for compression attempts
	DefaultJPEGQuality = 80
)

// ImageDimensions holds image dimension information
type ImageDimensions struct {
	OriginalWidth  int
	OriginalHeight int
	DisplayWidth   int
	DisplayHeight  int
}

// ResizeResult holds the result of image resizing
type ResizeResult struct {
	Buffer     []byte
	MediaType  string
	Dimensions *ImageDimensions
}

// ImageProcessor handles image processing operations
type ImageProcessor struct {
	targetSize  int64
	maxWidth    int
	maxHeight   int
	jpegQuality int
}

// CreateImageProcessor creates a new image processor with default settings
func CreateImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		targetSize:  ImageTargetRawSize,
		maxWidth:    ImageMaxWidth,
		maxHeight:   ImageMaxHeight,
		jpegQuality: DefaultJPEGQuality,
	}
}

// ProcessDataURI processes an image data URI
// Returns the processed data URI or the original if no processing needed
func (ip *ImageProcessor) ProcessDataURI(dataURI string) (string, error) {
	parsed := ParseDataUri(dataURI)
	if parsed == nil {
		return "", fmt.Errorf("invalid data URI format")
	}

	// Decode base64
	rawData, err := base64.StdEncoding.DecodeString(parsed.Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Process the raw image
	result, err := ip.ProcessImage(rawData, parsed.MediaType)
	if err != nil {
		return "", err
	}

	// Encode back to base64
	encodedData := base64.StdEncoding.EncodeToString(result.Buffer)

	// Build new data URI
	mediaType := result.MediaType
	if mediaType == "jpg" {
		mediaType = "jpeg"
	}

	return fmt.Sprintf("data:image/%s;base64,%s", mediaType, encodedData), nil
}

// ProcessImage processes raw image bytes
func (ip *ImageProcessor) ProcessImage(data []byte, mediaType string) (*ResizeResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("image is empty (0 bytes)")
	}

	// Decode image
	img, format, err := image.Decode(strings.NewReader(string(data)))
	if err != nil {
		// If we can't decode, return as-is if within size limit
		if int64(len(data)) <= ip.targetSize {
			return &ResizeResult{
				Buffer:    data,
				MediaType: mediaType,
			}, nil
		}
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Normalize format
	if format == "jpg" {
		format = "jpeg"
	}

	// Get dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Check if original is acceptable
	originalSize := int64(len(data))
	if originalSize <= ip.targetSize && width <= ip.maxWidth && height <= ip.maxHeight {
		return &ResizeResult{
			Buffer:    data,
			MediaType: format,
			Dimensions: &ImageDimensions{
				OriginalWidth:  width,
				OriginalHeight: height,
				DisplayWidth:   width,
				DisplayHeight:  height,
			},
		}, nil
	}

	// Need to resize or compress
	result, err := ip.compressAndResize(img, format, width, height)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// compressAndResize compresses and/or resizes an image
func (ip *ImageProcessor) compressAndResize(img image.Image, format string, origWidth, origHeight int) (*ResizeResult, error) {
	// First, try compression without resize
	for quality := ip.jpegQuality; quality >= 20; quality -= 20 {
		result, err := ip.encodeAsJPEG(img, quality)
		if err != nil {
			continue
		}

		if int64(len(result)) <= ip.targetSize {
			return &ResizeResult{
				Buffer:    result,
				MediaType: "jpeg",
				Dimensions: &ImageDimensions{
					OriginalWidth:  origWidth,
					OriginalHeight: origHeight,
					DisplayWidth:   origWidth,
					DisplayHeight:  origHeight,
				},
			}, nil
		}
	}

	// Need to resize
	// Calculate new dimensions maintaining aspect ratio
	newWidth, newHeight := ip.calculateNewDimensions(origWidth, origHeight)

	// Resize image (using simple nearest-neighbor for now)
	resized := ip.resizeImage(img, newWidth, newHeight)

	// Encode resized image
	for quality := ip.jpegQuality; quality >= 20; quality -= 20 {
		result, err := ip.encodeAsJPEG(resized, quality)
		if err != nil {
			continue
		}

		if int64(len(result)) <= ip.targetSize {
			return &ResizeResult{
				Buffer:    result,
				MediaType: "jpeg",
				Dimensions: &ImageDimensions{
					OriginalWidth:  origWidth,
					OriginalHeight: origHeight,
					DisplayWidth:   newWidth,
					DisplayHeight:  newHeight,
				},
			}, nil
		}
	}

	// Return the best we can do
	result, err := ip.encodeAsJPEG(resized, 20)
	if err != nil {
		return nil, err
	}

	return &ResizeResult{
		Buffer:    result,
		MediaType: "jpeg",
		Dimensions: &ImageDimensions{
			OriginalWidth:  origWidth,
			OriginalHeight: origHeight,
			DisplayWidth:   newWidth,
			DisplayHeight:  newHeight,
		},
	}, nil
}

// calculateNewDimensions calculates new dimensions maintaining aspect ratio
func (ip *ImageProcessor) calculateNewDimensions(origWidth, origHeight int) (int, int) {
	if origWidth <= ip.maxWidth && origHeight <= ip.maxHeight {
		return origWidth, origHeight
	}

	ratio := float64(origWidth) / float64(origHeight)

	var newWidth, newHeight int
	if origWidth > origHeight {
		newWidth = ip.maxWidth
		newHeight = int(float64(newWidth) / ratio)
	} else {
		newHeight = ip.maxHeight
		newWidth = int(float64(newHeight) * ratio)
	}

	return newWidth, newHeight
}

// resizeImage resizes an image using simple nearest-neighbor interpolation
func (ip *ImageProcessor) resizeImage(img image.Image, newWidth, newHeight int) image.Image {
	// Create new RGBA image
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// Simple nearest-neighbor scaling
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			// Map destination coordinates to source
			srcX := (x * img.Bounds().Dx()) / newWidth
			srcY := (y * img.Bounds().Dy()) / newHeight
			dst.Set(x, y, img.At(srcX+img.Bounds().Min.X, srcY+img.Bounds().Min.Y))
		}
	}

	return dst
}

// encodeAsJPEG encodes an image as JPEG with the given quality
func (ip *ImageProcessor) encodeAsJPEG(img image.Image, quality int) ([]byte, error) {
	// Create a temporary file
	f, err := os.CreateTemp("", "image-*.jpg")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	options := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(f, img, options); err != nil {
		return nil, err
	}

	// Read back
	f.Seek(0, 0)
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// encodeAsPNG encodes an image as PNG
func (ip *ImageProcessor) encodeAsPNG(img image.Image) ([]byte, error) {
	f, err := os.CreateTemp("", "image-*.png")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return nil, err
	}

	f.Seek(0, 0)
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Global image processor instance
var globalImageProcessor = CreateImageProcessor()

// ProcessImageDataURI processes an image data URI using the global processor
func ProcessImageDataURI(dataURI string) (string, error) {
	return globalImageProcessor.ProcessDataURI(dataURI)
}

// ProcessImageBytes processes raw image bytes using the global processor
func ProcessImageBytes(data []byte, mediaType string) (*ResizeResult, error) {
	return globalImageProcessor.ProcessImage(data, mediaType)
}

// dataURIRegex matches data URIs for quick detection
var dataURIRegex = regexp.MustCompile(`^data:image/([a-zA-Z0-9.+_-]+);base64,(.+)$`)

// ExtractImageFromDataURI extracts image data from a data URI
func ExtractImageFromDataURI(dataURI string) (mediaType string, data []byte, err error) {
	matches := dataURIRegex.FindStringSubmatch(dataURI)
	if matches == nil {
		return "", nil, fmt.Errorf("invalid image data URI")
	}

	mediaType = matches[1]
	encodedData := matches[2]

	data, err = base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	return mediaType, data, nil
}

// BuildImageDataURL builds an image data URI from raw bytes
func BuildImageDataURL(mediaType string, data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:image/%s;base64,%s", mediaType, encoded)
}