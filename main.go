package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	gpt3 "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
)

func GetResponse(client *gpt3.Client, ctx context.Context, quesiton string) {
	req := gpt3.CompletionRequest{
		Model:     gpt3.GPT3TextDavinci001,
		MaxTokens: 300,
		Prompt:    quesiton,
		Stream:    true,
	}

	resp, err := client.CreateCompletionStream(ctx, req)
	if err != nil {
		fmt.Errorf("CreateCompletionStream returned error: %v", err)
	}
	defer resp.Close()

	counter := 0
	for {
		data, err := resp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			fmt.Errorf("Stream error: %v", err)
		} else {
			counter++
			fmt.Print(data.Choices[0].Text)

		}
	}
	if counter == 0 {
		fmt.Errorf("Stream did not return any responses")
	}
	fmt.Println("")
}

type NullWriter int

func (NullWriter) Write([]byte) (int, error) { return 0, nil }

func main() {
	log.SetOutput(new(NullWriter))
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		panic("Missing API KEY")
	}

	ctx := context.Background()
	client := gpt3.NewClient(apiKey)
	rootCmd := &cobra.Command{
		Use:   "chatgpt",
		Short: "Chat with ChatGPT in console.",
		Run: func(cmd *cobra.Command, args []string) {
			scanner := bufio.NewScanner(os.Stdin)
			quit := false

			for !quit {
				fmt.Print("輸入你的問題(quit 離開): ")

				if !scanner.Scan() {
					break
				}

				question := scanner.Text()
				switch question {
				case "quit":
					quit = true

				default:
					GetResponse(client, ctx, question)
				}
			}
		},
	}

	rootCmd.Execute()
}
