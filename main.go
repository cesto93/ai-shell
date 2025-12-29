package main

import (
	"context"
	"fmt"
	"log"
)

func main() {
	fmt.Println("Starting AI Shell...")

	ctx := context.Background()
	prompt := "Why is the sky blue?"

	fmt.Printf("Querying Ollama with prompt: '%s'\n", prompt)
	err := CallOllama(ctx, prompt)
	if err != nil {
		log.Fatalf("Error calling Ollama: %v", err)
	}
}
