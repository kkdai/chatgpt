package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
)

const defaultModel = openai.GPT4o

const helpText = `Commands:
  /system <prompt>  set the system prompt and start a new conversation
  /reset            clear the conversation history
  /help             show this help
  quit, exit        leave
Press Ctrl+C while an answer is streaming to interrupt it.`

// lookupAPIKey returns the API key from API_KEY or OPENAI_API_KEY.
func lookupAPIKey() string {
	if key := os.Getenv("API_KEY"); key != "" {
		return key
	}
	return os.Getenv("OPENAI_API_KEY")
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// ask runs one request. Ctrl+C cancels the request instead of the program.
func ask(ctx context.Context, chat *Chat, timeout time.Duration, question string) {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if err := chat.Ask(ctx, os.Stdout, question); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			fmt.Fprintln(os.Stderr, "[interrupted]")
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

func run(ctx context.Context, in io.Reader, chat *Chat, timeout time.Duration) {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for {
		fmt.Print("Input your question (type `quit` to exit, `/help` for commands): ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		switch {
		case input == "quit" || input == "exit":
			return
		case input == "/help":
			fmt.Println(helpText)
		case input == "/reset":
			chat.Reset()
			fmt.Println("Conversation cleared.")
		case input == "/system" || strings.HasPrefix(input, "/system "):
			chat.SetSystem(strings.TrimSpace(strings.TrimPrefix(input, "/system")))
			fmt.Println("System prompt updated, conversation cleared.")
		default:
			if question := validateQuestion(input); question != "" {
				ask(ctx, chat, timeout, question)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
	}
}

func main() {
	var (
		model   string
		system  string
		timeout time.Duration
	)

	rootCmd := &cobra.Command{
		Use:   "chatgpt",
		Short: "Chat with ChatGPT in console.",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiKey := lookupAPIKey()
			if apiKey == "" {
				return errors.New("missing API key: set API_KEY or OPENAI_API_KEY")
			}
			chat := NewChat(openai.NewClient(apiKey), model, system)
			run(cmd.Context(), os.Stdin, chat, timeout)
			return nil
		},
	}
	rootCmd.Flags().StringVarP(&model, "model", "m", envOr("OPENAI_MODEL", defaultModel), "model to use (env OPENAI_MODEL)")
	rootCmd.Flags().StringVarP(&system, "system", "s", "", "system prompt")
	rootCmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "maximum time for one answer (0 for no limit)")

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func validateQuestion(question string) string {
	quest := strings.Trim(question, " ")
	keywords := []string{"", "loop", "break", "continue", "cls", "block"}
	for _, x := range keywords {
		if quest == x {
			return ""
		}
	}
	return quest
}
