package service

import (
	"context"
	"errors"
	"fmt"

	"ai-shell/llm"
	"ai-shell/service/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// maxMsgSize bounds gRPC message sizes in bytes (base64 images can be large).
const maxMsgSize = 64 << 20

// ErrUnavailable is returned when the service cannot be reached. Sessions use
// it to fall back to local execution.
var ErrUnavailable = errors.New("ai-shell service unavailable")

// ChatRequest carries the messages and session configuration sent to the
// service. When SystemPrompt is non-empty it overrides the agent prompt
// entirely (used by non-agent callers such as the commit command).
type ChatRequest struct {
	Messages        []llm.Message
	SystemPrompt    string
	Agent           string
	Model           string
	Provider        string
	Tools           map[string]bool
	Backend         string
	AgentFiles      bool
	Confirm         bool
	AllowedCommands []string
}

func dial() (*grpc.ClientConn, error) {
	path := SocketPath()
	if path == "" {
		return nil, fmt.Errorf("cannot determine service socket path")
	}
	return grpc.NewClient(
		"unix://"+path,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxMsgSize),
			grpc.MaxCallSendMsgSize(maxMsgSize),
		),
	)
}

func toProtoRequest(req ChatRequest) *proto.ChatRequest {
	return &proto.ChatRequest{
		Messages:        messagesToProto(req.Messages),
		SystemPrompt:    req.SystemPrompt,
		Agent:           req.Agent,
		Model:           req.Model,
		Provider:        req.Provider,
		Tools:           req.Tools,
		Backend:         req.Backend,
		AgentFiles:      req.AgentFiles,
		Confirm:         req.Confirm,
		AllowedCommands: req.AllowedCommands,
	}
}

// Client is a connection to a running ai-shell service.
type Client struct {
	conn *grpc.ClientConn
	svc  proto.AIServiceClient
}

// NewClient dials the service socket. The connection is lazy: no traffic is
// sent until the first RPC.
func NewClient() (*Client, error) {
	conn, err := dial()
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, svc: proto.NewAIServiceClient(conn)}, nil
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Ping reports the service's protocol version.
func (c *Client) Ping(ctx context.Context) (*proto.PingResponse, error) {
	return c.svc.Ping(ctx, &proto.PingRequest{})
}

// Chat runs the agent loop on the service and returns the resulting messages.
func (c *Client) Chat(ctx context.Context, req ChatRequest) ([]llm.Message, error) {
	resp, err := c.svc.Chat(ctx, toProtoRequest(req))
	if err != nil {
		return nil, ErrUnavailable
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}
	return messagesFromProto(resp.Messages), nil
}

// Stop asks the service to shut down gracefully.
func (c *Client) Stop(ctx context.Context) error {
	_, err := c.svc.Stop(ctx, &proto.StopRequest{})
	return err
}

// Chat is a convenience for one-shot callers: it dials, performs a single
// chat round-trip, and closes the connection.
func Chat(ctx context.Context, req ChatRequest) ([]llm.Message, error) {
	c, err := NewClient()
	if err != nil {
		return nil, ErrUnavailable
	}
	defer c.Close()
	return c.Chat(ctx, req)
}
