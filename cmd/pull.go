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
	Short: "Download a GGUF model from HuggingFace",
	Long: `Download a GGUF model file from a HuggingFace repository to the llamacpp models directory.

The repo is the HuggingFace repository path (e.g., unsloth/Qwen3.5-2B-GGUF).
The filename is the specific GGUF file to download (e.g., Qwen3.5-2B-Q4_K_M.gguf).

The file is saved to ~/.ai-shell/models/llamacpp/ and the config is updated to use this model.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPull(args[0], args[1])
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
}

func runPull(repo, filename string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot get home dir: %w", err)
	}
	destDir := filepath.Join(home, ".ai-shell", "models", "llamacpp")
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
				fmt.Fprintf(os.Stderr, "\rDownloading... %.1f%% (%s/%s)", pct, formatBytes(written), formatBytes(contentLength))
			} else {
				fmt.Fprintf(os.Stderr, "\rDownloading... %s", formatBytes(written))
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

	modelName := strings.TrimSuffix(filename, ".gguf")
	if err := config.SaveModelWithProvider(modelName, "llamacpp"); err != nil {
		fmt.Printf("Warning: failed to update config: %v\n", err)
	}

	fmt.Printf("Model saved to %s\n", destPath)
	return nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
