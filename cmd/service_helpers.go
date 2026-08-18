package cmd

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"ai-shell/config"
	"ai-shell/llm"
	"ai-shell/service"
)

// chatRequestFromConfig builds a service.ChatRequest carrying the session
// configuration (agent, model, provider, tools, backend, AGENTS.md support,
// and confirmation policy) from a loaded config.
func chatRequestFromConfig(cfg *config.Config, messages []llm.Message) service.ChatRequest {
	return service.ChatRequest{
		Messages:        messages,
		Agent:           cfg.Agent,
		Model:           cfg.LLM.Model,
		Provider:        cfg.LLM.Provider,
		Tools:           cfg.Tools,
		Backend:         cfg.LitertLM.Backend,
		AgentFiles:      cfg.AgentFiles,
		Confirm:         cfg.Shell.Confirm,
		AllowedCommands: cfg.Shell.AllowedCommands,
	}
}

// chatWithServiceFallback routes a chat request through the running service
// when active, falling back to local execution when the service is
// unreachable. wrap, if non-nil, is applied to service-originated errors that
// are not ErrUnavailable.
func chatWithServiceFallback(req service.ChatRequest, wrap func(error) error, local func() ([]llm.Message, error)) ([]llm.Message, error) {
	if !service.IsActive() {
		return local()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := service.Chat(ctx, req)
	if err == nil {
		return result, nil
	}
	if !errors.Is(err, service.ErrUnavailable) {
		if wrap != nil {
			return nil, wrap(err)
		}
		return nil, err
	}
	slog.Debug("service unavailable, falling back to local execution", "err", err)
	return local()
}
