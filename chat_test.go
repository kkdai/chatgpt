package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// newTestChat starts a fake streaming server. handler receives the decoded
// request and returns the SSE chunks to send; a nil slice means HTTP 500.
func newTestChat(t *testing.T, system string, handler func(openai.ChatCompletionRequest) []string) *Chat {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		chunks := handler(req)
		if chunks == nil {
			http.Error(w, `{"error":{"message":"boom"}}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, c := range chunks {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", c)
		}
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":3,\"total_tokens\":15}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)

	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = srv.URL + "/v1"
	return NewChat(openai.NewClientWithConfig(cfg), "test-model", system)
}

func TestAskStreamsAndKeepsHistory(t *testing.T) {
	var requests []openai.ChatCompletionRequest
	chat := newTestChat(t, "be brief", func(r openai.ChatCompletionRequest) []string {
		requests = append(requests, r)
		return []string{"Hel", "lo"}
	})

	var out bytes.Buffer
	if err := chat.Ask(context.Background(), &out, "hi"); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "Hello\n" {
		t.Errorf("output = %q", got)
	}
	if err := chat.Ask(context.Background(), &out, "again"); err != nil {
		t.Fatal(err)
	}

	second := requests[1]
	if second.Model != "test-model" {
		t.Errorf("model = %q", second.Model)
	}
	if second.StreamOptions == nil || !second.StreamOptions.IncludeUsage {
		t.Error("stream usage was not requested")
	}
	if usage := chat.Usage(); usage == nil || usage.TotalTokens != 15 {
		t.Errorf("usage = %+v, want 15 total tokens", usage)
	}
	var roles []string
	for _, m := range second.Messages {
		roles = append(roles, m.Role)
	}
	if got := strings.Join(roles, ","); got != "system,user,assistant,user" {
		t.Errorf("roles = %s", got)
	}
	if second.Messages[2].Content != "Hello" {
		t.Errorf("assistant message = %q", second.Messages[2].Content)
	}
}

func TestAskErrorLeavesHistoryUnchanged(t *testing.T) {
	chat := newTestChat(t, "", func(openai.ChatCompletionRequest) []string { return nil })

	var out bytes.Buffer
	if err := chat.Ask(context.Background(), &out, "hi"); err == nil {
		t.Fatal("expected error")
	}
	if len(chat.messages) != 0 {
		t.Errorf("history = %v", chat.messages)
	}
}

func TestResetAndSetSystem(t *testing.T) {
	chat := newTestChat(t, "sys", func(openai.ChatCompletionRequest) []string { return []string{"ok"} })
	if err := chat.Ask(context.Background(), &bytes.Buffer{}, "hi"); err != nil {
		t.Fatal(err)
	}

	chat.Reset()
	if len(chat.messages) != 1 || chat.messages[0].Role != openai.ChatMessageRoleSystem {
		t.Errorf("after Reset: %v", chat.messages)
	}

	chat.SetSystem("")
	if len(chat.messages) != 0 {
		t.Errorf("after SetSystem(\"\"): %v", chat.messages)
	}
}

func TestValidateQuestion(t *testing.T) {
	for in, want := range map[string]string{"": "", "  ": "", "loop": "", " hello ": "hello"} {
		if got := validateQuestion(in); got != want {
			t.Errorf("validateQuestion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSaveLoadConversation(t *testing.T) {
	chat := NewChat(nil, "test-model", "be brief")
	chat.messages = append(chat.messages, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser, Content: "hello",
	}, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant, Content: "hi",
	})
	path := t.TempDir() + "/conversation.json"
	if err := chat.Save(path, false); err != nil {
		t.Fatal(err)
	}
	if err := chat.Save(path, false); err == nil {
		t.Fatal("expected refusing to overwrite existing file")
	}
	if err := chat.Save(path, true); err != nil {
		t.Fatalf("force save: %v", err)
	}

	loaded := NewChat(nil, "test-model", "old system")
	if err := loaded.Load(path); err != nil {
		t.Fatal(err)
	}
	if loaded.system != "be brief" || len(loaded.messages) != 3 {
		t.Fatalf("loaded state = system %q, messages %v", loaded.system, loaded.messages)
	}
	if loaded.messages[2].Content != "hi" {
		t.Errorf("assistant message = %q", loaded.messages[2].Content)
	}
}

func TestLoadConversationRejectsInvalidFile(t *testing.T) {
	chat := NewChat(nil, "test-model", "keep this")
	path := t.TempDir() + "/bad.json"
	if err := os.WriteFile(path, []byte(`{"version":2,"messages":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := chat.Load(path); err == nil {
		t.Fatal("expected unsupported version error")
	}
	if chat.system != "keep this" || len(chat.messages) != 1 {
		t.Errorf("failed load changed conversation: %+v", chat.messages)
	}
}

func TestCodeFenceWriterHandlesSplitFence(t *testing.T) {
	var out bytes.Buffer
	w := &codeFenceWriter{w: &out}
	for _, part := range []string{"before `", "``go\n", "fmt.Println(1)\n", "`", "`` after"} {
		if _, err := w.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "\x1b[36m") || !strings.Contains(got, "\x1b[0m") {
		t.Errorf("missing code fence colors: %q", got)
	}
	if strings.ReplaceAll(strings.ReplaceAll(got, "\x1b[36m", ""), "\x1b[0m", "") != "before ```go\nfmt.Println(1)\n``` after" {
		t.Errorf("colored output content changed: %q", got)
	}
}

func TestEstimateCost(t *testing.T) {
	usage := &openai.Usage{PromptTokens: 1000, CompletionTokens: 500}
	got, ok := estimateCost("gpt-4o-mini", usage)
	if !ok || got != 0.00045 {
		t.Errorf("estimateCost = %v, %v; want 0.00045, true", got, ok)
	}
	if _, ok := estimateCost("custom-model", usage); ok {
		t.Error("custom model should not have a guessed price")
	}
}
