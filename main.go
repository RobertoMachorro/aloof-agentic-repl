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
)

type generateRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Stream  bool   `json:"stream"`
	Context []int  `json:"context,omitempty"`
}

type generateChunk struct {
	Response string `json:"response"`
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
		fmt.Printf("%s> %s", ansiCyan, ansiReset)
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

	sp := &streamProcessor{}
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
		sp.write(c.Response)
		if c.Done {
			newContext = c.Context
			break
		}
	}
	sp.flushAll()

	return newContext, sc.Err()
}

// streamProcessor renders <think>…</think> blocks in dim italic and normal
// response text in default style, handling tags that may span token boundaries.
type streamProcessor struct {
	thinking bool
	buf      string
}

const tagOpen = "<think>"
const tagClose = "</think>"

func (sp *streamProcessor) write(token string) {
	sp.buf += token
	sp.drain()
}

func (sp *streamProcessor) flushAll() {
	if sp.buf == "" {
		return
	}
	if sp.thinking {
		fmt.Print(ansiDim + ansiItalic + sp.buf + ansiReset)
	} else {
		fmt.Print(sp.buf)
	}
	sp.buf = ""
}

func (sp *streamProcessor) drain() {
	for {
		tag := tagOpen
		if sp.thinking {
			tag = tagClose
		}

		idx := strings.Index(sp.buf, tag)
		if idx >= 0 {
			before := sp.buf[:idx]
			if before != "" {
				if sp.thinking {
					fmt.Print(ansiDim + ansiItalic + before + ansiReset)
				} else {
					fmt.Print(before)
				}
			}
			if sp.thinking {
				fmt.Print(ansiDim + "\n[/thinking]\n" + ansiReset)
				sp.thinking = false
			} else {
				fmt.Print(ansiDim + "[thinking]\n" + ansiReset)
				sp.thinking = true
			}
			sp.buf = sp.buf[idx+len(tag):]
			continue
		}

		// No complete tag found; hold back any partial tag prefix at the end.
		hold := partialPrefixLen(sp.buf, tag)
		safe := sp.buf[:len(sp.buf)-hold]
		if safe != "" {
			if sp.thinking {
				fmt.Print(ansiDim + ansiItalic + safe + ansiReset)
			} else {
				fmt.Print(safe)
			}
		}
		sp.buf = sp.buf[len(sp.buf)-hold:]
		return
	}
}

// partialPrefixLen returns the length of the longest suffix of s that is also
// a prefix of tag, so partial tags at the stream boundary are not printed yet.
func partialPrefixLen(s, tag string) int {
	for n := len(tag) - 1; n > 0; n-- {
		if strings.HasSuffix(s, tag[:n]) {
			return n
		}
	}
	return 0
}
