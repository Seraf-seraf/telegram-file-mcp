#!/usr/bin/env bash

set -euo pipefail

env_file=".env"

if [[ ! -f "$env_file" ]]; then
	printf 'Файл .env не найден. Создайте его на основе .env.example.\n' >&2
	exit 1
fi

bot_token="$(awk -F= '$1 == "TELEGRAM_BOT_TOKEN" { sub(/^[^=]*=/, ""); gsub(/^[[:space:]]+|[[:space:]]+$/, ""); if ($0 ~ /^".*"$/) { sub(/^"/, ""); sub(/"$/, "") } else if ($0 ~ /^\047.*\047$/) { sub(/^\047/, ""); sub(/\047$/, "") }; print; exit }' "$env_file")"

if [[ ! "$bot_token" =~ ^[0-9]+:[A-Za-z0-9_-]+$ ]]; then
	printf 'В .env задайте корректный TELEGRAM_BOT_TOKEN.\n' >&2
	exit 1
fi

if ! command -v curl >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then
	printf 'Для этой команды нужны curl и jq.\n' >&2
	exit 1
fi

if ! response="$(curl --silent --config - --max-time 15 --fail-with-body 2>/dev/null <<EOF
url = "https://api.telegram.org/bot${bot_token}/getUpdates?limit=100&timeout=0"
EOF
)"; then
	if [[ -z "$response" ]]; then
		printf 'Не удалось получить обновления Telegram. Проверьте сеть и настройки бота.\n' >&2
		exit 1
	fi
fi

if ! jq -e '.ok == true and (.result | type == "array")' >/dev/null 2>&1 <<<"$response"; then
	printf 'Telegram API не вернул обновления. Проверьте токен и отсутствие webhook у бота.\n' >&2
	exit 1
fi

chat_rows="$(jq -r '
  [.result[]
   | (.message // .edited_message // .channel_post // .edited_channel_post // empty)
   | .chat
   | select(.id != null)
   | {
       id: (.id | tostring),
       name: (.title // .username // ([.first_name, .last_name] | map(select(. != null and . != "")) | join(" ")) // "без имени")
     }
  ]
  | unique_by(.id)
  | to_entries[]
  | "\(.key + 1)\t\(.value.id)\t\(.value.name | gsub("[\\t\\r\\n]"; " "))"
' <<<"$response")"

if [[ -z "$chat_rows" ]]; then
	printf 'Чаты не найдены. Отправьте боту сообщение и запустите make chatid ещё раз.\n' >&2
	exit 1
fi

printf 'Выберите чат для TELEGRAM_CHAT_ID:\n'
while IFS=$'\t' read -r number chat_id chat_name; do
	printf '  %s) %s — %s\n' "$number" "$chat_id" "$chat_name"
done <<<"$chat_rows"

while true; do
	read -r -p 'Номер чата: ' choice
	if [[ "$choice" =~ ^[0-9]+$ ]] && selected_row="$(awk -F '\t' -v number="$choice" '$1 == number { print; exit }' <<<"$chat_rows")" && [[ -n "$selected_row" ]]; then
		IFS=$'\t' read -r _ selected_chat_id _ <<<"$selected_row"
		break
	fi
	printf 'Введите номер из списка.\n' >&2
done

current_chat_id="$(awk -F= '$1 == "TELEGRAM_CHAT_ID" { sub(/^[^=]*=/, ""); gsub(/^[[:space:]]+|[[:space:]]+$/, ""); if ($0 ~ /^".*"$/) { sub(/^"/, ""); sub(/"$/, "") } else if ($0 ~ /^\047.*\047$/) { sub(/^\047/, ""); sub(/\047$/, "") }; print; exit }' "$env_file")"

if [[ -n "$current_chat_id" ]]; then
	read -r -p "TELEGRAM_CHAT_ID уже задан. Заменить его на ${selected_chat_id}? [y/N] " confirm
	if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
		printf 'Настройка не изменена.\n'
		exit 0
	fi
fi

tmp_file="$(mktemp "${env_file}.XXXXXX")"
trap 'rm -f "$tmp_file"' EXIT
cp -p "$env_file" "$tmp_file"
awk -v chat_id="$selected_chat_id" '
  BEGIN { updated = 0 }
  /^TELEGRAM_CHAT_ID=/ {
    if (!updated) print "TELEGRAM_CHAT_ID=" chat_id
    updated = 1
    next
  }
  { print }
  END {
    if (!updated) print "TELEGRAM_CHAT_ID=" chat_id
  }
' "$env_file" >"$tmp_file"
mv "$tmp_file" "$env_file"
trap - EXIT

printf 'TELEGRAM_CHAT_ID обновлён в .env.\n'
