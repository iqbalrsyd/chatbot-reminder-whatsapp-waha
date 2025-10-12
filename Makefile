.PHONY: help build run stop restart test clean logs status dev

help:
@echo "🤖 WhatsApp AI Bot - Available Commands:"
@echo ""
@echo "  make build     - Build the bot binary"
@echo "  make run       - Run the bot in foreground"
@echo "  make dev       - Run bot in background"
@echo "  make stop      - Stop the running bot"
@echo "  make restart   - Restart the bot"
@echo "  make logs      - Show bot logs"
@echo "  make status    - Check bot status"
@echo "  make test      - Run unit tests"
@echo "  make clean     - Clean artifacts"
@echo ""

build:
@echo "🔨 Building bot..."
@go build -o bot main.go
@echo "✅ Build complete"

run:
@echo "🚀 Starting bot..."
@go run main.go

dev:
@echo "🚀 Starting bot in background..."
@fuser -k 8080/tcp 2>/dev/null || true
@sleep 2
@nohup go run main.go > /tmp/bot.log 2>&1 &
@sleep 2
@echo "✅ Bot started. Logs: tail -f /tmp/bot.log"

stop:
@echo "🛑 Stopping bot..."
@fuser -k 8080/tcp 2>/dev/null || echo "No bot running"

restart: stop dev

logs:
@tail -f /tmp/bot.log

status:
@echo "📊 Bot Status:"
@if lsof -i:8080 > /dev/null 2>&1; then \
echo "✅ Bot is RUNNING"; \
echo ""; \
lsof -i:8080; \
else \
echo "❌ Bot is NOT running"; \
fi

test:
@echo "🧪 Running tests..."
@go test ./internal/... -v

clean:
@echo "🧹 Cleaning..."
@rm -f bot /tmp/bot.log
@echo "✅ Cleaned"
