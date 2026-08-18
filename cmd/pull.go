package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"ai-shell/config"

	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull <repo> <filename>",
	Short: "Download a model from HuggingFace",
	Long: `Download a model file from a HuggingFace repository to the local models directory.

The repo is the HuggingFace repository path (e.g., unsloth/Qwen3.5-2B-GGUF).
The filename is the specific model file to download (e.g., Qwen3.5-2B-Q4_K_M.gguf).

The destination is chosen by the filename extension: .litertlm files are saved
to ~/.ai-shell/models/litertlm/ and registered as LiteRT-LM models; anything
else is saved to ~/.ai-shell/models/llamacpp/ as a GGUF model. The config is
updated to use this model.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPull(args[0], args[1])
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
}

func runPull(repo, filename string) error {
	provider := "llamacpp"
	ext := ".gguf"
	if strings.HasSuffix(strings.ToLower(filename), ".litertlm") {
		ext = ".litertlm"
		provider = "litertlm"
	}

	destDir, err := config.ModelsDir(provider)
	if err != nil {
		return fmt.Errorf("cannot determine models directory: %w", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create models directory: %w", err)
	}

	destPath := filepath.Join(destDir, filename)

	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("model file already exists at %s", destPath)
	}

	url := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repo, filename)
	fmt.Printf("Downloading %s ...\n", url)

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		os.Remove(destPath)
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(destPath)
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	contentLength := resp.ContentLength
	buf := make([]byte, 32*1024)
	var written int64
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				os.Remove(destPath)
				return fmt.Errorf("write error: %w", werr)
			}
			written += int64(n)
			if contentLength > 0 {
				pct := float64(written) / float64(contentLength) * 100
				fmt.Fprintf(os.Stderr, "\rDownloading... %.1f%% (%s/%s)", pct, config.FormatFileSize(written), config.FormatFileSize(contentLength))
			} else {
				fmt.Fprintf(os.Stderr, "\rDownloading... %s", config.FormatFileSize(written))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(destPath)
			return fmt.Errorf("download error: %w", err)
		}
	}
	fmt.Fprintf(os.Stderr, "\n")

	modelName := strings.TrimSuffix(filename, ext)
	if err := config.SaveModelWithProvider(modelName, provider); err != nil {
		fmt.Printf("Warning: failed to update config: %v\n", err)
	}

	fmt.Printf("Model saved to %s\n", destPath)
	return nil
}
