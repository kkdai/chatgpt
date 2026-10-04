package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// Chat keeps the conversation history and talks to the Chat Completions API.
type Chat struct {
	client   *openai.Client
	model    string
	system   string
	messages []openai.ChatCompletionMessage
}

// NewChat creates a conversation. An empty system prompt means none is sent.
func NewChat(client *openai.Client, model, system string) *Chat {
	c := &Chat{client: client, model: model}
	c.SetSystem(system)
	return c
}

// SetSystem replaces the system prompt and clears the history.
func (c *Chat) SetSystem(system string) {
	c.system = system
	c.Reset()
}

// Reset clears the history, keeping the system prompt.
func (c *Chat) Reset() {
	c.messages = nil
	if c.system != "" {
		c.messages = append(c.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: c.system,
		})
	}
}

// Ask sends the question with the conversation history and streams the
// answer to w. The exchange is added to the history only if some answer was
// received, so an interrupted stream keeps its partial text but a failed
// request leaves the history unchanged.
func (c *Chat) Ask(ctx context.Context, w io.Writer, question string) error {
	history := append(c.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: question,
	})

	stream, err := c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: history,
		Stream:   true,
	})
	if err != nil {
		return fmt.Errorf("create stream: %w", err)
	}
	defer stream.Close()

	var answer strings.Builder
	var streamErr error
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			streamErr = fmt.Errorf("stream: %w", err)
			break
		}
		if len(resp.Choices) == 0 {
			continue
		}
		text := resp.Choices[0].Delta.Content
		answer.WriteString(text)
		fmt.Fprint(w, text)
	}
	fmt.Fprintln(w)

	if answer.Len() > 0 {
		c.messages = append(history, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: answer.String(),
		})
	} else if streamErr == nil {
		streamErr = errors.New("stream did not return any response")
	}
	return streamErr
}
