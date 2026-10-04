chatgpt: Chat GPT console client in Golang
======================

[![GitHub license](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/kkdai/chatgpt/master/LICENSE) ![Go](https://github.com/kkdai/chatgpt/workflows/Go/badge.svg)

A Golang console client for ChatGPT (<https://chat.openai.com>) using GPT

Install
--------------

    go get -u -x github.com/kkdai/chatgpt

Request for API from OpenAI
---------------------

Request your OpenAPI key in [https://platform.openai.com/api-keys](https://platform.openai.com/api-keys)

Usage
---------------------

    export API_KEY=YOUR_KEY   # or OPENAI_API_KEY
    chatgpt

Options:

    -m, --model string     model to use (default "gpt-4o", env OPENAI_MODEL)
    -s, --system string    system prompt
        --timeout duration maximum time for one answer (default 5m0s, 0 for no limit)

The conversation keeps its history across questions. In-session commands:
`/system <prompt>`, `/reset`, `/help`, `quit` / `exit`. Press Ctrl+C while an
answer is streaming to interrupt it.

Snapshot
---------------

![](img/chatgpt.gif)

Contribute
---------------

Please open up an issue on GitHub before you put a lot efforts on pull request.
The code submitting to PR must be filtered with `gofmt`

License
---------------

This package is licensed under MIT license. See LICENSE for details.
