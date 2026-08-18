package service

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ai-shell/config"
	"ai-shell/llm"
	"ai-shell/service/proto"
	"ai-shell/tools"

	"google.golang.org/grpc"
)

// Server implements the AIService gRPC API, wrapping the ai-shell logic
// (prompts, AGENTS.md files, tools, and LLM calls) on the service side.
type Server struct {
	proto.UnimplementedAIServiceServer

	// callLLM is swappable for tests.
	callLLM  func(ctx context.Context, agent *llm.Agent, executor llm.ToolExecutor, messages []llm.Message) ([]llm.Message, error)
	stop     chan struct{}
	stopOnce sync.Once
}

// NewServer creates a Server that delegates LLM calls to the agent.
func NewServer() *Server {
	return &Server{
		callLLM: func(ctx context.Context, agent *llm.Agent, executor llm.ToolExecutor, messages []llm.Message) ([]llm.Message, error) {
			return agent.CallLLM(ctx, executor, messages)
		},
		stop: make(chan struct{}),
	}
}

// Ping reports the service's protocol version.
func (s *Server) Ping(_ context.Context, _ *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{Version: ServiceVersion}, nil
}

// Chat builds the agent from the session config, wraps the prompt and
// AGENTS.md files, runs the LLM loop with the service-side executor, and
// returns the resulting messages. LLM errors are reported in the response
// (not as a gRPC error) so clients can distinguish them from connectivity
// failures.
func (s *Server) Chat(ctx context.Context, req *proto.ChatRequest) (*proto.ChatResponse, error) {
	messages := messagesFromProto(req.Messages)

	var agent *llm.Agent
	if req.SystemPrompt != "" {
		agent = &llm.Agent{
			Prompt:   req.SystemPrompt,
			Model:    req.Model,
			Provider: req.Provider,
			Backend:  req.Backend,
		}
	} else {
		agent = llm.NewAgentFor(req.Agent, req.Model, req.Provider, req.Tools)
		agent.Backend = req.Backend
		agent.AgentFiles = llm.GetAgentFiles(req.AgentFiles)
	}

	executor := &ServiceExecutor{
		confirm: req.Confirm,
		allowed: req.AllowedCommands,
	}

	result, err := s.callLLM(ctx, agent, executor, messages)
	if err != nil {
		return &proto.ChatResponse{Error: err.Error()}, nil
	}
	return &proto.ChatResponse{Messages: messagesToProto(result)}, nil
}

// Stop triggers a graceful shutdown of the service.
func (s *Server) Stop(_ context.Context, _ *proto.StopRequest) (*proto.StopResponse, error) {
	s.stopOnce.Do(func() { close(s.stop) })
	return &proto.StopResponse{}, nil
}

// StopCh is closed when Stop is called.
func (s *Server) StopCh() <-chan struct{} {
	return s.stop
}

// Serve listens on the unix socket and serves the AIService until ctx is
// cancelled, Stop is called, or the listener fails. A stale socket file left
// by a crashed run is removed when no live service is listening on it.
func Serve(ctx context.Context, s *Server) error {
	path := SocketPath()
	if path == "" {
		return fmt.Errorf("cannot determine service socket path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	lis, err := net.Listen("unix", path)
	if err != nil {
		if IsActive() {
			return fmt.Errorf("service already running at %s", path)
		}
		if rmErr := os.Remove(path); rmErr == nil {
			lis, err = net.Listen("unix", path)
		}
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", path, err)
		}
	}
	os.Chmod(path, 0600)

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMsgSize),
		grpc.MaxSendMsgSize(maxMsgSize),
	)
	proto.RegisterAIServiceServer(grpcServer, s)

	slog.Info("ai-shell service listening", "socket", path)

	errCh := make(chan error, 1)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		grpcServer.Stop()
		os.Remove(path)
		return err
	case <-ctx.Done():
	case <-s.StopCh():
	}

	grpcServer.GracefulStop()
	os.Remove(path)
	return nil
}

// ServiceExecutor executes tools on the service side. It honors the session's
// confirm policy: when confirm is true, RunCommand is restricted to the
// allowed commands and WriteFile is denied (there is no interactive
// confirmation path); when confirm is false, everything runs automatically.
type ServiceExecutor struct {
	confirm bool
	allowed []string
}

func (e *ServiceExecutor) ExecuteTool(call llm.ToolCall) (string, error) {
	switch call.Name {
	case "RunCommand":
		cmd, ok := call.Arguments["command"].(string)
		if !ok {
			return "Error: Invalid tool arguments", nil
		}
		if e.confirm && !config.IsAllowedCommand(getCommandName(cmd), e.allowed) {
			return "Error: Command execution denied by user", nil
		}
		output, err := tools.RunCommand(cmd)
		if err != nil {
			return fmt.Sprintf("Error: %v\nOutput: %s", err, output), nil
		}
		return output, nil

	case "WriteFile":
		path, ok1 := call.Arguments["path"].(string)
		content, ok2 := call.Arguments["content"].(string)
		if !ok1 || !ok2 {
			return "Error: Invalid tool arguments", nil
		}
		if e.confirm {
			return "Error: File write denied by user", nil
		}
		return tools.WriteFile(strings.TrimPrefix(path, "@"), content)

	case "ReadFile":
		path, ok := call.Arguments["path"].(string)
		if !ok {
			return "Error: Invalid tool arguments", nil
		}
		return tools.ReadFile(strings.TrimPrefix(path, "@"))

	case "KVSet":
		key, ok1 := call.Arguments["key"].(string)
		value, ok2 := call.Arguments["value"].(string)
		if !ok1 || !ok2 {
			return "Error: Invalid tool arguments", nil
		}
		return tools.KVSet(key, value)

	case "KVGet":
		key, ok := call.Arguments["key"].(string)
		if !ok {
			return "Error: Invalid tool arguments", nil
		}
		return tools.KVGet(key)

	case "KVList":
		return tools.KVList()

	default:
		return fmt.Sprintf("Error: Unknown tool %s", call.Name), nil
	}
}

func (e *ServiceExecutor) IsAllowedCommand(cmd string) bool {
	return config.IsAllowedCommand(getCommandName(cmd), e.allowed)
}

func (e *ServiceExecutor) AskConfirmation(cmd string) bool {
	if !e.confirm {
		return true
	}
	return e.IsAllowedCommand(cmd)
}

func getCommandName(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) > 0 {
		return parts[0]
	}
	return cmd
}
