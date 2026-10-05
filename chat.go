package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	openai "github.com/sashabaranov/go-openai"
)

// Chat keeps the conversation history and talks to the Chat Completions API.
type Chat struct {
	client   *openai.Client
	model    string
	system   string
	messages []openai.ChatCompletionMessage
	usage    *openai.Usage

	// ReasoningEffort is sent as reasoning_effort when non-empty (o-series models).
	ReasoningEffort string
	// JSONSchema, when set, forces the answer to match this schema (Structured Outputs).
	JSONSchema json.RawMessage
	// ImageModel is the model used by /draw (default gpt-image-1).
	ImageModel string
	images     []string
}

const maxImageSize = 20 << 20

var imageMIMETypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// AttachImage queues a local image to be sent with the next question.
func (c *Chat) AttachImage(path string) error {
	mime, ok := imageMIMETypes[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return errors.New("unsupported image type (use png, jpg, gif or webp)")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxImageSize+1))
	if err != nil {
		return fmt.Errorf("read image: %w", err)
	}
	if len(data) > maxImageSize {
		return errors.New("image exceeds 20 MiB limit")
	}
	c.images = append(c.images, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(data))
	return nil
}

// PendingImages returns the number of images waiting for the next question.
func (c *Chat) PendingImages() int {
	return len(c.images)
}

func (c *Chat) userMessage(question string) openai.ChatCompletionMessage {
	if len(c.images) == 0 {
		return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: question}
	}
	parts := []openai.ChatMessagePart{{Type: openai.ChatMessagePartTypeText, Text: question}}
	for _, url := range c.images {
		parts = append(parts, openai.ChatMessagePart{
			Type:     openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{URL: url},
		})
	}
	return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, MultiContent: parts}
}

type codeFenceWriter struct {
	w       io.Writer
	inCode  bool
	pending string
}

// terminalSafeWriter removes terminal control characters from untrusted output.
// Newlines and tabs are retained to preserve normal formatted responses.
type terminalSafeWriter struct {
	w io.Writer
}

func (w terminalSafeWriter) Write(p []byte) (int, error) {
	text := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, string(p))
	_, err := io.WriteString(w.w, text)
	return len(p), err
}

func (w *codeFenceWriter) Write(p []byte) (int, error) {
	n := len(p)
	text := w.pending + string(p)
	w.pending = ""
	trailingTicks := len(text) - len(strings.TrimRight(text, "`"))
	if trailingTicks > 0 && trailingTicks < 3 {
		w.pending = text[len(text)-trailingTicks:]
		text = text[:len(text)-trailingTicks]
	}
	for len(text) > 0 {
		index := strings.Index(text, "```")
		if index < 0 {
			if err := w.writeText(text); err != nil {
				return n, err
			}
			break
		}
		if err := w.writeText(text[:index]); err != nil {
			return n, err
		}
		w.inCode = !w.inCode
		text = text[index+3:]
		if w.inCode {
			if _, err := io.WriteString(w.w, "\x1b[36m"); err != nil {
				return n, err
			}
		} else if _, err := io.WriteString(w.w, "\x1b[0m"); err != nil {
			return n, err
		}
		if _, err := io.WriteString(w.w, "```"); err != nil {
			return n, err
		}
	}
	return n, nil
}

func (w *codeFenceWriter) writeText(text string) error {
	_, err := io.WriteString(w.w, text)
	return err
}

func (w *codeFenceWriter) Flush() error {
	if w.pending != "" {
		if err := w.writeText(w.pending); err != nil {
			return err
		}
		w.pending = ""
	}
	if w.inCode {
		w.inCode = false
		_, err := io.WriteString(w.w, "\x1b[0m")
		return err
	}
	return nil
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

type savedConversation struct {
	Version  int                            `json:"version"`
	Messages []openai.ChatCompletionMessage `json:"messages"`
}

// Save writes the conversation as JSON. Existing files are never replaced
// unless overwrite is true.
func (c *Chat) Save(path string, overwrite bool) error {
	flags := os.O_WRONLY | os.O_CREATE
	if !overwrite {
		flags |= os.O_EXCL
	} else {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		return fmt.Errorf("open conversation file: %w", err)
	}
	defer file.Close()
	if err := file.Chmod(0600); err != nil {
		return fmt.Errorf("set conversation file permissions: %w", err)
	}

	if err := json.NewEncoder(file).Encode(savedConversation{
		Version:  1,
		Messages: c.messages,
	}); err != nil {
		return fmt.Errorf("write conversation file: %w", err)
	}
	return nil
}

// Load replaces the current conversation with a saved JSON conversation.
func (c *Chat) Load(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open conversation file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat conversation file: %w", err)
	}
	if info.Size() > 16<<20 {
		return errors.New("conversation file exceeds 16 MiB limit")
	}

	var saved savedConversation
	decoder := json.NewDecoder(io.LimitReader(file, 16<<20))
	if err := decoder.Decode(&saved); err != nil {
		return fmt.Errorf("decode conversation file: %w", err)
	}
	if saved.Version != 1 {
		return fmt.Errorf("unsupported conversation version %d", saved.Version)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err != nil {
			return fmt.Errorf("decode conversation file trailing data: %w", err)
		}
		return errors.New("conversation file contains trailing data")
	}
	for i, message := range saved.Messages {
		switch message.Role {
		case openai.ChatMessageRoleSystem, openai.ChatMessageRoleUser, openai.ChatMessageRoleAssistant:
		default:
			return fmt.Errorf("invalid role %q in message %d", message.Role, i)
		}
	}
	c.messages = saved.Messages
	c.usage = nil
	c.system = ""
	if len(c.messages) > 0 && c.messages[0].Role == openai.ChatMessageRoleSystem {
		c.system = c.messages[0].Content
	}
	return nil
}

// Usage returns token usage reported by the most recent request, if available.
func (c *Chat) Usage() *openai.Usage {
	return c.usage
}

// Ask sends the question with the conversation history and streams the
// answer to w. The exchange is added to the history only if some answer was
// received, so an interrupted stream keeps its partial text but a failed
// request leaves the history unchanged.
func (c *Chat) Ask(ctx context.Context, w io.Writer, question string) error {
	c.usage = nil
	output := w
	var colorWriter *codeFenceWriter
	if file, ok := w.(*os.File); ok && isTerminal(file) {
		colorWriter = &codeFenceWriter{w: w}
		output = terminalSafeWriter{w: colorWriter}
	}
	history := append(c.messages, c.userMessage(question))

	req := openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: history,
		Stream:   true,
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
		ReasoningEffort: c.ReasoningEffort,
	}
	if len(c.JSONSchema) > 0 {
		req.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:   "response",
				Schema: c.JSONSchema,
				Strict: true,
			},
		}
	}
	stream, err := c.client.CreateChatCompletionStream(ctx, req)
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
		if resp.Usage != nil {
			usage := *resp.Usage
			c.usage = &usage
		}
		if len(resp.Choices) == 0 {
			continue
		}
		text := resp.Choices[0].Delta.Content
		answer.WriteString(text)
		fmt.Fprint(output, text)
	}
	fmt.Fprintln(output)
	if colorWriter != nil {
		colorWriter.Flush()
	}

	if answer.Len() > 0 {
		c.images = nil
		c.messages = append(history, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: answer.String(),
		})
	} else if streamErr == nil {
		streamErr = errors.New("stream did not return any response")
	}
	return streamErr
}
