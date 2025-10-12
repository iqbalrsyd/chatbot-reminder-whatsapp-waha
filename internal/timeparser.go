package internal

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// SmartTimeParser handles intelligent time parsing with fuzzy matching and LLM fallback
type SmartTimeParser struct {
	geminiClient *GeminiClient // For LLM fallback
	location     *time.Location
}

// TimeResult contains parsed time and extracted message
type TimeResult struct {
	TargetTime         time.Time  // Main reminder time
	Message            string     // Extracted reminder message
	PreNotifyTime      time.Time  // 5 minutes before
	MidnightNotifyTime *time.Time // Midnight notification (if applicable)
	ParseMethod        string     // How it was parsed (regex/fuzzy/llm)
}

// Time constants for natural language
const (
	DefaultMorningHour   = 7  // "pagi" default
	DefaultAfternoonHour = 15 // "sore" default
	DefaultEveningHour   = 19 // "malam" default
	DefaultNightHour     = 21 // "malem" default
	PreNotifyMinutes     = 5  // Notification before main time
)

// NewSmartTimeParser creates a new smart time parser
func NewSmartTimeParser(geminiClient *GeminiClient) *SmartTimeParser {
	loc, _ := time.LoadLocation("Asia/Jakarta") // WIB timezone
	return &SmartTimeParser{
		geminiClient: geminiClient,
		location:     loc,
	}
}

// ParseSmartTime is the main entry point for time parsing
// It tries multiple strategies: direct parsing → fuzzy matching → LLM fallback
func (stp *SmartTimeParser) ParseSmartTime(input string) (*TimeResult, error) {
	input = strings.TrimSpace(input)
	log.Printf("🕐 [SmartTimeParser] Input: %s", input)

	// Strategy 1: Direct regex-based parsing (fastest)
	result, err := stp.parseWithRegex(input)
	if err == nil {
		log.Printf("✅ [SmartTimeParser] Parsed with REGEX: %s", result.TargetTime.Format("2006-01-02 15:04"))
		return result, nil
	}

	// Strategy 2: Fuzzy matching for typos (medium speed)
	result, err = stp.parseWithFuzzyMatching(input)
	if err == nil {
		log.Printf("✅ [SmartTimeParser] Parsed with FUZZY MATCH: %s", result.TargetTime.Format("2006-01-02 15:04"))
		return result, nil
	}

	// Strategy 3: LLM fallback (slower but most flexible)
	if stp.geminiClient != nil {
		result, err = stp.parseWithLLM(input)
		if err == nil {
			log.Printf("✅ [SmartTimeParser] Parsed with LLM: %s", result.TargetTime.Format("2006-01-02 15:04"))
			return result, nil
		}
	}

	return nil, fmt.Errorf("tidak dapat memahami format waktu: %s", input)
}

// parseWithRegex handles common time patterns with regex
func (stp *SmartTimeParser) parseWithRegex(input string) (*TimeResult, error) {
	input = strings.ToLower(input)
	now := time.Now().In(stp.location)

	// Remove common prefixes
	input = cleanInput(input)

	// Pattern 1: "X menit lagi" or "X jam lagi"
	// Example: "5 menit lagi", "2 jam lagi"
	if match := regexp.MustCompile(`(\d+)\s*(menit|mnit|mnt)\s*(lagi|lg)`).FindStringSubmatch(input); match != nil {
		minutes := parseInt(match[1])
		targetTime := now.Add(time.Duration(minutes) * time.Minute)
		message := extractMessage(input, match[0])
		log.Printf("🕐 [SmartTimeParser] Detected: %d minutes from now", minutes)
		return stp.buildResult(targetTime, message, "regex:relative_minutes"), nil
	}

	if match := regexp.MustCompile(`(\d+)\s*(jam|jm)\s*(lagi|lg)`).FindStringSubmatch(input); match != nil {
		hours := parseInt(match[1])
		targetTime := now.Add(time.Duration(hours) * time.Hour)
		message := extractMessage(input, match[0])
		log.Printf("🕐 [SmartTimeParser] Detected: %d hours from now", hours)
		return stp.buildResult(targetTime, message, "regex:relative_hours"), nil
	}

	// Pattern 2: "besok jam XX:XX" or "besok XX.XX"
	// Example: "besok jam 15:30", "besok 14.00"
	if strings.Contains(input, "besok") || strings.Contains(input, "besk") || strings.Contains(input, "bsk") {
		hour, minute, hasTime := extractTimeFromText(input)
		tomorrow := now.AddDate(0, 0, 1)

		var targetTime time.Time
		if hasTime {
			targetTime = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), hour, minute, 0, 0, stp.location)
		} else {
			// No specific time mentioned, check for pagi/sore/malam
			hour = extractDayPeriodHour(input)
			targetTime = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), hour, 0, 0, 0, stp.location)
		}

		message := extractMessage(input, "besok")
		log.Printf("🕐 [SmartTimeParser] Detected: tomorrow at %02d:%02d", targetTime.Hour(), targetTime.Minute())
		return stp.buildResultWithMidnight(targetTime, message, "regex:tomorrow"), nil
	}

	// Pattern 3: "lusa jam XX:XX"
	// Example: "lusa jam 9 pagi"
	if strings.Contains(input, "lusa") || strings.Contains(input, "lusaa") {
		hour, minute, hasTime := extractTimeFromText(input)
		dayAfterTomorrow := now.AddDate(0, 0, 2)

		var targetTime time.Time
		if hasTime {
			targetTime = time.Date(dayAfterTomorrow.Year(), dayAfterTomorrow.Month(), dayAfterTomorrow.Day(), hour, minute, 0, 0, stp.location)
		} else {
			hour = extractDayPeriodHour(input)
			targetTime = time.Date(dayAfterTomorrow.Year(), dayAfterTomorrow.Month(), dayAfterTomorrow.Day(), hour, 0, 0, 0, stp.location)
		}

		message := extractMessage(input, "lusa")
		log.Printf("🕐 [SmartTimeParser] Detected: day after tomorrow at %02d:%02d", targetTime.Hour(), targetTime.Minute())
		return stp.buildResultWithMidnight(targetTime, message, "regex:lusa"), nil
	}

	// Pattern 4: "hari ini jam XX:XX" or just "jam XX:XX"
	// Example: "hari ini jam 20:30", "jam 14.00"
	hour, minute, hasTime := extractTimeFromText(input)
	if hasTime {
		targetTime := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, stp.location)

		// If time has passed today, assume tomorrow
		if targetTime.Before(now) {
			targetTime = targetTime.AddDate(0, 0, 1)
			log.Printf("🕐 [SmartTimeParser] Time passed today, moving to tomorrow")
		}

		message := extractMessage(input, "")
		log.Printf("🕐 [SmartTimeParser] Detected: today/tomorrow at %02d:%02d", targetTime.Hour(), targetTime.Minute())

		// Add midnight notification if moved to tomorrow
		if targetTime.Day() > now.Day() {
			return stp.buildResultWithMidnight(targetTime, message, "regex:today_moved"), nil
		}
		return stp.buildResult(targetTime, message, "regex:today"), nil
	}

	// Pattern 5: Just "pagi", "sore", "malam" without specific time
	// Example: "besok pagi", "hari ini malam"
	if dayPeriodHour := extractDayPeriodHour(input); dayPeriodHour > 0 {
		targetTime := time.Date(now.Year(), now.Month(), now.Day(), dayPeriodHour, 0, 0, 0, stp.location)

		if strings.Contains(input, "besok") {
			targetTime = targetTime.AddDate(0, 0, 1)
			message := extractMessage(input, "besok")
			log.Printf("🕐 [SmartTimeParser] Detected: tomorrow %s", getDayPeriodName(dayPeriodHour))
			return stp.buildResultWithMidnight(targetTime, message, "regex:tomorrow_period"), nil
		}

		if targetTime.Before(now) {
			targetTime = targetTime.AddDate(0, 0, 1)
		}

		message := extractMessage(input, "")
		log.Printf("🕐 [SmartTimeParser] Detected: %s", getDayPeriodName(dayPeriodHour))
		return stp.buildResult(targetTime, message, "regex:day_period"), nil
	}

	return nil, fmt.Errorf("regex parsing failed")
}

// parseWithFuzzyMatching handles typos using Levenshtein-like distance
func (stp *SmartTimeParser) parseWithFuzzyMatching(input string) (*TimeResult, error) {
	input = strings.ToLower(input)
	log.Printf("🔍 [SmartTimeParser] Trying fuzzy matching...")

	// Common typos dictionary
	typoMap := map[string]string{
		"besk":    "besok",
		"bsk":     "besok",
		"besook":  "besok",
		"har ini": "hari ini",
		"hri ini": "hari ini",
		"ingetin": "ingatkan",
		"ingatn":  "ingatkan",
		"ingtin":  "ingatkan",
		"jm":      "jam",
		"mnit":    "menit",
		"mnt":     "menit",
		"sre":     "sore",
		"mlm":     "malam",
		"mlam":    "malam",
		"pg":      "pagi",
		"lusaa":   "lusa",
	}

	// Replace typos
	corrected := input
	for typo, correct := range typoMap {
		if fuzzyMatch(input, typo) {
			corrected = strings.ReplaceAll(corrected, typo, correct)
			log.Printf("🔧 [SmartTimeParser] Fixed typo: '%s' → '%s'", typo, correct)
		}
	}

	// If correction made, retry with regex
	if corrected != input {
		return stp.parseWithRegex(corrected)
	}

	return nil, fmt.Errorf("fuzzy matching failed")
}

// parseWithLLM uses Gemini AI to understand complex/ambiguous time expressions
func (stp *SmartTimeParser) parseWithLLM(input string) (*TimeResult, error) {
	log.Printf("🤖 [SmartTimeParser] Falling back to LLM...")

	now := time.Now().In(stp.location)
	prompt := fmt.Sprintf(`Kamu adalah time parser. Extract informasi waktu dari input user berikut.

Current time: %s (WIB)

User input: "%s"

Tugas:
1. Tentukan tanggal dan jam yang dimaksud user
2. Extract pesan/reminder dari input (buang kata-kata waktu)

Response format (JSON):
{
  "date": "YYYY-MM-DD",
  "time": "HH:MM",
  "message": "extracted message",
  "is_tomorrow": true/false
}

Contoh:
Input: "ingatkan besok jam 3 sore meeting penting"
Output: {"date": "2025-10-13", "time": "15:00", "message": "meeting penting", "is_tomorrow": true}

Input: "5 menit lagi minum obat"
Output: {"date": "2025-10-12", "time": "20:30", "message": "minum obat", "is_tomorrow": false}

Sekarang parse input di atas. Hanya return JSON, tidak ada teks lain.`,
		now.Format("2006-01-02 15:04 Monday"), input)

	response, err := stp.geminiClient.CallGemini(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM parsing failed: %w", err)
	}

	// Parse LLM JSON response
	result, err := parseLLMResponse(response, now, stp.location)
	if err != nil {
		log.Printf("❌ [SmartTimeParser] Failed to parse LLM response: %v", err)
		return nil, err
	}

	return result, nil
}

// buildResult creates TimeResult with pre-notification
func (stp *SmartTimeParser) buildResult(targetTime time.Time, message string, method string) *TimeResult {
	preNotifyTime := targetTime.Add(-PreNotifyMinutes * time.Minute)

	if message == "" {
		message = "Reminder"
	}

	return &TimeResult{
		TargetTime:    targetTime,
		Message:       message,
		PreNotifyTime: preNotifyTime,
		ParseMethod:   method,
	}
}

// buildResultWithMidnight creates TimeResult with midnight notification
func (stp *SmartTimeParser) buildResultWithMidnight(targetTime time.Time, message string, method string) *TimeResult {
	result := stp.buildResult(targetTime, message, method)

	// Add midnight notification (00:00 on the day of reminder)
	midnightTime := time.Date(
		targetTime.Year(),
		targetTime.Month(),
		targetTime.Day(),
		0, 0, 0, 0,
		stp.location,
	)
	result.MidnightNotifyTime = &midnightTime

	return result
}

// Helper functions

// cleanInput removes common prefixes and normalizes input
func cleanInput(input string) string {
	input = strings.ToLower(input)
	prefixes := []string{"ingatkan", "ingetin", "ingatn", "reminder", "saya", "aku", "untuk", "ttg", "tentang"}

	for _, prefix := range prefixes {
		input = strings.ReplaceAll(input, prefix, "")
	}

	return strings.TrimSpace(input)
}

// extractTimeFromText extracts hour and minute from text
// Returns: hour, minute, found
func extractTimeFromText(text string) (int, int, bool) {
	text = strings.ToLower(text)

	// Special case: "jam X malam/malem/mlm" → convert to 24h format (PM)
	// Example: "jam 10 malam" → 22:00 (10 PM)
	if match := regexp.MustCompile(`jam\s+(\d{1,2})\s*(malam|malem|mlm)`).FindStringSubmatch(text); match != nil {
		hour := parseInt(match[1])
		if hour >= 1 && hour <= 12 {
			// Convert to PM (add 12 hours)
			hour = hour + 12
			if hour == 24 {
				hour = 0 // midnight
			}
			log.Printf("🕐 [SmartTimeParser] Converted to PM: %02d:00", hour)
			return hour, 0, true
		}
	}

	// Special case: "jam X sore" → use afternoon time (12-18)
	if match := regexp.MustCompile(`jam\s+(\d{1,2})\s*(sore|sre)`).FindStringSubmatch(text); match != nil {
		hour := parseInt(match[1])
		if hour >= 1 && hour <= 6 {
			// Afternoon: 1 sore = 13:00, 6 sore = 18:00
			hour = hour + 12
			log.Printf("🕐 [SmartTimeParser] Converted to afternoon: %02d:00", hour)
			return hour, 0, true
		}
	}

	// Pattern: "jam 20:30", "20:30", "20.30", "jam 20"
	patterns := []string{
		`jam\s+(\d{1,2})[:\.](\d{2})`, // jam 20:30 or jam 20.30
		`(\d{1,2})[:\.](\d{2})`,       // 20:30 or 20.30
		`jam\s+(\d{1,2})`,             // jam 20
	}

	for i, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(text); match != nil {
			hour := parseInt(match[1])
			var minute int
			if len(match) > 2 && match[2] != "" {
				minute = parseInt(match[2])
			}

			// Validate
			if hour >= 0 && hour < 24 && minute >= 0 && minute < 60 {
				log.Printf("🕐 [SmartTimeParser] Extracted time: %02d:%02d (pattern %d)", hour, minute, i+1)
				return hour, minute, true
			}
		}
	}

	return 0, 0, false
}

// extractDayPeriodHour extracts hour based on day period (pagi/sore/malam)
func extractDayPeriodHour(text string) int {
	text = strings.ToLower(text)

	if strings.Contains(text, "pagi") || strings.Contains(text, "pg") {
		return DefaultMorningHour
	}
	if strings.Contains(text, "sore") || strings.Contains(text, "sre") {
		return DefaultAfternoonHour
	}
	if strings.Contains(text, "malam") || strings.Contains(text, "mlm") || strings.Contains(text, "malem") {
		return DefaultEveningHour
	}

	return 0
}

// getDayPeriodName returns name of day period
func getDayPeriodName(hour int) string {
	switch hour {
	case DefaultMorningHour:
		return "pagi"
	case DefaultAfternoonHour:
		return "sore"
	case DefaultEveningHour, DefaultNightHour:
		return "malam"
	default:
		return fmt.Sprintf("jam %d", hour)
	}
}

// extractMessage removes time-related words and extracts the reminder message
func extractMessage(text string, timeIndicator string) string {
	text = strings.ToLower(text)

	// Remove time-related keywords with word boundaries
	// Use regex to match whole words only
	removePatterns := []string{
		`\bingatkan\b`, `\bingetin\b`, `\breminder\b`,
		`\bsaya\b`, `\baku\b`, `\buntuk\b`, `\bbuat\b`,
		`\bbesok\b`, `\blusa\b`, `\bhari\s+ini\b`,
		`\bjam\b`, `\bmenit\b`, `\blagi\b`,
		`\bpagi\b`, `\bsore\b`, `\bmalam\b`, `\bmalem\b`, `\bmlm\b`,
		`\bini\b`, `\bnanti\b`,
	}

	for _, pattern := range removePatterns {
		re := regexp.MustCompile(pattern)
		text = re.ReplaceAllString(text, " ")
	}

	// Remove specific time indicator if provided
	if timeIndicator != "" {
		text = strings.ReplaceAll(text, timeIndicator, " ")
	}

	// Remove time patterns (20:30, 14.00, 10 malam, etc)
	text = regexp.MustCompile(`\d{1,2}[:\.]?\d{0,2}\s*(malam|malem|sore|pagi)?`).ReplaceAllString(text, "")

	// Clean up multiple spaces and punctuation
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// If nothing left, return default
	if text == "" {
		return "Reminder"
	}

	return text
}

// parseInt safely parses string to int
func parseInt(s string) int {
	var num int
	fmt.Sscanf(s, "%d", &num)
	return num
}

// fuzzyMatch checks if two strings are similar (simple implementation)
func fuzzyMatch(text, pattern string) bool {
	return strings.Contains(text, pattern) || levenshteinDistance(text, pattern) <= 2
}

// levenshteinDistance calculates edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// parseLLMResponse parses JSON response from LLM
func parseLLMResponse(jsonStr string, now time.Time, location *time.Location) (*TimeResult, error) {
	// Extract JSON from response (may contain markdown code blocks)
	jsonStr = strings.TrimSpace(jsonStr)
	jsonStr = strings.Trim(jsonStr, "`")
	jsonStr = strings.TrimPrefix(jsonStr, "json")
	jsonStr = strings.TrimSpace(jsonStr)

	// Simple JSON parsing (for production, use encoding/json)
	datePattern := regexp.MustCompile(`"date":\s*"([^"]+)"`)
	timePattern := regexp.MustCompile(`"time":\s*"([^"]+)"`)
	messagePattern := regexp.MustCompile(`"message":\s*"([^"]+)"`)

	dateMatch := datePattern.FindStringSubmatch(jsonStr)
	timeMatch := timePattern.FindStringSubmatch(jsonStr)
	messageMatch := messagePattern.FindStringSubmatch(jsonStr)

	if dateMatch == nil || timeMatch == nil {
		return nil, fmt.Errorf("invalid LLM response format")
	}

	// Parse date and time
	targetTimeStr := fmt.Sprintf("%s %s", dateMatch[1], timeMatch[1])
	targetTime, err := time.ParseInLocation("2006-01-02 15:04", targetTimeStr, location)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM time: %w", err)
	}

	message := "Reminder"
	if messageMatch != nil && messageMatch[1] != "" {
		message = messageMatch[1]
	}

	// Check if it's tomorrow or later
	parser := &SmartTimeParser{location: location}
	if targetTime.Day() > now.Day() || targetTime.Month() > now.Month() {
		return parser.buildResultWithMidnight(targetTime, message, "llm"), nil
	}

	return parser.buildResult(targetTime, message, "llm"), nil
}
