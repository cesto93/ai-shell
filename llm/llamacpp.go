package llm

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ai-shell/config"
	"ai-shell/stats"

	"github.com/hybridgroup/yzma/pkg/llama"
)

type LlamacppCaller struct {
	Model    string
	Executor ToolExecutor

	once     sync.Once
	initErr  error
	lctx     llama.Context
	llm      llama.Model
	vocab    llama.Vocab
	smplr    llama.Sampler
	template string
	libDir   string
	modelDir string
}

const llamacppDefaultCtxSize = 4096
const llamacppDefaultMaxTokens = 2048

func NewLlamacppCaller(model string, executor ToolExecutor) *LlamacppCaller {
	return &LlamacppCaller{
		Model:    model,
		Executor: executor,
	}
}

func (l *LlamacppCaller) Call(ctx context.Context, systemPrompt string, messages []Message, tools []any) ([]Message, error) {
	if err := l.ensureInit(); err != nil {
		return nil, fmt.Errorf("llamacpp: %w", err)
	}
	prompt, err := l.buildPrompt(systemPrompt, messages)
	if err != nil {
		return nil, err
	}
	return l.generate(ctx, prompt, l.smplr)
}

// CallStructured generates output constrained by a GBNF grammar derived from
// the JSON schema in the OpenAI-style response_format. The grammar sampler is
// added after the greedy sampler so it filters candidate tokens.
func (l *LlamacppCaller) CallStructured(ctx context.Context, systemPrompt string, messages []Message, tools []any, responseFormat any) ([]Message, error) {
	if err := l.ensureInit(); err != nil {
		return nil, fmt.Errorf("llamacpp: %w", err)
	}
	grammar, err := grammarFromResponseFormat(responseFormat)
	if err != nil {
		return nil, fmt.Errorf("llamacpp: %w", err)
	}
	slog.Debug("llamacpp: structured output grammar", "grammar", grammar)

	chain := llama.SamplerChainInit(llama.SamplerChainDefaultParams())
	llama.SamplerChainAdd(chain, llama.SamplerInitGreedy())
	grammarSampler := llama.SamplerInitGrammar(l.vocab, grammar, "root")
	if grammarSampler == 0 {
		return nil, fmt.Errorf("llamacpp: grammar sampler failed to initialize (unsupported or invalid grammar)")
	}
	llama.SamplerChainAdd(chain, grammarSampler)

	prompt, err := l.buildPrompt(systemPrompt, messages)
	if err != nil {
		return nil, err
	}
	return l.generate(ctx, prompt, chain)
}

func (l *LlamacppCaller) ensureInit() error {
	l.once.Do(func() {
		l.initErr = l.initialize()
	})
	return l.initErr
}

func (l *LlamacppCaller) buildPrompt(systemPrompt string, messages []Message) (string, error) {
	chatMsgs := make([]llama.ChatMessage, 0, 1+len(messages))
	if systemPrompt != "" {
		chatMsgs = append(chatMsgs, llama.NewChatMessage("system", systemPrompt))
	}
	for _, msg := range messages {
		content := formatContent(msg.Content)
		if msg.Role == "tool" {
			content = "Tool result (" + msg.ToolCallID + "): " + content
		}
		chatMsgs = append(chatMsgs, llama.NewChatMessage(msg.Role, content))
	}

	prompt := l.applyChatTemplate(chatMsgs, true)
	if prompt == "" {
		return "", fmt.Errorf("chat template returned empty prompt")
	}
	return prompt, nil
}

func (l *LlamacppCaller) generate(ctx context.Context, prompt string, smplr llama.Sampler) ([]Message, error) {
	tokens := llama.Tokenize(l.vocab, prompt, true, true)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("tokenization failed")
	}
	promptTokens := len(tokens)

	batch := llama.BatchGetOne(tokens)

	var response strings.Builder
	completionTokens := 0
	for pos := int32(0); pos < llamacppDefaultMaxTokens; pos += batch.NTokens {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if _, err := llama.Decode(l.lctx, batch); err != nil {
			return nil, fmt.Errorf("llamacpp decode: %w", err)
		}

		token := llama.SamplerSample(smplr, l.lctx, -1)
		if llama.VocabIsEOG(l.vocab, token) {
			break
		}
		completionTokens++

		buf := make([]byte, 256)
		n := llama.TokenToPiece(l.vocab, token, buf, 0, false)
		if n > 0 {
			response.Write(buf[:n])
		}

		batch = llama.BatchGetOne([]llama.Token{token})
	}

	text := strings.TrimSpace(response.String())
	stats.RecordUsage("llamacpp", l.Model, stats.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	})
	return []Message{{Role: "assistant", Content: text}}, nil
}

func formatContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (l *LlamacppCaller) applyChatTemplate(chatMsgs []llama.ChatMessage, addAssistant bool) string {
	buf := make([]byte, 131072)
	n := llama.ChatApplyTemplate(l.template, chatMsgs, addAssistant, buf)
	if n <= 0 {
		return ""
	}
	if n >= int32(len(buf)) {
		buf = make([]byte, n+1)
		n = llama.ChatApplyTemplate(l.template, chatMsgs, addAssistant, buf)
		if n <= 0 {
			return ""
		}
	}
	return string(buf[:n])
}

func (l *LlamacppCaller) initialize() error {
	libDir, err := config.LibDir()
	if err != nil {
		return fmt.Errorf("cannot determine lib dir: %w", err)
	}
	modelDir, err := config.ModelsDir("llamacpp")
	if err != nil {
		return fmt.Errorf("cannot determine models dir: %w", err)
	}
	l.libDir = libDir
	l.modelDir = modelDir

	slog.Debug("llamacpp: loading library", "dir", l.libDir)
	if err := llama.Load(l.libDir); err != nil {
		return fmt.Errorf("failed to load llama library from %s: %w", l.libDir, err)
	}

	llama.LogSet(llama.LogSilent())
	llama.Init()

	modelPath := filepath.Join(l.modelDir, l.Model)
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		modelPath = filepath.Join(l.modelDir, l.Model+".gguf")
		if _, err := os.Stat(modelPath); os.IsNotExist(err) {
			return fmt.Errorf("model file not found: tried %s and %s",
				filepath.Join(l.modelDir, l.Model), modelPath)
		}
	}

	slog.Debug("llamacpp: loading model", "path", modelPath)
	llm, err := llama.ModelLoadFromFile(modelPath, llama.ModelDefaultParams())
	if err != nil {
		return fmt.Errorf("failed to load model from %s: %w", modelPath, err)
	}
	if llm == 0 {
		return fmt.Errorf("failed to load model from %s: returned null", modelPath)
	}
	l.llm = llm

	l.vocab = llama.ModelGetVocab(llm)

	ctxParams := llama.ContextDefaultParams()
	ctxParams.NCtx = llamacppDefaultCtxSize
	ctxParams.NBatch = llamacppDefaultCtxSize

	slog.Debug("llamacpp: creating context")
	lctx, err := llama.InitFromModel(llm, ctxParams)
	if err != nil {
		llama.ModelFree(llm)
		return fmt.Errorf("failed to create context: %w", err)
	}
	l.lctx = lctx

	chain := llama.SamplerChainInit(llama.SamplerChainDefaultParams())
	llama.SamplerChainAdd(chain, llama.SamplerInitGreedy())
	l.smplr = chain

	l.template = llama.ModelChatTemplate(llm, "")
	if l.template == "" {
		l.template = "chatml"
	}

	slog.Debug("llamacpp: initialized successfully",
		"model", filepath.Base(modelPath),
		"template", l.template)

	return nil
}
