package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
