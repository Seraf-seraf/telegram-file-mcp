# telegram-file-mcp

Небольшой MCP-сервис отправляет переданный файл в один заранее настроенный Telegram-чат.

```text
ChatGPT → MCP/Vercel → Telegram Bot API
```

## Конфигурация

- `TELEGRAM_BOT_TOKEN` — токен Telegram-бота.
- `TELEGRAM_CHAT_ID` — единственный чат-получатель.
- `MAX_FILE_BYTES` — лимит файла, по умолчанию 20 MiB; максимум 50 MiB.

## Локальный запуск

```sh
export TELEGRAM_BOT_TOKEN=...
export TELEGRAM_CHAT_ID=...
go run ./cmd/server
```

MCP endpoint: `http://localhost:8080/api/mcp` (`PORT` меняет порт локального сервера).

Проверки: `make fmt-check vet test test-race build`.

## Vercel

Импортируйте репозиторий в Vercel, добавьте `TELEGRAM_BOT_TOKEN` и `TELEGRAM_CHAT_ID` (при необходимости `MAX_FILE_BYTES`), затем выполните deploy. Vercel запускает Go backend из `cmd/server/main.go`; MCP endpoint будет доступен по `/api/mcp`. Подключите его через [MCP Inspector](https://github.com/modelcontextprotocol/inspector), проверьте `tools/list` и вызов инструмента. Vercel может обращаться к Telegram Bot API напрямую; Telegram proxy/VPN сервису не нужен.

## Ограничения MVP

MCP endpoint не использует OAuth; получатель жёстко задан конфигурацией. Endpoint предназначен для личного developer-mode использования. OAuth 2.1 нужен отдельной задачей перед публичной публикацией или многопользовательским использованием.

MCP server работает независимо от ChatGPT. End-to-end вызов write tool из ChatGPT зависит от доступности custom MCP write-actions для конкретного аккаунта и rollout.
