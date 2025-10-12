package internal

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ReminderManager manages reminders with SQLite database
type ReminderManager struct {
	db         *sql.DB
	wahaClient *WAHAClient
	stopChan   chan bool
}

// Reminder represents a scheduled reminder
type Reminder struct {
	ID           int
	ChatID       string
	Message      string
	ReminderTime time.Time
	CreatedAt    time.Time
	Notified     bool
	Cancelled    bool
}

// NewReminderManager creates a new reminder manager with database
func NewReminderManager(wahaClient *WAHAClient) *ReminderManager {
	db, err := sql.Open("sqlite3", "./reminders.db")
	if err != nil {
		log.Fatalf("❌ Failed to open database: %v", err)
	}

	// Create reminders table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS reminders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id TEXT NOT NULL,
		message TEXT NOT NULL,
		reminder_time DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		notified BOOLEAN DEFAULT 0,
		cancelled BOOLEAN DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_reminder_time ON reminders(reminder_time);
	CREATE INDEX IF NOT EXISTS idx_chat_id ON reminders(chat_id);
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("❌ Failed to create table: %v", err)
	}

	log.Println("✅ Reminder database initialized")

	return &ReminderManager{
		db:         db,
		wahaClient: wahaClient,
		stopChan:   make(chan bool),
	}
}

// AddReminder adds a new reminder to database
func (rm *ReminderManager) AddReminder(chatID, message string, reminderTime time.Time) error {
	query := `INSERT INTO reminders (chat_id, message, reminder_time) VALUES (?, ?, ?)`
	result, err := rm.db.Exec(query, chatID, message, reminderTime)
	if err != nil {
		return fmt.Errorf("failed to add reminder: %w", err)
	}

	id, _ := result.LastInsertId()
	log.Printf("✅ Reminder added: ID=%d, ChatID=%s, Time=%s, Message=%s",
		id, chatID, reminderTime.Format("2006-01-02 15:04:05"), message)

	return nil
}

// AddSmartReminder adds reminder using SmartTimeParser with automatic scheduling
func (rm *ReminderManager) AddSmartReminder(chatID string, input string, parser *SmartTimeParser) (*TimeResult, error) {
	// Parse time with smart parser
	result, err := parser.ParseSmartTime(input)
	if err != nil {
		return nil, err
	}

	// Add main reminder (5 minutes before target time)
	err = rm.AddReminder(chatID, result.Message, result.PreNotifyTime)
	if err != nil {
		return nil, fmt.Errorf("failed to add main reminder: %w", err)
	}

	// Add midnight notification if applicable (for tomorrow/lusa reminders)
	if result.MidnightNotifyTime != nil {
		midnightMsg := fmt.Sprintf("📅 Hari ini: %s", result.Message)
		err = rm.AddReminder(chatID, midnightMsg, *result.MidnightNotifyTime)
		if err != nil {
			log.Printf("⚠️  Failed to add midnight notification: %v", err)
		} else {
			log.Printf("🌙 Midnight notification scheduled for %s", result.MidnightNotifyTime.Format("2006-01-02 00:00"))
		}
	}

	return result, nil
}

// GetPendingReminders gets all pending reminders that should be sent
func (rm *ReminderManager) GetPendingReminders() ([]Reminder, error) {
	loc, _ := time.LoadLocation("Asia/Jakarta")

	query := `
		SELECT id, chat_id, message, reminder_time, created_at 
		FROM reminders 
		WHERE notified = 0 AND cancelled = 0 AND reminder_time <= datetime('now', 'localtime')
		ORDER BY reminder_time ASC
	`

	rows, err := rm.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query reminders: %w", err)
	}
	defer rows.Close()

	var reminders []Reminder
	for rows.Next() {
		var r Reminder
		var reminderTimeStr, createdAtStr string

		err := rows.Scan(&r.ID, &r.ChatID, &r.Message, &reminderTimeStr, &createdAtStr)
		if err != nil {
			log.Printf("❌ Error scanning row: %v", err)
			continue
		}

		// Parse time strings with WIB timezone
		r.ReminderTime, err = time.ParseInLocation("2006-01-02 15:04:05", reminderTimeStr, loc)
		if err != nil {
			log.Printf("⚠️  Failed to parse reminder_time '%s': %v", reminderTimeStr, err)
			r.ReminderTime = time.Now() // Fallback to current time
		}

		r.CreatedAt, err = time.ParseInLocation("2006-01-02 15:04:05", createdAtStr, loc)
		if err != nil {
			r.CreatedAt = time.Now()
		}

		reminders = append(reminders, r)
	}

	return reminders, nil
}

// MarkAsNotified marks a reminder as sent
func (rm *ReminderManager) MarkAsNotified(id int) error {
	query := `UPDATE reminders SET notified = 1 WHERE id = ?`
	_, err := rm.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to mark reminder as notified: %w", err)
	}
	return nil
}

// CancelReminder cancels a reminder
func (rm *ReminderManager) CancelReminder(id int) error {
	query := `UPDATE reminders SET cancelled = 1 WHERE id = ?`
	_, err := rm.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to cancel reminder: %w", err)
	}
	return nil
}

// GetUserReminders gets all active reminders for a user
func (rm *ReminderManager) GetUserReminders(chatID string) ([]Reminder, error) {
	loc, _ := time.LoadLocation("Asia/Jakarta")

	query := `
		SELECT id, chat_id, message, reminder_time, created_at 
		FROM reminders 
		WHERE chat_id = ? AND notified = 0 AND cancelled = 0
		ORDER BY reminder_time ASC
	`

	rows, err := rm.db.Query(query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user reminders: %w", err)
	}
	defer rows.Close()

	var reminders []Reminder
	for rows.Next() {
		var r Reminder
		var reminderTimeStr, createdAtStr string

		err := rows.Scan(&r.ID, &r.ChatID, &r.Message, &reminderTimeStr, &createdAtStr)
		if err != nil {
			log.Printf("❌ Error scanning row: %v", err)
			continue
		}

		r.ReminderTime, err = time.ParseInLocation("2006-01-02 15:04:05", reminderTimeStr, loc)
		if err != nil {
			log.Printf("⚠️  Failed to parse reminder_time: %v", err)
			r.ReminderTime = time.Now()
		}

		r.CreatedAt, err = time.ParseInLocation("2006-01-02 15:04:05", createdAtStr, loc)
		if err != nil {
			r.CreatedAt = time.Now()
		}

		reminders = append(reminders, r)
	}

	return reminders, nil
}

// StartScheduler starts the reminder scheduler
func (rm *ReminderManager) StartScheduler() {
	log.Println("⏰ Reminder scheduler started (checks every 30 seconds)")

	go func() {
		ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				rm.checkAndSendReminders()
			case <-rm.stopChan:
				log.Println("⏹️  Reminder scheduler stopped")
				return
			}
		}
	}()
}

// StopScheduler stops the reminder scheduler
func (rm *ReminderManager) StopScheduler() {
	close(rm.stopChan)
	rm.db.Close()
}

// checkAndSendReminders checks and sends pending reminders
func (rm *ReminderManager) checkAndSendReminders() {
	reminders, err := rm.GetPendingReminders()
	if err != nil {
		log.Printf("❌ Error getting pending reminders: %v", err)
		return
	}

	if len(reminders) == 0 {
		return
	}

	log.Printf("📬 Found %d pending reminder(s)", len(reminders))

	for _, reminder := range reminders {
		// Format time - use current time if reminder time is zero
		displayTime := reminder.ReminderTime
		if displayTime.IsZero() {
			displayTime = time.Now()
		}

		message := fmt.Sprintf("⏰ **REMINDER**\n\n📝 %s\n\n🕐 Waktu target: %s",
			reminder.Message,
			displayTime.Format("Monday, 02 Jan 2006 15:04"))

		err := rm.wahaClient.SendText(reminder.ChatID, message)
		if err != nil {
			log.Printf("❌ Failed to send reminder ID=%d: %v", reminder.ID, err)
			continue
		}

		// Mark as notified
		if err := rm.MarkAsNotified(reminder.ID); err != nil {
			log.Printf("❌ Failed to mark reminder ID=%d as notified: %v", reminder.ID, err)
		} else {
			log.Printf("✅ Reminder sent and marked: ID=%d, ChatID=%s", reminder.ID, reminder.ChatID)
		}
	}
}

// ParseReminderTime parses user input to extract reminder time
func ParseReminderTime(message string) (time.Time, string, error) {
	now := time.Now()
	message = strings.ToLower(strings.TrimSpace(message))

	// Remove "ingatkan", "saya", "aku", etc.
	message = strings.ReplaceAll(message, "ingatkan", "")
	message = strings.ReplaceAll(message, "saya", "")
	message = strings.ReplaceAll(message, "aku", "")
	message = strings.ReplaceAll(message, "untuk", "")
	message = strings.TrimSpace(message)

	var reminderTime time.Time
	var reminderMessage string

	// Parse different time formats
	if strings.Contains(message, "besok") {
		// Tomorrow at specified time or midnight
		message = strings.ReplaceAll(message, "besok", "")
		message = strings.TrimSpace(message)

		// Try to extract time like "jam 3 sore", "15:00", "3pm"
		hour, minute, hasTime := extractTime(message)

		if hasTime {
			// Tomorrow at specified time
			reminderTime = time.Date(now.Year(), now.Month(), now.Day()+1, hour, minute, 0, 0, now.Location())
			// Remove time from message
			reminderMessage = removeTimeFromMessage(message)
		} else {
			// Tomorrow at midnight (00:00)
			reminderTime = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			reminderMessage = message
		}
	} else if strings.Contains(message, "hari ini") || strings.Contains(message, "jam") {
		// Today at specified time
		message = strings.ReplaceAll(message, "hari ini", "")
		message = strings.TrimSpace(message)

		hour, minute, hasTime := extractTime(message)

		if !hasTime {
			return time.Time{}, "", fmt.Errorf("tidak dapat menemukan waktu yang valid")
		}

		reminderTime = time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

		// If time has passed today, move to tomorrow
		if reminderTime.Before(now) {
			reminderTime = reminderTime.AddDate(0, 0, 1)
		}

		reminderMessage = removeTimeFromMessage(message)
	} else {
		// Try to extract time directly
		hour, minute, hasTime := extractTime(message)

		if !hasTime {
			return time.Time{}, "", fmt.Errorf("format waktu tidak dikenali. Gunakan: 'besok jam 3', 'hari ini jam 14:00', 'jam 20.30'")
		}

		reminderTime = time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

		// If time has passed today, move to tomorrow
		if reminderTime.Before(now) {
			reminderTime = reminderTime.AddDate(0, 0, 1)
		}

		reminderMessage = removeTimeFromMessage(message)
	}

	// If no message extracted, use default
	if strings.TrimSpace(reminderMessage) == "" {
		reminderMessage = "Reminder"
	}

	return reminderTime, reminderMessage, nil
}

// extractTime extracts hour and minute from text
func extractTime(text string) (hour int, minute int, found bool) {
	text = strings.ToLower(text)

	// Try format: jam 20.30, jam 20:30, jam 20
	if strings.Contains(text, "jam") {
		parts := strings.Fields(text)
		for i, part := range parts {
			if part == "jam" && i+1 < len(parts) {
				timeStr := parts[i+1]
				// Try HH:MM or HH.MM
				if strings.Contains(timeStr, ":") || strings.Contains(timeStr, ".") {
					timeStr = strings.ReplaceAll(timeStr, ".", ":")
					var h, m int
					if _, err := fmt.Sscanf(timeStr, "%d:%d", &h, &m); err == nil {
						if h >= 0 && h < 24 && m >= 0 && m < 60 {
							return h, m, true
						}
					}
				} else {
					// Try just hour
					var h int
					if _, err := fmt.Sscanf(timeStr, "%d", &h); err == nil {
						if h >= 0 && h < 24 {
							return h, 0, true
						}
					}
				}
			}
		}
	}

	// Try format: 20:30, 20.30, 2030
	parts := strings.Fields(text)
	for _, part := range parts {
		if strings.Contains(part, ":") || strings.Contains(part, ".") {
			timeStr := strings.ReplaceAll(part, ".", ":")
			var h, m int
			if _, err := fmt.Sscanf(timeStr, "%d:%d", &h, &m); err == nil {
				if h >= 0 && h < 24 && m >= 0 && m < 60 {
					return h, m, true
				}
			}
		}
	}

	// Try words: "sore" (afternoon), "malam" (evening), "pagi" (morning)
	if strings.Contains(text, "sore") {
		// Default afternoon time: 15:00
		return 15, 0, true
	} else if strings.Contains(text, "malam") {
		// Default evening time: 20:00
		return 20, 0, true
	} else if strings.Contains(text, "pagi") {
		// Default morning time: 07:00
		return 7, 0, true
	}

	return 0, 0, false
}

// removeTimeFromMessage removes time-related words from message
func removeTimeFromMessage(text string) string {
	// Remove time patterns
	text = strings.ReplaceAll(text, "jam", "")
	text = strings.ReplaceAll(text, "pagi", "")
	text = strings.ReplaceAll(text, "sore", "")
	text = strings.ReplaceAll(text, "malam", "")

	// Remove time formats like 20:30, 20.30
	words := strings.Fields(text)
	var cleanWords []string
	for _, word := range words {
		if !strings.Contains(word, ":") && !strings.Contains(word, ".") {
			// Check if it's not a pure number (time)
			var num int
			if _, err := fmt.Sscanf(word, "%d", &num); err != nil || num > 24 {
				cleanWords = append(cleanWords, word)
			}
		}
	}

	return strings.TrimSpace(strings.Join(cleanWords, " "))
}
