#!/bin/bash
# Start WhatsApp AI Bot

echo "🤖 Starting WhatsApp AI Bot..."

# Check if already running
if lsof -i:8080 > /dev/null 2>&1; then
    echo "⚠️  Bot is already running on port 8080!"
    echo "   Use ./stop.sh to stop it first"
    exit 1
fi

# Check .env file
if [ ! -f .env ]; then
    echo "❌ .env file not found!"
    echo "   Copy .env.example to .env and configure it"
    exit 1
fi

# Start bot in background
echo "🚀 Starting bot in background..."
nohup go run main.go > /tmp/bot.log 2>&1 &

# Wait and check if started successfully
sleep 3

if lsof -i:8080 > /dev/null 2>&1; then
    echo "✅ Bot started successfully!"
    echo ""
    echo "📋 View logs: tail -f /tmp/bot.log"
    echo "🛑 Stop bot:  ./stop.sh"
else
    echo "❌ Failed to start bot. Check logs:"
    tail -20 /tmp/bot.log
    exit 1
fi
