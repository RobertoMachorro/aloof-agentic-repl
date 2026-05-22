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
	ansiRed    = "\033[31m"
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

type section int

const (
	sectionNone     section = iota
	sectionThinking section = iota
	sectionResponse section = iota
)

var debugMode = os.Getenv("ALOOF_DEBUG") == "1"

func debugf(format string, args ...any) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}

func main() {
	apiKey := os.Getenv("ALOOF_API_KEY")
	endpoint := os.Getenv("ALOOF_ENDPOINT_KEY")
	model := os.Getenv("ALOOF_ENDPOINT_MODEL")

	if apiKey == "" || endpoint == "" || model == "" {
		fmt.Fprintln(os.Stderr, "error: ALOOF_API_KEY, ALOOF_ENDPOINT_KEY, and ALOOF_ENDPOINT_MODEL must be set")
		os.Exit(1)
	}

	fmt.Printf("%saloof%s — model: %s", ansiCyan, ansiReset, model)
	if debugMode {
		fmt.Printf("  %s[debug on]%s", ansiRed, ansiReset)
	}
	fmt.Println()
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

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 256*1024), 256*1024)

	current := sectionNone
	var newContext []int

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}

		var c generateChunk
		if err := json.Unmarshal(line, &c); err != nil {
			debugf("unmarshal error: %v  raw: %s", err, line)
			continue
		}
		debugf("chunk: thinking=%q response=%q done=%v", c.Thinking, c.Response, c.Done)

		// Determine which section this chunk belongs to based on payload content.
		var incoming section
		switch {
		case c.Thinking != "":
			incoming = sectionThinking
		case c.Response != "":
			incoming = sectionResponse
		}

		// Print section header on transitions.
		if incoming != sectionNone && incoming != current {
			switch incoming {
			case sectionThinking:
				fmt.Printf("\n%s%s=== Thinking ===%s\n", ansiDim, ansiItalic, ansiReset)
			case sectionResponse:
				if current == sectionThinking {
					fmt.Println()
				}
				fmt.Printf("\n%s=== Response ===%s\n", ansiBold, ansiReset)
			}
			debugf("section: %d → %d", current, incoming)
			current = incoming
		}

		// Print content for the active section.
		switch current {
		case sectionThinking:
			fmt.Print(ansiDim + ansiItalic + c.Thinking + ansiReset)
		case sectionResponse:
			fmt.Print(c.Response)
		}

		if c.Done {
			newContext = c.Context
			debugf("done: context_len=%d final_section=%d", len(newContext), current)
			break
		}
	}
	if err := sc.Err(); err != nil {
		debugf("scanner error: %v", err)
		return newContext, err
	}

	return newContext, nil
}
