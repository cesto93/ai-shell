package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ai-shell/llm"
)

// readInputFile reads a text-based input file into a string. PDFs are
// extracted via pdftotext (poppler-utils).
func readInputFile(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", path, err)
		}
		return string(data), nil
	case ".pdf":
		return readPDF(path)
	default:
		return "", fmt.Errorf("unsupported file extension %q (supported: .txt, .md, .pdf)", ext)
	}
}

func readPDF(path string) (string, error) {
	out, err := exec.Command("pdftotext", path, "-").Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext failed (is poppler-utils installed?): %w", err)
	}
	return string(out), nil
}

func isImage(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp"
}

func encodeImage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	mimeType := "image/jpeg"
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	}

	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data)), nil
}

// buildCommandTextAndImages assembles the prompt text plus the files referenced
// by a command's arguments. Arguments that resolve to existing files are read
// into the message: images are base64-encoded (returned separately), text/PDF
// files are appended as text. Non-file arguments are appended to the prompt
// text. It returns the combined text and the list of encoded images.
func buildCommandTextAndImages(prompt string, args []string) (string, []string, error) {
	var textParts []string
	var images []string

	appendText := func(s string) {
		if s != "" {
			textParts = append(textParts, strings.TrimSpace(s))
		}
	}

	appendText(prompt)
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil || info.IsDir() {
			appendText(arg)
			continue
		}
		if isImage(arg) {
			encoded, err := encodeImage(arg)
			if err != nil {
				return "", nil, fmt.Errorf("failed to read image %s: %w", arg, err)
			}
			images = append(images, encoded)
			continue
		}
		text, err := readInputFile(arg)
		if err != nil {
			return "", nil, err
		}
		appendText(text)
	}

	return strings.Join(textParts, "\n\n"), images, nil
}

// buildCommandContent assembles a user message from a command prompt plus its
// arguments. Arguments that resolve to existing files are read into the
// message: images become image_url content parts, text/PDF files are appended
// as text. Non-file arguments are appended to the prompt text. It returns a
// plain string when there are no images and a []ContentPart otherwise.
func buildCommandContent(prompt string, args []string) (any, error) {
	text, images, err := buildCommandTextAndImages(prompt, args)
	if err != nil {
		return nil, err
	}

	if len(images) == 0 {
		return text, nil
	}

	parts := []llm.ContentPart{{Type: "text", Text: text}}
	for _, img := range images {
		parts = append(parts, llm.ContentPart{
			Type:     "image_url",
			ImageURL: &llm.ContentImage{URL: img},
		})
	}
	return parts, nil
}
