package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type huggingFaceResult struct {
	Siblings []struct {
		Rfilename string `json:"rfilename"`
	} `json:"siblings"`
}

var pullCmd = &cobra.Command{
	Use:   "pull org/repo[:filename]",
	Short: "Pull a GGUF model from HuggingFace",
	Long: `Download a GGUF model file from HuggingFace and save it to ~/.ai-shell/models/.

Examples:
  ai-shell pull TheBloke/Mistral-7B-Instruct-v0.1-GGUF
  ai-shell pull TheBloke/Mistral-7B-Instruct-v0.1-GGUF:mistral-7b-instruct-v0.1.Q4_K_M.gguf
  ai-shell pull https://huggingface.co/TheBloke/Mistral-7B-Instruct-v0.1-GGUF`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		arg := args[0]

		org, repo, filename := parseHuggingFaceRef(arg)

		modelsDir, err := getModelsDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if filename == "" {
			listHuggingFaceFiles(org, repo)
			return
		}

		modelName := modelNameFromFile(filename)
		if modelExistsLocally(modelsDir, modelName) {
			fmt.Printf("Model %q already exists locally. Skipping download.\n", modelName)
			return
		}

		if err := downloadHuggingFaceModel(org, repo, filename, modelsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		companion := "mmproj-" + filename
		if modelName != companion {
			companionPath := filepath.Join(modelsDir, companion)
			if _, err := os.Stat(companionPath); os.IsNotExist(err) {
				if err := downloadHuggingFaceModel(org, repo, companion, modelsDir); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: companion file %s not found in repo: %v\n", companion, err)
				}
			}
		}

		if err := updateModelsIni(modelsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update models.ini: %v\n", err)
		}
	},
}

func parseHuggingFaceRef(ref string) (org, repo, filename string) {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimSuffix(ref, "/")

	if strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://") {
		parts := strings.SplitN(ref, "/blob/main/", 2)
		if len(parts) == 2 {
			pathParts := strings.Split(strings.TrimPrefix(parts[0], "https://huggingface.co/"), "/")
			if len(pathParts) >= 2 {
				return pathParts[0], pathParts[1], parts[1]
			}
		}
		parts = strings.SplitN(strings.TrimPrefix(ref, "https://huggingface.co/"), "/", 3)
		if len(parts) >= 2 {
			return parts[0], parts[1], ""
		}
		return "", "", ""
	}

	if idx := strings.LastIndex(ref, ":"); idx != -1 {
		repoPart := ref[:idx]
		filename = ref[idx+1:]
		parts := strings.Split(repoPart, "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], filename
		}
		return "", "", ""
	}

	parts := strings.Split(ref, "/")
	if len(parts) >= 2 {
		return parts[0], parts[1], ""
	}
	return "", "", ""
}

func getModelsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	modelsDir := filepath.Join(home, ".ai-shell", "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create models directory: %w", err)
	}
	return modelsDir, nil
}

func listHuggingFaceFiles(org, repo string) {
	apiURL := fmt.Sprintf("https://huggingface.co/api/models/%s/%s", org, repo)
	resp, err := http.Get(apiURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching model info: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "HuggingFace API error (%d): %s\n", resp.StatusCode, strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	var result huggingFaceResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	var ggufFiles []string
	for _, s := range result.Siblings {
		if strings.HasSuffix(strings.ToLower(s.Rfilename), ".gguf") {
			ggufFiles = append(ggufFiles, s.Rfilename)
		}
	}

	if len(ggufFiles) == 0 {
		fmt.Printf("No GGUF files found in %s/%s\n", org, repo)
		return
	}

	fmt.Printf("Available GGUF files in %s/%s:\n\n", org, repo)
	for _, f := range ggufFiles {
		fmt.Printf("  %s\n", f)
	}
	fmt.Printf("\nDownload with: ai-shell pull %s/%s:<filename>\n", org, repo)
}

func downloadHuggingFaceModel(org, repo, filename, destDir string) error {
	url := fmt.Sprintf("https://huggingface.co/%s/%s/resolve/main/%s", org, repo, filename)
	destPath := filepath.Join(destDir, filename)

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fmt.Printf("Downloading %s/%s:%s ...\n", org, repo, filename)

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Printf("Downloaded %s (%s)\n", filename, formatSize(written))
	return nil
}

func modelExistsLocally(modelsDir, modelName string) bool {
	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if modelNameFromFile(entry.Name()) == modelName {
			return true
		}
	}
	return false
}

func modelNameFromFile(filename string) string {
	name := strings.TrimSuffix(filename, ".gguf")
	name = strings.TrimPrefix(name, "mmproj-")
	return name
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func updateModelsIni(modelsDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read models directory: %w", err)
	}

	type modelFiles struct {
		modelPath  string
		mmprojPath string
	}
	groups := map[string]*modelFiles{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".gguf") {
			continue
		}
		name := modelNameFromFile(entry.Name())
		fullPath := filepath.Join(modelsDir, entry.Name())
		if groups[name] == nil {
			groups[name] = &modelFiles{}
		}
		if strings.HasPrefix(entry.Name(), "mmproj-") {
			groups[name].mmprojPath = fullPath
		} else {
			groups[name].modelPath = fullPath
		}
	}

	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		g := groups[k]
		if g.modelPath == "" {
			continue
		}
		buf.WriteString(fmt.Sprintf("[%s]\n", k))
		buf.WriteString(fmt.Sprintf("model = %s\n", g.modelPath))
		if g.mmprojPath != "" {
			buf.WriteString(fmt.Sprintf("mmproj = %s\n", g.mmprojPath))
		}
		buf.WriteString("\n")
	}

	iniPath := filepath.Join(home, ".ai-shell", "models.ini")
	if err := os.MkdirAll(filepath.Dir(iniPath), 0755); err != nil {
		return fmt.Errorf("failed to create .ai-shell directory: %w", err)
	}
	if err := os.WriteFile(iniPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write models.ini: %w", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
