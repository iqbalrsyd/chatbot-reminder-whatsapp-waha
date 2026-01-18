package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"whatsapp-ai-bot-waha/internal"

	"github.com/joho/godotenv"
)

// WebhookPayload represents incoming message from WAHA
type WebhookPayload struct {
	Event   string `json:"event"`
	Session string `json:"session"`
	Payload struct {
		From string `json:"from"`
		Body string `json:"body"`
		Type string `json:"type"`
	} `json:"payload"`
}

var (
	wahaClient      *internal.WAHAClient
	geminiClient    *internal.GeminiClient
	reminderManager *internal.ReminderManager
	timeParser      *internal.SmartTimeParser
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file found, using environment variables")
	}

	// Get configuration from environment
	port := getEnv("PORT", "8080")
	wahaURL := getEnv("WAHA_URL", "http://localhost:3000")
	session := getEnv("SESSION", "mybot")
	geminiKey := getEnv("GEMINI_API_KEY", "")
	wahaAPIKey := getEnv("WAHA_API_KEY", "") // WAHA API Key

	// Initialize clients
	wahaClient = internal.NewWAHAClient(wahaURL, session, wahaAPIKey)
	geminiClient = internal.NewGeminiClient(geminiKey)
	log.Printf("🤖 AI Provider: Gemini (gemini-2.5-flash)")

	reminderManager = internal.NewReminderManager(wahaClient)

	// Initialize SmartTimeParser with Gemini for LLM fallback
	timeParser = internal.NewSmartTimeParser(geminiClient)
	log.Println("🕐 SmartTimeParser initialized with LLM fallback")

	// Start reminder scheduler
	reminderManager.StartScheduler()
	defer reminderManager.StopScheduler()

	// Setup routes
	http.HandleFunc("/webhook", webhookHandler)
	http.HandleFunc("/health", healthHandler)

	// Start server
	log.Printf("🚀 WhatsApp AI Bot started on port %s", port)
	log.Printf("📡 WAHA URL: %s", wahaURL)
	log.Println("✅ Ready to receive messages!")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}

// webhookHandler handles incoming messages from WAHA
func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("❌ Error reading request body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse webhook payload
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("❌ Error parsing JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Log incoming message
	chatID := payload.Payload.From
	messageText := payload.Payload.Body

	log.Printf("📨 Received message from %s: %s", chatID, messageText)

	// Filter: Only respond to private chats (not groups)
	// Group chat IDs end with @g.us, private chats end with @c.us
	if strings.HasSuffix(chatID, "@g.us") {
		log.Printf("⏭️  Ignoring group message from %s", chatID)
		return
	}

	// Process message asynchronously to avoid blocking webhook response
	go processMessage(chatID, messageText)

	// Send immediate response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}

// processMessage processes the incoming message and decides action
func processMessage(chatID, messageText string) {
	// Check if message contains reminder keywords
	lowerText := strings.ToLower(messageText)
	trimmed := strings.TrimSpace(messageText)

	// Welcome/Help command
	if trimmed == "/start" || trimmed == "/help" || lowerText == "help" || lowerText == "bantuan" {
		handleWelcomeMessage(chatID)
		return
	}

	// List reminders command
	if lowerText == "list reminder" || lowerText == "daftar reminder" ||
		lowerText == "reminder saya" || lowerText == "cek reminder" ||
		trimmed == "/list" {
		handleListReminders(chatID)
		return
	}

	// Support various reminder keywords: ingatkan, ingetin, reminder, set reminder
	if strings.Contains(lowerText, "ingatkan") ||
		strings.Contains(lowerText, "ingetin") ||
		strings.Contains(lowerText, "ingetkan") || // common typo
		strings.Contains(lowerText, "reminder") {
		handleReminderRequest(chatID, messageText)
	} else if strings.Contains(lowerText, "jam berapa") || strings.Contains(lowerText, "waktu") {
		handleTimeRequest(chatID)
	} else {
		handleAIRequest(chatID, messageText)
	}
}

// handleReminderRequest creates a new reminder with SmartTimeParser
func handleReminderRequest(chatID, messageText string) {
	log.Printf("📝 Processing reminder request from %s", chatID)

	// Use SmartTimeParser to parse time and create reminder
	result, err := reminderManager.AddSmartReminder(chatID, messageText, timeParser)

	if err != nil {
		// Send error message with examples
		errorResponse := fmt.Sprintf(`❌ Maaf, tidak dapat memahami waktu yang dimaksud.

💡 Contoh yang benar:
• "ingatkan 5 menit lagi minum obat"
• "ingatkan 2 jam lagi meeting"
• "ingatkan besok jam 15:30 konsultasi"
• "ingatkan besok pagi ke kampus"
• "ingatkan lusa jam 9 bangun"
• "ingatkan hari ini jam 20:30 solat"
• "ingatkan jam 14.00 makan siang"

Error: %s`, err.Error())
		wahaClient.SendText(chatID, errorResponse)
		log.Printf("❌ Failed to parse reminder: %v", err)
		return
	}

	// Send detailed confirmation
	timeStr := result.TargetTime.Format("Monday, 02 Jan 2006 15:04")
	response := fmt.Sprintf(`✅ **Reminder berhasil ditambahkan!**

📝 Pesan: %s
⏰ Waktu target: %s
🔔 Notifikasi: %s
📊 Parsing method: %s`,
		result.Message,
		timeStr,
		result.PreNotifyTime.Format("02 Jan 15:04"),
		result.ParseMethod)

	// Add midnight notification info if applicable
	if result.MidnightNotifyTime != nil {
		response += fmt.Sprintf("\n🌙 Midnight reminder: %s", result.MidnightNotifyTime.Format("02 Jan 00:00"))
	}

	err = wahaClient.SendText(chatID, response)
	if err != nil {
		log.Printf("❌ Error sending reminder confirmation: %v", err)
	} else {
		log.Printf("✅ Reminder confirmed to %s (method: %s)", chatID, result.ParseMethod)
	}
}

// handleTimeRequest responds with current time
func handleTimeRequest(chatID string) {
	log.Printf("⏰ Processing time request from %s", chatID)

	currentTime := time.Now().Format("15:04:05")
	currentDate := time.Now().Format("Monday, 02 January 2006")

	response := fmt.Sprintf("🕐 Sekarang jam: %s\n📅 Tanggal: %s", currentTime, currentDate)
	err := wahaClient.SendText(chatID, response)
	if err != nil {
		log.Printf("❌ Error sending time response: %v", err)
	}
}

// handleWelcomeMessage sends welcome message with bot instructions
func handleWelcomeMessage(chatID string) {
	log.Printf("👋 Sending welcome message to %s", chatID)

	welcome := `*Halo! Selamat datang!* 👋

Saya adalah *AI Bot with Smart Reminder* yang bisa membantu kamu dengan:

*🤖 Chat AI*
Tanya apa saja! Saya powered by Gemini AI.
_Contoh:_ "Jelaskan apa itu API", "Resep nasi goreng"

*⏰ Smart Reminder*
Set reminder dengan natural language!
_Trigger:_ ketik "ingatkan" atau "ingetin"

*Contoh Reminder:*
• "ingatkan 5 menit lagi minum obat"
• "ingatkan 2 jam lagi meeting"
• "ingatkan besok jam 15:30 konsultasi"
• "ingatkan jam 10 malam ini keluar"
• "ingatkan lusa pagi ke kampus"

*📋 Fitur Lainnya:*
• *List reminder* - Lihat semua reminder aktif
• *Jam berapa* - Cek waktu sekarang
• */help* - Tampilkan pesan ini

*✨ Fitur Smart Reminder:*
🔔 Pre-notification 5 menit sebelumnya
🌙 Midnight alert untuk reminder besok
🧠 Support typo & natural language
💬 LLM fallback untuk parsing kompleks

_Silakan mulai chat atau set reminder!_ 😊`

	err := wahaClient.SendText(chatID, welcome)
	if err != nil {
		log.Printf("❌ Error sending welcome message: %v", err)
	}
}

// handleListReminders shows all active reminders for the user
func handleListReminders(chatID string) {
	log.Printf("📋 Fetching reminders for %s", chatID)

	reminders, err := reminderManager.GetUserReminders(chatID)
	if err != nil {
		log.Printf("❌ Error getting reminders: %v", err)
		wahaClient.SendText(chatID, "❌ Maaf, gagal mengambil daftar reminder.")
		return
	}

	if len(reminders) == 0 {
		response := `📋 *Daftar Reminder Kamu*

Kamu belum punya reminder aktif.

💡 _Buat reminder baru dengan:_
"ingatkan [waktu] [pesan]"

_Contoh:_
• "ingatkan besok jam 9 meeting"
• "ingatkan 1 jam lagi istirahat"`

		wahaClient.SendText(chatID, response)
		return
	}

	// Build reminder list
	response := fmt.Sprintf("📋 *Daftar Reminder Kamu* (%d active)\n\n", len(reminders))

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	for i, reminder := range reminders {
		// Calculate time remaining
		duration := reminder.ReminderTime.Sub(now)
		timeRemaining := formatDuration(duration)

		// Format reminder time
		timeStr := reminder.ReminderTime.Format("Mon, 02 Jan 15:04")

		// Clean message (remove "📅 Hari ini:" prefix if exists)
		message := strings.TrimPrefix(reminder.Message, "📅 Hari ini: ")

		response += fmt.Sprintf("*%d.* %s\n⏰ %s (%s)\n\n",
			i+1, message, timeStr, timeRemaining)
	}

	response += "_Ketik 'ingatkan [waktu] [pesan]' untuk tambah reminder baru_"

	err = wahaClient.SendText(chatID, response)
	if err != nil {
		log.Printf("❌ Error sending reminders list: %v", err)
	}
}

// formatDuration formats duration to human readable string
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "sudah lewat"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours >= 24 {
		days := hours / 24
		if days == 1 {
			return "besok"
		}
		return fmt.Sprintf("%d hari lagi", days)
	}

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%d jam %d menit lagi", hours, minutes)
		}
		return fmt.Sprintf("%d jam lagi", hours)
	}

	if minutes > 0 {
		return fmt.Sprintf("%d menit lagi", minutes)
	}

	seconds := int(d.Seconds())
	return fmt.Sprintf("%d detik lagi", seconds)
}

// handleAIRequest sends message to AI and replies with response
func handleAIRequest(chatID, messageText string) {
	log.Printf("🤖 Processing AI request from %s", chatID)

	aiResponse, err := geminiClient.CallGemini(messageText)
	if err != nil {
		log.Printf("❌ Error calling Gemini: %v", err)

		// Send error message to user
		errorMsg := "Maaf, saya sedang mengalami kendala. Coba lagi nanti ya! 😅"
		wahaClient.SendText(chatID, errorMsg)
		return
	}

	// Send AI response to user
	err = wahaClient.SendText(chatID, aiResponse)
	if err != nil {
		log.Printf("❌ Error sending AI response: %v", err)
	}
}

// healthHandler handles health check requests
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "healthy",
		"service":     "whatsapp-ai-bot",
		"ai_provider": "gemini",
		"model":       "gemini-2.5-flash",
	})
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
