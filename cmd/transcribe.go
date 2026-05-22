package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"ai-shell/config"
	"ai-shell/llm"

	"github.com/spf13/cobra"
)

var transcribeCmd = &cobra.Command{
	Use:   "transcribe <audio-file>",
	Short: "Transcribe an audio file using the current model",
	Long: `Send an audio file to the current LLM for transcription.
The model must support audio input (e.g., qwen3-asr via llamacpp).`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		audioPath := args[0]
		language, _ := cmd.Flags().GetString("language")

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		supportsAudio := false
		for _, t := range cfg.LLM.InputTypes {
			if t == "audio" {
				supportsAudio = true
				break
			}
		}
		if !supportsAudio {
			fmt.Fprintf(os.Stderr, "Error: The current model %q does not support audio input.\n", cfg.LLM.Model)
			fmt.Fprintf(os.Stderr, "Use a model with audio input capability (e.g., qwen3-asr via llamacpp).\n")
			os.Exit(1)
		}

		baseURL, apiKey := getTranscribeBaseURL(cfg.LLM.Provider)

		fmt.Fprintf(os.Stderr, "Transcribing %s using %s (%s)...\n", audioPath, cfg.LLM.Model, cfg.LLM.Provider)

		audioData, err := os.ReadFile(audioPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading audio file: %v\n", err)
			os.Exit(1)
		}

		audioFormat := audioFileFormat(audioPath)
		b64Audio := base64.StdEncoding.EncodeToString(audioData)

		promptText := "Transcribe the following audio."
		if language != "" {
			promptText = fmt.Sprintf("Transcribe the following audio to %s.", language)
		}

		messages := []llm.Message{
			{
				Role: "user",
				Content: []llm.ContentPart{
					{Type: "text", Text: promptText},
					{Type: "input_audio", InputAudio: &llm.InputAudio{Data: b64Audio, Format: audioFormat}},
				},
			},
		}

		reqBody := map[string]any{
			"model":    cfg.LLM.Model,
			"messages": messages,
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error building request: %v\n", err)
			os.Exit(1)
		}

		req, err := http.NewRequestWithContext(
			context.Background(),
			"POST",
			baseURL+"/chat/completions",
			bytes.NewBuffer(jsonBody),
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
			os.Exit(1)
		}

		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error sending request: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading response: %v\n", err)
			os.Exit(1)
		}

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "API error: %s\n%s\n", resp.Status, string(body))
			os.Exit(1)
		}

		var openAIResp llm.OpenAIResponse
		if err := json.Unmarshal(body, &openAIResp); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing response: %v\n%s\n", err, string(body))
			os.Exit(1)
		}

		if len(openAIResp.Choices) == 0 {
			fmt.Fprintf(os.Stderr, "Empty response from model\n")
			os.Exit(1)
		}

		content := openAIResp.Choices[0].Message.Content
		switch v := content.(type) {
		case string:
			fmt.Println(v)
		default:
			b, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println(string(b))
		}
	},
}

func getTranscribeBaseURL(provider string) (string, string) {
	switch provider {
	case "ollama":
		baseURL := os.Getenv("OLLAMA_HOST")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return baseURL + "/v1", ""
	case "llamacpp":
		baseURL := os.Getenv("LLAMACPP_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}
		return baseURL + "/v1", ""
	case "gemini":
		baseURL := "https://generativelanguage.googleapis.com/v1beta/openai"
		return baseURL, os.Getenv("GEMINI_API_KEY")
	case "litertlm":
		baseURL := os.Getenv("LITERTLM_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:9379"
		}
		return baseURL, ""
	case "openrouter":
		return "https://openrouter.ai/api/v1", os.Getenv("OPEN_ROUTE_KEY")
	default:
		return "http://localhost:11434/v1", ""
	}
}

func audioFileFormat(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".wav":
		return "wav"
	case ".mp3":
		return "mp3"
	case ".ogg":
		return "ogg"
	case ".flac":
		return "flac"
	case ".webm":
		return "webm"
	case ".m4a":
		return "mp4"
	case ".aac":
		return "aac"
	case ".opus":
		return "opus"
	default:
		return "wav"
	}
}

func init() {
	rootCmd.AddCommand(transcribeCmd)
	transcribeCmd.Flags().StringP("language", "l", "", "Language to transcribe to (e.g., \"English\", \"French\")")
}
