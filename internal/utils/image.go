package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"strings"

	"github.com/disintegration/imaging"
)

// Image size constants from API limits
const (
	APIImageMaxBase64Size = 5 * 1024 * 1024 // 5 MB base64
	ImageMaxWidth         = 2000
	ImageMaxHeight        = 2000
	ImageTargetRawSize    = 5 * 1024 * 1024 // 5 MB
)

// ImageDimensions contains image dimension information.
type ImageDimensions struct {
	OriginalWidth  int
	OriginalHeight int
	DisplayWidth   int
	DisplayHeight  int
}

// ResizeResult contains the result of image resizing.
type ResizeResult struct {
	Buffer     []byte
	MediaType  string
	Dimensions *ImageDimensions
}

// DetectImageFormatFromBuffer detects image format from magic bytes.
func DetectImageFormatFromBuffer(data []byte) string {
	if len(data) < 4 {
		return "image/png" // default
	}

	// PNG signature: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}

	// JPEG signature: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}

	// GIF signature: GIF87a or GIF89a
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}

	// WebP signature: RIFF....WEBP
	if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		if len(data) >= 12 && data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "image/webp"
		}
	}

	return "image/png" // default
}

// DetectImageFormatFromBase64 detects image format from base64 data.
func DetectImageFormatFromBase64(base64Data string) string {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "image/png"
	}
	return DetectImageFormatFromBuffer(data)
}

// GetImageDimensions extracts dimensions from an image.
func GetImageDimensions(data []byte) (int, int, error) {
	reader := bytes.NewReader(data)

	img, _, err := image.Decode(reader)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy(), nil
}

// ResizeImage resizes an image to fit within max dimensions while preserving aspect ratio.
// Uses Lanczos interpolation for high-quality downsampling (matching TS sharp library default).
// Ported from src/utils/imageResizer.ts
func ResizeImage(data []byte, maxWidth, maxHeight int, format string) ([]byte, error) {
	reader := bytes.NewReader(data)

	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Check if resize is needed
	if width <= maxWidth && height <= maxHeight {
		return data, nil
	}

	// Calculate new dimensions preserving aspect ratio
	newWidth, newHeight := calculateNewDimensions(width, height, maxWidth, maxHeight)

	// Use Lanczos interpolation for high-quality downsampling
	// Ported from src/utils/imageResizer.ts: uses sharp's default kernel (lanczos3)
	// imaging.Lanczos uses a 3-lobed Lanczos filter, equivalent to lanczos3 in sharp
	resized := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)

	// Encode to appropriate format
	var buf bytes.Buffer
	switch format {
	case "png":
		err = png.Encode(&buf, resized)
	case "gif":
		err = gif.Encode(&buf, resized, nil)
	default:
		err = jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 80})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	return buf.Bytes(), nil
}

// calculateNewDimensions calculates new dimensions preserving aspect ratio.
// Ported from src/utils/imageResizer.ts
func calculateNewDimensions(width, height, maxWidth, maxHeight int) (int, int) {
	if width <= maxWidth && height <= maxHeight {
		return width, height
	}

	ratio := float64(width) / float64(height)

	if width > maxWidth {
		width = maxWidth
		height = int(float64(width) / ratio)
	}

	if height > maxHeight {
		height = maxHeight
		width = int(float64(height) * ratio)
	}

	return width, height
}

// MaybeResizeAndCompressImage resizes and compresses an image to fit API limits.
// Ported from src/utils/imageResizer.ts:maybeResizeAndDownsampleImageBuffer
func MaybeResizeAndCompressImage(data []byte, ext string) (*ResizeResult, error) {
	if len(data) == 0 {
		return nil, &ImageResizeError{Message: "Image file is empty (0 bytes)"}
	}

	originalSize := len(data)
	detectedFormat := DetectImageFormatFromBuffer(data)
	mediaType := strings.TrimPrefix(detectedFormat, "image/")

	// Normalize jpg to jpeg
	if mediaType == "jpg" {
		mediaType = "jpeg"
	}

	// Try to get dimensions
	var originalWidth, originalHeight int
	var hasDimensions bool

	reader := bytes.NewReader(data)
	img, decodedFormat, err := image.Decode(reader)
	if err == nil {
		bounds := img.Bounds()
		originalWidth = bounds.Dx()
		originalHeight = bounds.Dy()
		hasDimensions = true
		mediaType = decodedFormat
		if mediaType == "jpg" {
			mediaType = "jpeg"
		}
	}

	// Check if original is within limits
	base64Size := base64StdEncodedSize(originalSize)
	withinSizeLimit := base64Size <= APIImageMaxBase64Size
	withinDimLimit := !hasDimensions || (originalWidth <= ImageMaxWidth && originalHeight <= ImageMaxHeight)

	if withinSizeLimit && withinDimLimit {
		result := &ResizeResult{
			Buffer:    data,
			MediaType: mediaType,
		}
		if hasDimensions {
			result.Dimensions = &ImageDimensions{
				OriginalWidth:  originalWidth,
				OriginalHeight: originalHeight,
				DisplayWidth:   originalWidth,
				DisplayHeight:  originalHeight,
			}
		}
		return result, nil
	}

	// Need to resize or compress
	if hasDimensions {
		// Resize if dimensions exceed limits
		if originalWidth > ImageMaxWidth || originalHeight > ImageMaxHeight {
			newWidth, newHeight := calculateNewDimensions(originalWidth, originalHeight, ImageMaxWidth, ImageMaxHeight)
			resized, err := ResizeImage(data, newWidth, newHeight, mediaType)
			if err == nil {
				data = resized
				originalWidth = newWidth
				originalHeight = newHeight
			}
		}

		// Try compression at various quality levels
		for _, quality := range []int{80, 60, 40, 20} {
			compressed, err := compressJPEG(data, quality)
			if err != nil {
				continue
			}

			base64Size = base64StdEncodedSize(len(compressed))
			if base64Size <= APIImageMaxBase64Size {
				return &ResizeResult{
					Buffer:    compressed,
					MediaType: "jpeg",
					Dimensions: &ImageDimensions{
						OriginalWidth:  originalWidth,
						OriginalHeight: originalHeight,
						DisplayWidth:   originalWidth,
						DisplayHeight:  originalHeight,
					},
				}, nil
			}
		}

		// Last resort: aggressive compression
		smallWidth := min(originalWidth, 1000)
		smallHeight := (originalHeight * smallWidth) / max(originalWidth, 1)
		resized, err := ResizeImage(data, smallWidth, smallHeight, "jpeg")
		if err != nil {
			return nil, &ImageResizeError{
				Message: fmt.Sprintf("Unable to resize image (%s). Please use a smaller image.",
					FormatFileSize(int64(originalSize))),
			}
		}

		return &ResizeResult{
			Buffer:    resized,
			MediaType: "jpeg",
			Dimensions: &ImageDimensions{
				OriginalWidth:  originalWidth,
				OriginalHeight: originalHeight,
				DisplayWidth:   smallWidth,
				DisplayHeight:  smallHeight,
			},
		}, nil
	}

	// No dimensions available, just check size
	if base64Size <= APIImageMaxBase64Size {
		return &ResizeResult{
			Buffer:    data,
			MediaType: mediaType,
		}, nil
	}

	// Try to compress as JPEG
	compressed, err := compressJPEG(data, 60)
	if err == nil && base64StdEncodedSize(len(compressed)) <= APIImageMaxBase64Size {
		return &ResizeResult{
			Buffer:    compressed,
			MediaType: "jpeg",
		}, nil
	}

	return nil, &ImageResizeError{
		Message: fmt.Sprintf("Unable to resize image (%s). The image exceeds the 5MB API limit.",
			FormatFileSize(int64(originalSize))),
	}
}

// compressJPEG compresses an image as JPEG at the given quality.
func compressJPEG(data []byte, quality int) ([]byte, error) {
	reader := bytes.NewReader(data)
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// base64StdEncodedSize calculates the size of base64-encoded data.
func base64StdEncodedSize(n int) int {
	// Base64 encodes 3 bytes into 4 characters
	return (n + 2) / 3 * 4
}

// ReadImageFile reads an image file and returns its data.
func ReadImageFile(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	mediaType := DetectImageFormatFromBuffer(data)
	return data, mediaType, nil
}

// EncodeImageToBase64 encodes image data to base64.
func EncodeImageToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeImageFromBase64 decodes base64 image data.
func DecodeImageFromBase64(base64Data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(base64Data)
}

// CreateImageMetadataText creates a text description of image metadata.
func CreateImageMetadataText(dims *ImageDimensions, sourcePath string) string {
	if dims == nil {
		if sourcePath != "" {
			return fmt.Sprintf("[Image source: %s]", sourcePath)
		}
		return ""
	}

	if dims.OriginalWidth <= 0 || dims.OriginalHeight <= 0 ||
		dims.DisplayWidth <= 0 || dims.DisplayHeight <= 0 {
		if sourcePath != "" {
			return fmt.Sprintf("[Image source: %s]", sourcePath)
		}
		return ""
	}

	wasResized := dims.OriginalWidth != dims.DisplayWidth || dims.OriginalHeight != dims.DisplayHeight

	if !wasResized && sourcePath == "" {
		return ""
	}

	var parts []string
	if sourcePath != "" {
		parts = append(parts, fmt.Sprintf("source: %s", sourcePath))
	}

	if wasResized {
		scaleFactor := float64(dims.OriginalWidth) / float64(dims.DisplayWidth)
		parts = append(parts, fmt.Sprintf(
			"original %dx%d, displayed at %dx%d. Multiply coordinates by %.2f to map to original image.",
			dims.OriginalWidth, dims.OriginalHeight,
			dims.DisplayWidth, dims.DisplayHeight,
			scaleFactor,
		))
	}

	return fmt.Sprintf("[Image: %s]", strings.Join(parts, ", "))
}

// PNG dimension extraction from header (for dimension checks without full decode)
func extractPNGDimensions(data []byte) (int, int, bool) {
	// PNG header is 8 bytes, IHDR starts at byte 8
	// IHDR: width (4 bytes), height (4 bytes)
	if len(data) < 24 {
		return 0, 0, false
	}

	// Check PNG signature
	if data[0] != 0x89 || data[1] != 0x50 || data[2] != 0x4E || data[3] != 0x47 {
		return 0, 0, false
	}

	// Extract width and height from IHDR (big-endian)
	width := int(binary.BigEndian.Uint32(data[16:20]))
	height := int(binary.BigEndian.Uint32(data[20:24]))

	return width, height, true
}

// ReadPNGDimensions reads PNG dimensions from file without fully decoding.
func ReadPNGDimensions(r io.Reader) (int, int, error) {
	header := make([]byte, 24)
	n, err := io.ReadFull(r, header)
	if err != nil || n < 24 {
		return 0, 0, fmt.Errorf("failed to read PNG header")
	}

	width, height, ok := extractPNGDimensions(header)
	if !ok {
		return 0, 0, fmt.Errorf("invalid PNG header")
	}

	return width, height, nil
}