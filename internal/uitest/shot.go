package uitest

import (
	"bufio"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// shotPath returns the file a screenshot named name is written to. The name
// is a slash-separated path relative to outDir; ".png" is appended unless it
// already ends that way. Names that would leave outDir are rejected.
func shotPath(outDir, name string) (string, error) {
	rel := filepath.FromSlash(strings.TrimSpace(name))
	if rel == "" {
		return "", fmt.Errorf("screenshot name is empty")
	}
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("screenshot name %q must be a relative path inside the output directory", name)
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".png") {
		rel += ".png"
	}
	return filepath.Join(outDir, rel), nil
}

// opaqueImage wraps pixels read back from the screen (premultiplied RGBA,
// w×h) as an image, flattened onto black: whatever the draw callback left
// transparent appears black, exactly as it does in the window. pix is
// modified in place.
func opaqueImage(pix []byte, w, h int) (*image.RGBA, error) {
	if w <= 0 || h <= 0 || len(pix) != 4*w*h {
		return nil, fmt.Errorf("screenshot buffer has %d bytes, want %d×%d×4", len(pix), w, h)
	}
	for i := 3; i < len(pix); i += 4 {
		pix[i] = 0xff
	}
	return &image.RGBA{Pix: pix, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}, nil
}

// writeShot saves the screen pixels as a PNG under outDir and returns the path
// of the file written.
func writeShot(outDir, name string, pix []byte, w, h int) (string, error) {
	path, err := shotPath(outDir, name)
	if err != nil {
		return "", fmt.Errorf("uitest: %w", err)
	}
	img, err := opaqueImage(pix, w, h)
	if err != nil {
		return "", fmt.Errorf("uitest: %s: %w", name, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("uitest: saving screenshot: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("uitest: saving screenshot: %w", err)
	}
	bw := bufio.NewWriter(f)
	// Screenshots are large and written often; favour speed over size.
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	err = enc.Encode(bw, img)
	if err == nil {
		err = bw.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("uitest: saving screenshot %s: %w", path, err)
	}
	return path, nil
}
