package service

import (
	"ai-shell/llm"
	"ai-shell/service/proto"
)

// convert.go translates between llm.Message and the wire protocol types.

func toolCallToProto(tc llm.OpenAIToolCall) *proto.RPCToolCall {
	return &proto.RPCToolCall{
		Id:        tc.ID,
		Type:      tc.Type,
		Name:      tc.Function.Name,
		Arguments: tc.Function.Arguments,
	}
}

func toolCallFromProto(tc *proto.RPCToolCall) llm.OpenAIToolCall {
	var out llm.OpenAIToolCall
	out.ID = tc.Id
	out.Type = tc.Type
	out.Function.Name = tc.Name
	out.Function.Arguments = tc.Arguments
	return out
}

func contentPartToProto(p llm.ContentPart) *proto.RPCContentPart {
	out := &proto.RPCContentPart{Type: p.Type, Text: p.Text}
	if p.ImageURL != nil {
		out.ImageUrl = p.ImageURL.URL
	}
	if p.InputAudio != nil {
		out.InputAudioData = p.InputAudio.Data
		out.InputAudioFormat = p.InputAudio.Format
	}
	return out
}

func contentPartFromProto(p *proto.RPCContentPart) llm.ContentPart {
	out := llm.ContentPart{Type: p.Type, Text: p.Text}
	if p.ImageUrl != "" {
		out.ImageURL = &llm.ContentImage{URL: p.ImageUrl}
	}
	if p.InputAudioData != "" || p.InputAudioFormat != "" {
		out.InputAudio = &llm.InputAudio{Data: p.InputAudioData, Format: p.InputAudioFormat}
	}
	return out
}

func messageToProto(m llm.Message) *proto.RPCMessage {
	out := &proto.RPCMessage{
		Role:       m.Role,
		ToolCallId: m.ToolCallID,
	}
	for _, tc := range m.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, toolCallToProto(tc))
	}
	switch c := m.Content.(type) {
	case string:
		out.Content = &proto.RPCContent{Kind: &proto.RPCContent_Text{Text: c}}
	case []llm.ContentPart:
		parts := &proto.RPCContentParts{}
		for _, p := range c {
			parts.Parts = append(parts.Parts, contentPartToProto(p))
		}
		out.Content = &proto.RPCContent{Kind: &proto.RPCContent_Parts{Parts: parts}}
	}
	return out
}

func messageFromProto(p *proto.RPCMessage) llm.Message {
	out := llm.Message{
		Role:       p.Role,
		ToolCallID: p.ToolCallId,
	}
	for _, tc := range p.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, toolCallFromProto(tc))
	}
	if c := p.Content; c != nil {
		switch k := c.Kind.(type) {
		case *proto.RPCContent_Text:
			out.Content = k.Text
		case *proto.RPCContent_Parts:
			parts := make([]llm.ContentPart, 0, len(k.Parts.Parts))
			for _, p := range k.Parts.Parts {
				parts = append(parts, contentPartFromProto(p))
			}
			out.Content = parts
		}
	}
	return out
}

func messagesToProto(msgs []llm.Message) []*proto.RPCMessage {
	out := make([]*proto.RPCMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, messageToProto(m))
	}
	return out
}

func messagesFromProto(msgs []*proto.RPCMessage) []llm.Message {
	out := make([]llm.Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, messageFromProto(m))
	}
	return out
}
