# llawk

AWK-like simple text processor but working with LLM (Large Language Model).
It allows you to easily perform text conversion, extraction, translation, and any other text processing tasks.

For example:

```shell
$ ls -l | llawk -f json "Extract file names and sizes"
{
  "files": [
    {
      "name": "LICENSE",
      "size": 1079
    },
    {
      "name": "README.md",
      "size": 3358
    },
    .
    .
    .
  ]
}
```

## Supported LLMs

- OpenAI
    - GPT-4o
    - GPT-4o-mini
    - o1
- Google
    - gemini-1.5-flash
    - gemini-1.5-pro
    - gemini-2.0-flash-exp
- Ollama
    - any models which supports chat completion


## Installation

To install `llawk`, use the following command:

```shell
$ go install github.com/llawk/llawk@latest
```

## Usage

llawk takes an instruction from the command line, and an input text from stdin or a file.

```shell
$ echo hello | llawk "Translate to Japanese"
       ^^^^^         ^^^^^^^^^^^^^^^^^^^^^^^
       Input         Instruction

$ llawk -i input.txt "Translate to Japanese"
           ^^^^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^
           InputFile Instruction
```

It supports a JSON output format using the `-f json` option:

```shell
$ ls -l | llawk -f json "Extract file names and sizes"
```

You can also specify a JSON Schema for the output:

```shell
$ echo hello | llawk -f '{"type":"object","properties":{"japanese":{"type":"string"},"french":{"type":"string"}}}' "Translate to Japanese"
```

You can specify the model using the `-m` option or the `LLAWK_MODEL` environment variable:

```shell
$ echo hello | llawk -m gemini-1.5-flash "Translate to Japanese"
```

To see the list of supported models, run `llawk -m list`.

### OpenAI models

OpenAI models require an API key to access.
Please set `OPENAI_API_KEY` environment variable before using OpenAI models.

```shell
$ export OPENAI_API_KEY=sk-xxxxxx
$ echo hello | llawk -m gpt-4o "Translate to Japanese"
```

You can specify the OpenAI API organization using the `OPENAI_ORG_ID` environment variable.

### Google models

Google models require an API key to access.
Please set `GEMINI_API_KEY` environment variable before using Google models.

```shell
$ export GEMINI_API_KEY=xxxxxx
$ echo hello | llawk -m gemini-1.5-flash "Translate to Japanese"
```

### Ollama models

You can use Ollama models without any special settings because Ollama does not require an API key.

Any model supported by Ollama that offers chat completion can be used.

```shell
$ echo hello | llawk -m ollama:llama3.2 "Translate to Japanese"
$ echo hello | llawk -m ollama:hf.co/elyza/Llama-3-ELYZA-JP-8B-GGUF "Translate to Japanese"
```

If you want to connect to a specific Ollama server, you can specify the server URL with the `OLLAMA_HOST` environment variable.

```shell
$ export OLLAMA_HOST=https://192.168.1.123:11434
$ echo hello | llawk -m ollama:llama3.2 "Translate to Japanese"
```
