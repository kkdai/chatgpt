package main

import (
	"bufio"
	"context"
	"encoding/json"
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
  /save <file>      save the conversation as JSON (use --force to overwrite)
  /load <file>      load a saved JSON conversation
  /image <file>     attach an image (png, jpg, gif, webp) to your next question
  /draw <prompt>    generate an image and save it as draw-<time>.png
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
func ask(ctx context.Context, chat *Chat, timeout time.Duration, question string) error {
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
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
	}
	if usage := chat.Usage(); usage != nil {
		fmt.Fprintf(os.Stderr, "Tokens: %d prompt, %d completion, %d total",
			usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
		if cost, ok := estimateCost(chat.model, usage); ok {
			fmt.Fprintf(os.Stderr, " (estimated cost: $%.6f)", cost)
		}
		fmt.Fprintln(os.Stderr)
	}
	return nil
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
		case input == "/save" || strings.HasPrefix(input, "/save "):
			fields := strings.Fields(input)
			force := false
			var path string
			for _, field := range fields[1:] {
				if field == "--force" {
					force = true
				} else if path == "" {
					path = field
				} else {
					path = ""
					break
				}
			}
			if path == "" {
				fmt.Fprintln(os.Stderr, "Usage: /save [--force] <file>")
			} else if err := chat.Save(path, force); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving conversation: %v\n", err)
			} else {
				fmt.Printf("Conversation saved to %s.\n", path)
			}
		case input == "/load" || strings.HasPrefix(input, "/load "):
			fields := strings.Fields(input)
			if len(fields) != 2 {
				fmt.Fprintln(os.Stderr, "Usage: /load <file>")
			} else if err := chat.Load(fields[1]); err != nil {
				fmt.Fprintf(os.Stderr, "Error loading conversation: %v\n", err)
			} else {
				fmt.Printf("Conversation loaded from %s.\n", fields[1])
			}
		case input == "/image" || strings.HasPrefix(input, "/image "):
			path := strings.TrimSpace(strings.TrimPrefix(input, "/image"))
			if path == "" {
				fmt.Fprintln(os.Stderr, "Usage: /image <file>")
			} else if err := chat.AttachImage(path); err != nil {
				fmt.Fprintf(os.Stderr, "Error attaching image: %v\n", err)
			} else {
				fmt.Printf("Image attached (%d pending); it is sent with your next question.\n", chat.PendingImages())
			}
		case input == "/draw" || strings.HasPrefix(input, "/draw "):
			prompt := strings.TrimSpace(strings.TrimPrefix(input, "/draw"))
			if prompt == "" {
				fmt.Fprintln(os.Stderr, "Usage: /draw <prompt>")
			} else {
				drawCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
				if timeout > 0 {
					var cancel context.CancelFunc
					drawCtx, cancel = context.WithTimeout(drawCtx, timeout)
					defer cancel()
				}
				path, err := chat.Draw(drawCtx, ".", prompt)
				stop()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				} else {
					fmt.Printf("Image saved to %s\n", path)
				}
			}
		case input == "/system" || strings.HasPrefix(input, "/system "):
			chat.SetSystem(strings.TrimSpace(strings.TrimPrefix(input, "/system")))
			fmt.Println("System prompt updated, conversation cleared.")
		default:
			if question := validateQuestion(input); question != "" {
				_ = ask(ctx, chat, timeout, question)
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
		prompt  string
		baseURL string
		timeout time.Duration
		effort  string
		schema  string
	)

	rootCmd := &cobra.Command{
		Use:   "chatgpt",
		Short: "Chat with ChatGPT in console.",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiKey := lookupAPIKey()
			if apiKey == "" {
				return errors.New("missing API key: set API_KEY or OPENAI_API_KEY")
			}
			cfg := openai.DefaultConfig(apiKey)
			if baseURL != "" {
				cfg.BaseURL = baseURL
			}
			chat := NewChat(openai.NewClientWithConfig(cfg), model, system)
			chat.ReasoningEffort = effort
			chat.ImageModel = os.Getenv("OPENAI_IMAGE_MODEL")
			if schema != "" {
				raw := []byte(schema)
				if strings.HasPrefix(schema, "@") {
					var err error
					if raw, err = os.ReadFile(schema[1:]); err != nil {
						return fmt.Errorf("read json schema: %w", err)
					}
				}
				if !json.Valid(raw) {
					return errors.New("--json-schema is not valid JSON")
				}
				chat.JSONSchema = json.RawMessage(raw)
			}
			if prompt != "" || !isTerminal(os.Stdin) {
				var input []byte
				if !isTerminal(os.Stdin) {
					var err error
					input, err = readLimited(os.Stdin, maxStdinSize)
					if err != nil {
						return fmt.Errorf("read stdin: %w", err)
					}
				}
				question := prompt
				if piped := strings.TrimSpace(string(input)); piped != "" {
					if question != "" {
						question += "\n\n"
					}
					question += piped
				}
				question = strings.TrimSpace(question)
				if question == "" {
					return errors.New("provide a prompt with -p or pipe input to stdin")
				}
				if err := ask(cmd.Context(), chat, timeout, question); err != nil {
					return err
				}
				return nil
			}
			run(cmd.Context(), os.Stdin, chat, timeout)
			return nil
		},
	}
	rootCmd.Flags().StringVarP(&model, "model", "m", envOr("OPENAI_MODEL", defaultModel), "model to use (env OPENAI_MODEL)")
	rootCmd.Flags().StringVarP(&system, "system", "s", "", "system prompt")
	rootCmd.Flags().StringVarP(&prompt, "prompt", "p", "", "send one prompt and exit")
	rootCmd.Flags().StringVar(&baseURL, "base-url", envOr("OPENAI_BASE_URL", ""), "OpenAI-compatible API base URL (env OPENAI_BASE_URL)")
	rootCmd.Flags().StringVar(&effort, "reasoning-effort", envOr("OPENAI_REASONING_EFFORT", ""), "reasoning effort for o-series models: low, medium, high (env OPENAI_REASONING_EFFORT)")
	rootCmd.Flags().StringVar(&schema, "json-schema", "", "force JSON output matching this JSON schema (inline JSON or @file)")
	rootCmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "maximum time for one answer (0 for no limit)")

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

// maxStdinSize bounds piped input so unbounded streams cannot exhaust memory.
const maxStdinSize = 16 << 20

// readLimited reads all of r, failing if it holds more than limit bytes.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("input exceeds %d MiB limit", limit>>20)
	}
	return data, nil
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func estimateCost(model string, usage *openai.Usage) (float64, bool) {
	type rates struct{ input, output float64 }
	prices := map[string]rates{
		"gpt-4o":       {2.50, 10.00},
		"gpt-4o-mini":  {0.15, 0.60},
		"gpt-4.1":      {2.00, 8.00},
		"gpt-4.1-mini": {0.40, 1.60},
		"gpt-4.1-nano": {0.10, 0.40},
	}
	rate, ok := prices[model]
	if !ok || usage == nil {
		return 0, false
	}
	return (float64(usage.PromptTokens)*rate.input +
		float64(usage.CompletionTokens)*rate.output) / 1_000_000, true
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
