# 🤖 WhatsApp AI Bot with Smart Reminder

WhatsApp bot powered by **Gemini AI** with intelligent **Smart Reminder** system that understands natural language.

[![Go Version](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org)
[![Gemini](https://img.shields.io/badge/AI-Gemini%202.5%20Flash-orange.svg)](https://ai.google.dev/)
[![WAHA](https://img.shields.io/badge/WhatsApp-WAHA-green.svg)](https://waha.devlike.pro/)

## ✨ Features

### 🤖 **AI Chat (Gemini)**
- Natural conversation with Google Gemini 2.5 Flash
- WhatsApp-formatted responses (bold, italic, lists)
- Context-aware and helpful

### ⏰ **Smart Reminder System**
- **Natural Language Parsing** - "ingatkan jam 10 malam ini keluar"
- **Multi-format Support**:
  - Relative: "5 menit lagi", "2 jam lagi"
  - Absolute: "jam 20:30", "besok jam 9"
  - Day periods: "pagi" (07:00), "sore" (15:00), "malam" (20:00)
  - Future dates: "besok", "lusa"
- **Typo Tolerance** - Fuzzy matching with Levenshtein distance
- **LLM Fallback** - Gemini AI for complex time expressions
- **Auto Pre-notifications** - 5 minutes before target time
- **Midnight Alerts** - For tomorrow/future reminders
- **List Management** - View all active reminders

### 📋 **Additional Features**
- `/start` or `help` - Show welcome message with guide
- `list reminder` - View all active reminders with countdown
- `jam berapa` - Check current time

## 🚀 Quick Start

### Prerequisites
- Go 1.22 or higher
- Docker (for WAHA)
- Gemini API Key ([Get free key](https://aistudio.google.com/app/apikey))

### Installation

1. **Clone repository**
```bash
git clone <your-repo-url>
cd whatsapp-ai-bot-waha
```

2. **Run setup**
```bash
./setup.sh
```

3. **Configure environment**
```bash
nano .env
```
Add your `GEMINI_API_KEY`

4. **Start WAHA container**
```bash
docker run -d -p 3000:3000 --name waha devlikeapro/waha
```

5. **Start bot**
```bash
./start.sh
# or
make dev
```

## 📖 Usage

### Set Reminders

**Basic formats:**
```
ingatkan 5 menit lagi minum obat
ingetin 2 jam lagi meeting
ingatkan jam 20:30 solat isya
```

**Natural language:**
```
ingatkan jam 10 malam ini keluar
ingatkan besok jam 15:30 konsultasi
ingatkan besok pagi ke kampus
ingatkan lusa jam 9 bangun
```

**Supported keywords:** `ingatkan`, `ingetin`, `ingetkan`, `reminder`

### List Reminders
```
list reminder
daftar reminder
cek reminder
/list
```

### AI Chat
Just type anything else and the bot will respond!
```
Jelaskan apa itu machine learning
Resep nasi goreng
Apa ibukota Indonesia?
```

### Help
```
/start
/help
help
bantuan
```

## 🛠️ Development

### Make Commands
```bash
make help      # Show all commands
make build     # Build binary
make dev       # Run in background
make stop      # Stop bot
make restart   # Restart bot
make logs      # View logs
make status    # Check status
make test      # Run tests
make clean     # Clean artifacts
```

### Manual Scripts
```bash
./setup.sh     # Initial setup
./start.sh     # Start bot
./stop.sh      # Stop bot
```

## 📁 Project Structure

```
.
├── main.go                 # Entry point
├── internal/
│   ├── gemini.go          # Gemini AI client
│   ├── timeparser.go      # Smart time parser
│   ├── reminder.go        # Reminder manager
│   ├── waha.go           # WhatsApp API client
│   └── *_test.go         # Unit tests
├── reminders.db          # SQLite database
├── .env                  # Configuration
├── Makefile             # Build automation
└── README.md           # This file
```

## 🔧 Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `WAHA_URL` | WAHA instance URL | `http://localhost:3000` |
| `SESSION` | WhatsApp session name | `mybot` |
| `GEMINI_API_KEY` | Gemini API key | Required |

## 🧪 Testing

Run unit tests:
```bash
make test
# or
go test ./internal/... -v
```

Test specific feature:
```bash
go test ./internal -run TestSmartTimeParser -v
```

## 📊 Database

View reminders:
```bash
sqlite3 reminders.db "SELECT * FROM reminders ORDER BY id DESC LIMIT 10;"
```

Backup database:
```bash
cp reminders.db reminders.db.backup
```

## 🐛 Troubleshooting

**Bot not responding?**
- Check logs: `make logs` or `tail -f /tmp/bot.log`
- Verify WAHA is running: `docker ps`
- Check bot status: `make status`

**Gemini errors?**
- Verify API key in `.env`
- Check API quota: [Google AI Studio](https://aistudio.google.com/)

**Reminder not firing?**
- Check database: `sqlite3 reminders.db "SELECT * FROM reminders;"`
- Verify timezone (WIB/Asia/Jakarta)
- Check scheduler logs

## 📝 Example Commands

```bash
# Setup and start
./setup.sh
./start.sh

# Check status
make status

# View logs
make logs

# Stop bot
./stop.sh

# Restart after code changes
make restart

# Clean and rebuild
make clean
make build
```

## 🤝 Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create feature branch
3. Commit changes
4. Push to branch
5. Open pull request

## 📄 License

MIT License - feel free to use for personal or commercial projects!

## 🙏 Acknowledgments

- [WAHA](https://waha.devlike.pro/) - WhatsApp HTTP API
- [Google Gemini](https://ai.google.dev/) - AI model
- [Go SQLite](https://github.com/mattn/go-sqlite3) - Database driver

## 📧 Support

For issues or questions:
- Open an issue on GitHub
- Check logs with `make logs`
- Review documentation files

---

**Made with ❤️ using Go, Gemini AI, and WAHA**
