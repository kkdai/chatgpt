package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	gpt3 "github.com/PullRequestInc/go-gpt3"
	"github.com/spf13/cobra"
)

func PrintResult(client gpt3.Client, ctx context.Context, quesiton string) {
	resp, err := client.Completion(ctx, gpt3.CompletionRequest{
		Prompt: []string{
			quesiton,
		},
		MaxTokens: gpt3.IntPtr(500),
		Echo:      true,
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(13)
	}
	fmt.Printf("%s\n", resp.Choices[0].Text)
}

type NullWriter int

func (NullWriter) Write([]byte) (int, error) { return 0, nil }

func main() {
	log.SetOutput(new(NullWriter))
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatalln("Missing API KEY")
	}

	ctx := context.Background()
	client := gpt3.NewClient(apiKey)
	rootCmd := &cobra.Command{
		Use:   "ilovegpt",
		Short: "Chat with ChatGPT in console.",
		Run: func(cmd *cobra.Command, args []string) {
			scanner := bufio.NewScanner(os.Stdin)
			quit := false

			for !quit {
				fmt.Print("Input your quesiton:> ")

				if !scanner.Scan() {
					break
				}

				question := scanner.Text()
				switch question {
				case "quit":
					quit = true

				default:
					PrintResult(client, ctx, question)
				}
			}
		},
	}

	rootCmd.Execute()
}
