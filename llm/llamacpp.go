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
	"github.com/hybridgroup/yzma/pkg/mtmd"
)

type LlamacppCaller struct {
	Model    string
	Executor ToolExecutor

	once      sync.Once
	initErr   error
	lctx      llama.Context
	llm       llama.Model
	vocab     llama.Vocab
	smplr     llama.Sampler
	template  string
	libDir    string
	modelDir  string
	mtmdCtx   mtmd.Context
	hasVision bool
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
	return l.chat(ctx, systemPrompt, messages, l.smplr)
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

	return l.chat(ctx, systemPrompt, messages, chain)
}

// chat routes text-only prompts through buildPrompt + generate and multimodal
// prompts (image attachments) through the mtmd vision path.
func (l *LlamacppCaller) chat(ctx context.Context, systemPrompt string, messages []Message, smplr llama.Sampler) ([]Message, error) {
	if imagesPresent(messages) {
		if l.mtmdCtx == 0 || !l.hasVision {
			return nil, fmt.Errorf("llamacpp: image input requires a vision projector. Place a *mmproj*.gguf matching your model in %s", l.modelDir)
		}
		inp, err := l.buildVisionPrompt(systemPrompt, messages)
		if err != nil {
			return nil, err
		}
		return l.generateVision(ctx, inp, smplr)
	}
	prompt, err := l.buildPrompt(systemPrompt, messages)
	if err != nil {
		return nil, err
	}
	return l.generate(ctx, prompt, smplr)
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

// visionInput holds the chat-template rendered prompt plus the decoded image
// payloads in the order their markers appear in the prompt.
type visionInput struct {
	prompt string
	images [][]byte
}

// buildVisionPrompt renders a multimodal conversation for the chat template,
// inserting the mtmd media marker at each image's position and collecting the
// decoded image bytes.
func (l *LlamacppCaller) buildVisionPrompt(systemPrompt string, messages []Message) (*visionInput, error) {
	chatMsgs := make([]llama.ChatMessage, 0, 1+len(messages))
	if systemPrompt != "" {
		chatMsgs = append(chatMsgs, llama.NewChatMessage("system", systemPrompt))
	}
	marker := mtmd.DefaultMarker()
	var images [][]byte
	for _, msg := range messages {
		content, imgs := formatVisionContent(msg.Content, marker)
		images = append(images, imgs...)
		if msg.Role == "tool" {
			content = "Tool result (" + msg.ToolCallID + "): " + content
		}
		chatMsgs = append(chatMsgs, llama.NewChatMessage(msg.Role, content))
	}

	prompt := l.applyChatTemplate(chatMsgs, true)
	if prompt == "" {
		return nil, fmt.Errorf("chat template returned empty prompt")
	}
	return &visionInput{prompt: prompt, images: images}, nil
}

// formatVisionContent renders a message's content as template text, emitting
// the media marker once per image and returning the decoded image bytes.
func formatVisionContent(content any, marker string) (string, [][]byte) {
	switch v := content.(type) {
	case string:
		return v, nil
	case []ContentPart:
		var sb strings.Builder
		var images [][]byte
		for _, p := range v {
			switch p.Type {
			case "text":
				sb.WriteString(p.Text)
			case "image_url":
				if p.ImageURL == nil {
					continue
				}
				data, err := decodeDataURL(p.ImageURL.URL)
				if err != nil {
					slog.Warn("llamacpp: skipping invalid image data URL", "err", err)
					continue
				}
				sb.WriteString(marker)
				images = append(images, data)
			case "input_audio":
				slog.Warn("llamacpp: audio input is not supported")
			}
		}
		return sb.String(), images
	case nil:
		return "", nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// imagesPresent reports whether any message carries an image_url content part.
func imagesPresent(messages []Message) bool {
	for _, msg := range messages {
		if parts, ok := msg.Content.([]ContentPart); ok {
			for _, p := range parts {
				if p.Type == "image_url" && p.ImageURL != nil {
					return true
				}
			}
		}
	}
	return false
}

func (l *LlamacppCaller) generate(ctx context.Context, prompt string, smplr llama.Sampler) ([]Message, error) {
	tokens := llama.Tokenize(l.vocab, prompt, true, true)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("tokenization failed")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if _, err := llama.Decode(l.lctx, llama.BatchGetOne(tokens)); err != nil {
		return nil, fmt.Errorf("llamacpp decode: %w", err)
	}
	return l.sample(ctx, smplr, len(tokens))
}

// generateVision evaluates a multimodal prompt (text + images) through the
// mtmd context, then samples the completion starting from the resulting
// context position.
func (l *LlamacppCaller) generateVision(ctx context.Context, inp *visionInput, smplr llama.Sampler) ([]Message, error) {
	if len(inp.images) == 0 {
		return l.generate(ctx, inp.prompt, smplr)
	}
	if l.mtmdCtx == 0 {
		return nil, fmt.Errorf("llamacpp: multimodal context not initialized")
	}

	chunks := mtmd.InputChunksInit()
	defer mtmd.InputChunksFree(chunks)

	bitmaps := make([]mtmd.Bitmap, 0, len(inp.images))
	for _, data := range inp.images {
		if len(data) == 0 {
			continue
		}
		b := mtmd.BitmapInitFromBuf(l.mtmdCtx, &data[0], uint64(len(data)), false)
		if b.Bitmap == 0 {
			mtmd.BitmapFree(b.Bitmap)
			return nil, fmt.Errorf("llamacpp: failed to decode image")
		}
		bitmaps = append(bitmaps, b.Bitmap)
	}
	defer func() {
		for _, b := range bitmaps {
			mtmd.BitmapFree(b)
		}
	}()
	if len(bitmaps) == 0 {
		return l.generate(ctx, inp.prompt, smplr)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	input := mtmd.NewInputText(inp.prompt, true, true)
	if res := mtmd.Tokenize(l.mtmdCtx, chunks, input, bitmaps); res != 0 {
		return nil, fmt.Errorf("llamacpp: mtmd tokenize failed: %d", res)
	}

	var nPast llama.Pos
	if res := mtmd.HelperEvalChunks(l.mtmdCtx, l.lctx, chunks, 0, 0, int32(llamacppDefaultCtxSize), true, &nPast); res != 0 {
		return nil, fmt.Errorf("llamacpp: mtmd eval chunks failed: %d", res)
	}

	return l.sample(ctx, smplr, len(inp.prompt))
}

// sample runs the token generation loop from the current context state (the
// logits of the last decoded token).
func (l *LlamacppCaller) sample(ctx context.Context, smplr llama.Sampler, promptTokens int) ([]Message, error) {
	var response strings.Builder
	completionTokens := 0
	for pos := int32(0); pos < llamacppDefaultMaxTokens; pos++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
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

		if _, err := llama.Decode(l.lctx, llama.BatchGetOne([]llama.Token{token})); err != nil {
			return nil, fmt.Errorf("llamacpp decode: %w", err)
		}
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
	case []ContentPart:
		var sb strings.Builder
		for _, p := range v {
			if p.Type == "text" {
				sb.WriteString(p.Text)
			}
		}
		return sb.String()
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

	l.setupVision(llm)

	slog.Debug("llamacpp: initialized successfully",
		"model", filepath.Base(modelPath),
		"template", l.template,
		"vision", l.hasVision)

	return nil
}

// setupVision initializes the mtmd (multimodal) context when a vision
// projector (mmproj) is available. Failures degrade gracefully: text-only
// inference keeps working and image requests surface a clear error.
func (l *LlamacppCaller) setupVision(llm llama.Model) {
	mmproj, err := config.FindLlamacppMMProj()
	if err != nil || mmproj == "" {
		slog.Debug("llamacpp: no mmproj found, image input disabled", "err", err)
		return
	}
	if err := mtmd.Load(l.libDir); err != nil {
		slog.Debug("llamacpp: llama library has no mtmd support, image input disabled", "err", err)
		return
	}
	mtmd.LogSet(llama.LogSilent())

	mctx, err := mtmd.InitFromFile(mmproj, llm, mtmd.ContextParamsDefault())
	if err != nil || mctx == 0 {
		slog.Warn("llamacpp: failed to initialize multimodal context, image input disabled", "mmproj", mmproj, "err", err)
		return
	}
	l.mtmdCtx = mctx
	l.hasVision = mtmd.SupportVision(mctx)
	slog.Debug("llamacpp: multimodal context initialized", "mmproj", mmproj, "vision", l.hasVision)
}
