package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const defaultImageModel = openai.CreateImageModelGptImage1

// Draw generates an image for prompt and saves it as a PNG in dir.
// It returns the saved file path. The file is never overwritten.
func (c *Chat) Draw(ctx context.Context, dir, prompt string) (string, error) {
	model := c.ImageModel
	if model == "" {
		model = defaultImageModel
	}
	resp, err := c.client.CreateImage(ctx, openai.ImageRequest{
		Prompt: prompt,
		Model:  model,
		N:      1,
	})
	if err != nil {
		return "", fmt.Errorf("generate image: %w", err)
	}
	if len(resp.Data) == 0 {
		return "", errors.New("no image returned")
	}
	img := resp.Data[0]
	if img.B64JSON == "" {
		if img.URL != "" {
			return img.URL, nil
		}
		return "", errors.New("no image data returned")
	}
	data, err := base64.StdEncoding.DecodeString(img.B64JSON)
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}
	name := fmt.Sprintf("%s/draw-%s.png", dir, time.Now().Format("20060102-150405.000"))
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("save image: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", fmt.Errorf("save image: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("save image: %w", err)
	}
	return name, nil
}
