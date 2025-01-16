package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/openai/openai-go"
	"github.com/spf13/pflag"
)

var (
	DEFAULT_MODEL = "gpt-4o-mini"
)

func init() {
	model := os.Getenv("LLAWK_MODEL")
	if model != "" {
		DEFAULT_MODEL = model
	}
}

//go:embed prompt/system.txt
var systemPromptTemplateText string

//go:embed prompt/user.txt
var userPromptTemplateText string

func init() {
	systemPromptTemplateText = strings.TrimSuffix(systemPromptTemplateText, "\n")
	userPromptTemplateText = strings.TrimSuffix(userPromptTemplateText, "\n")
}

var systemPromptTemplate = template.Must(template.New("system").Parse(systemPromptTemplateText))
var userPromptTemplate = template.Must(template.New("prompt").Parse(userPromptTemplateText))

type Request struct {
	Instruct   string
	Input      string
	InputName  string
	Format     string
	Schema     string
	OutputName string
	Verbose    bool
}

func (r *Request) SystemPrompt() string {
	var buf strings.Builder
	if err := systemPromptTemplate.Execute(&buf, map[string]string{
		"CurrentTime": time.Now().Format(time.RFC3339),
	}); err != nil {
		return fmt.Sprintf("Failed to generate prompt: %v", err)
	}
	return buf.String()
}

func (r *Request) UserPrompt() string {
	var buf strings.Builder
	if err := userPromptTemplate.Execute(&buf, r); err != nil {
		return fmt.Sprintf("Failed to generate prompt: %v", err)
	}
	return buf.String()
}

type LLM interface {
	io.Closer

	Invoke(context.Context, io.Writer, *Request) error
}

func NewLLM(model string) (LLM, error) {
	for _, m := range models {
		if m.Name == model || (m.Matcher != nil && m.Matcher(model)) {
			return m.Dialer.Dial(model)
		}
	}
	return nil, fmt.Errorf("Unknown model: %s\nPlease check -m list", model)
}

type LLMDialer interface {
	Dial(model string) (LLM, error)
}

var models = []struct {
	Name    string
	Dialer  LLMDialer
	Matcher func(string) bool
}{
	{"gpt-4o", OpenAIDialer{Stream: true}, nil},
	{"gpt-4o-mini", OpenAIDialer{Stream: true}, nil},
	{"o1", OpenAIDialer{Stream: false}, nil},
	{"gemini-1.5-flash", GoogleDialer{}, nil},
	{"gemini-1.5-pro", GoogleDialer{}, nil},
	{"gemini-2.0-flash-exp", GoogleDialer{}, nil},
	{"ollama:(model name)", OllamaDialer{}, func(s string) bool { return strings.HasPrefix(s, "ollama:") }},
}

type NewLineTracer struct {
	w          io.Writer
	HasNewLine bool
}

func (t *NewLineTracer) Write(p []byte) (n int, err error) {
	if len(p) > 0 {
		t.HasNewLine = p[len(p)-1] == '\n'
	}
	return t.w.Write(p)
}

func IsJSONSchema(s string) bool {
	var x openai.ResponseFormatJSONSchemaJSONSchemaParam
	return json.Unmarshal([]byte(s), &x) == nil
}

func main() {
	flag := pflag.NewFlagSet("llawk", pflag.ExitOnError)
	flag.SortFlags = false
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "llawk - A CLI text operation tool using Large Language Models")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] INSTRUCT\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Environment Variables:")
		fmt.Fprintln(os.Stderr, "  Common:")
		fmt.Fprintln(os.Stderr, "    LLAWK_MODEL      Default model to use.")
		fmt.Fprintln(os.Stderr, "  for OpenAI models:")
		fmt.Fprintln(os.Stderr, "    OPENAI_API_KEY   API key.")
		fmt.Fprintln(os.Stderr, "    OPENAI_ORG_ID    Organization ID.")
		fmt.Fprintln(os.Stderr, "  for Google models:")
		fmt.Fprintln(os.Stderr, "    GEMINI_API_KEY   API key.")
		fmt.Fprintln(os.Stderr, "  for Ollama models:")
		fmt.Fprintln(os.Stderr, "    OLLAMA_HOST	  Hostname of the Ollama API.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  $ llawk -i en.txt -o ja.txt 'Translate it into Japanese'")
		fmt.Fprintln(os.Stderr, `  $ cat comments.txt | llawk -f json 'Guess the sentiment for each lines. Output in JSON format including an array named "sentiments".'`)
	}

	inputName := flag.StringP("input", "i", "-", "Input file. Use - for stdin.")
	outputName := flag.StringP("output", "o", "-", "Output file. Use - for stdout.")
	format := flag.StringP("format", "f", "text", `Output format. "text", "json", or JSON Schema string.`)
	model := flag.StringP("model", "m", DEFAULT_MODEL, `Model to use. Use "list" to list available models.`)
	verbose := flag.BoolP("verbose", "v", false, "Enable verbose output.")

	if err := flag.Parse(os.Args[1:]); errors.Is(err, pflag.ErrHelp) {
		os.Exit(0)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse flags: %v\n", err)
		os.Exit(1)
	}

	if *model == "list" {
		fmt.Println("Available models:")
		for _, m := range models {
			if m.Name == DEFAULT_MODEL {
				fmt.Println(" ", m.Name, "(default)")
			} else {
				fmt.Println(" ", m.Name)
			}
		}
		os.Exit(0)
	}

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	r := &Request{
		Instruct: flag.Arg(0),
		Verbose:  *verbose,
	}

	switch *format {
	case "text":
		r.Format = "plain text"
	case "json":
		r.Format = "JSON"
	default:
		if IsJSONSchema(*format) {
			r.Format = "JSON Schema"
			r.Schema = *format
		} else {
			fmt.Fprintf(os.Stderr, "Unsupported format: %s\n", *format)
			os.Exit(1)
		}
	}

	llm, err := NewLLM(*model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer llm.Close()

	if *inputName == "-" || *inputName == "" {
		bs, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read input: %v\n", err)
			os.Exit(1)
		}
		r.Input = string(bs)
		r.InputName = "<stdin>"
	} else {
		f, err := os.Open(*inputName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open input file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		bs, err := io.ReadAll(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read input: %v\n", err)
			os.Exit(1)
		}
		r.Input = string(bs)
		r.InputName = *inputName
	}

	var output io.Writer
	if *outputName == "-" || *outputName == "" {
		output = os.Stdout
		r.OutputName = "<stdout>"
	} else {
		f, err := os.Create(*outputName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create output file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		output = f
		r.OutputName = *outputName
	}

	if *verbose {
		fmt.Fprintln(os.Stderr, "Model:", *model)
		fmt.Fprintln(os.Stderr, "--- system ---")
		fmt.Fprintln(os.Stderr, r.SystemPrompt())
		fmt.Fprintf(os.Stderr, "--- user (input: %q) ---\n", r.InputName)
		fmt.Fprintln(os.Stderr, r.UserPrompt())
		if *outputName == "-" || *outputName == "" {
			fmt.Fprintf(os.Stderr, "--- result (output: %q) ---\n", r.OutputName)
		}
	}

	writer := &NewLineTracer{w: output}
	if err := llm.Invoke(context.Background(), writer, r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !writer.HasNewLine {
		fmt.Fprintln(output)
	}
}
