#!/bin/bash
# Setup WhatsApp AI Bot

echo "🔧 Setting up WhatsApp AI Bot..."
echo ""

# Check Go installation
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed!"
    echo "   Install Go from: https://go.dev/dl/"
    exit 1
fi
echo "✅ Go $(go version | awk '{print $3}') installed"

# Check .env file
if [ ! -f .env ]; then
    echo "📝 Creating .env file from template..."
    cp .env.example .env
    echo "✅ .env file created"
    echo ""
    echo "⚠️  Please edit .env and configure:"
    echo "   - GEMINI_API_KEY (get from https://aistudio.google.com/app/apikey)"
    echo "   - WAHA_URL (your WAHA instance)"
fi

# Install dependencies
echo ""
echo "📦 Installing dependencies..."
go mod download
echo "✅ Dependencies installed"

# Build bot
echo ""
echo "🔨 Building bot..."
go build -o bot main.go
echo "✅ Bot built successfully"

echo ""
echo "✅ Setup complete!"
echo ""
echo "📋 Next steps:"
echo "1. Edit .env and add your GEMINI_API_KEY"
echo "2. Start WAHA: docker run -d -p 3000:3000 devlikeapro/waha"
echo "3. Start bot: ./start.sh"
echo ""
