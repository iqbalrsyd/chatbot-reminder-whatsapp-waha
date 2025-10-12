#!/bin/bash
# Stop WhatsApp AI Bot

echo "🛑 Stopping WhatsApp AI Bot..."

if lsof -i:8080 > /dev/null 2>&1; then
    fuser -k 8080/tcp
    echo "✅ Bot stopped"
else
    echo "⚠️  No bot running on port 8080"
fi
