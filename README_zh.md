chatgpt: 以 Golang 撰寫的 Chat GPT 終端機客戶端
======================

[![GitHub license](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/kkdai/chatgpt/master/LICENSE) ![Go](https://github.com/kkdai/chatgpt/workflows/Go/badge.svg)

[English](README.md) | 中文

使用 GPT 的 ChatGPT (<https://chat.openai.com>) Golang 終端機客戶端

功能
--------------

- 使用 Chat Completions API，支援串流輸出與多輪對話記憶
- 可選擇模型（`--model` / `OPENAI_MODEL`），並可自訂 base URL（`--base-url` / `OPENAI_BASE_URL`），相容 Azure OpenAI 與其他 OpenAI 相容服務
- 非互動模式與管線輸入（`-p`）
- `/save`、`/load` 對話存檔與讀取（JSON）、`/system`、`/reset`
- 顯示 token 用量與預估費用、程式碼區塊上色、Ctrl+C 中斷、請求逾時設定

安裝
--------------

    go get -u -x github.com/kkdai/chatgpt

申請 OpenAI API Key
---------------------

請至 [https://platform.openai.com/api-keys](https://platform.openai.com/api-keys) 申請 API key。

使用方式
---------------------

    export API_KEY=YOUR_KEY   # 或 OPENAI_API_KEY
    chatgpt

選項：

    -m, --model string     使用的模型（預設 "gpt-4o"，環境變數 OPENAI_MODEL）
    -s, --system string    system prompt
    -p, --prompt string    送出單一 prompt 後結束
        --base-url string  OpenAI 相容 API 的 base URL（環境變數 OPENAI_BASE_URL）
        --timeout duration 單次回答的最長時間（預設 5m0s，0 表示不限制）
        --reasoning-effort string  o-series 模型的推理程度：low、medium 或 high（環境變數 OPENAI_REASONING_EFFORT）
        --json-schema string       指定結構化輸出的 JSON Schema

單次或管線使用時，可用 `-p` 傳入 prompt；管線輸入的文字會附加在該 prompt 之後。
沒有 `-p` 時，管線輸入會被當成單一 prompt 送出：

    cat file.txt | chatgpt -p "請摘要這份文件"
    echo "什麼是 Go？" | chatgpt

互動模式下會保留對話歷史。對話中可用的指令：`/system <prompt>`、`/reset`、
`/save <file>`、`/load <file>`、`/image <file>`、`/help`、`quit` / `exit`。
`/image <file>` 會將本機圖片附加至下一個問題，適用於支援視覺功能的模型。
對話檔使用 JSON 格式；`/save` 不會覆蓋既有檔案，除非使用 `/save --force <file>`。
回答串流輸出時按 Ctrl+C 可中斷。

### 推理、圖片與結構化輸出

- 使用 `--reasoning-effort low|medium|high` 設定 o-series 模型的推理程度，也可透過 `OPENAI_REASONING_EFFORT` 環境變數設定。
- 在互動模式輸入 `/image <file>`，可將本機圖片附加至下一個問題；請使用支援視覺功能的模型。
- 使用 `--json-schema '<JSON Schema>'` 指定結構化輸出的格式。

當 API 回報 token 用量時，會連同預估費用輸出到 stderr（支援 GPT-4o 與 GPT-4.1 系列）。
預估值依公開的每 token 價格計算，可能與各服務商實際計價不同。輸出至終端機時程式碼區塊會上色；
重新導向的輸出則維持純文字。

Snapshot
---------------

![](img/chatgpt.gif)

貢獻
---------------

在投入大量心力撰寫 PR 前，請先在 GitHub 開 issue 討論。
提交的程式碼必須通過 `gofmt` 格式化。

授權
---------------

本專案採用 MIT 授權，詳見 LICENSE。
