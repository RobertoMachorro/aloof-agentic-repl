package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	ansiReset  = "\033[0m"
	ansiDim    = "\033[2m"
	ansiItalic = "\033[3m"
	ansiCyan   = "\033[36m"
	ansiBold   = "\033[1m"
)

type generateRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Stream  bool   `json:"stream"`
	Context []int  `json:"context,omitempty"`
}

type generateChunk struct {
	Response string `json:"response"`
	Thinking string `json:"thinking"`
	Done     bool   `json:"done"`
	Context  []int  `json:"context,omitempty"`
}

func main() {
	apiKey := os.Getenv("ALOOF_API_KEY")
	endpoint := os.Getenv("ALOOF_ENDPOINT_KEY")
	model := os.Getenv("ALOOF_ENDPOINT_MODEL")

	if apiKey == "" || endpoint == "" || model == "" {
		fmt.Fprintln(os.Stderr, "error: ALOOF_API_KEY, ALOOF_ENDPOINT_KEY, and ALOOF_ENDPOINT_MODEL must be set")
		os.Exit(1)
	}

	fmt.Printf("%saloof%s — model: %s\n", ansiCyan, ansiReset, model)
	fmt.Println("Ctrl+C or Ctrl+D to exit.")
	fmt.Println()

	var context []int
	scanner := bufio.NewScanner(os.Stdin)

	for {
		if len(context) > 0 {
			fmt.Printf("%s(ctx: %d tokens)%s > ", ansiDim, len(context), ansiReset)
		} else {
			fmt.Printf("%s>%s ", ansiCyan, ansiReset)
		}
		if !scanner.Scan() {
			fmt.Println()
			break
		}
		prompt := strings.TrimSpace(scanner.Text())
		if prompt == "" {
			continue
		}

		newCtx, err := generate(apiKey, endpoint, model, prompt, context)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}
		if len(newCtx) > 0 {
			context = newCtx
		}
		fmt.Print("\n\n")
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "input error: %v\n", err)
	}
}

const (
	phaseNone     = iota
	phaseThinking
	phaseResponse
)

func generate(apiKey, endpoint, model, prompt string, context []int) ([]int, error) {
	reqBody, err := json.Marshal(generateRequest{
		Model:   model,
		Prompt:  prompt,
		Stream:  true,
		Context: context,
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	phase := phaseNone
	sc := bufio.NewScanner(resp.Body)
	var newContext []int

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var c generateChunk
		if err := json.Unmarshal(line, &c); err != nil {
			continue
		}

		if c.Thinking != "" && phase != phaseThinking {
			fmt.Printf("\n%s%s=== Thinking ===%s\n", ansiDim, ansiItalic, ansiReset)
			phase = phaseThinking
		}

		if c.Response != "" && phase != phaseResponse {
			if phase == phaseThinking {
				fmt.Println()
			}
			fmt.Printf("\n%s=== Response ===%s\n", ansiBold, ansiReset)
			phase = phaseResponse
		}

		switch phase {
		case phaseThinking:
			fmt.Print(ansiDim + ansiItalic + c.Thinking + ansiReset)
		case phaseResponse:
			fmt.Print(c.Response)
		}

		if c.Done {
			newContext = c.Context
			break
		}
	}

	return newContext, sc.Err()
}
