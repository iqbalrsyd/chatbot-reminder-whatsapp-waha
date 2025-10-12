package internal

import (
	"testing"
	"time"
)

func TestSmartTimeParser_ParseSmartTime(t *testing.T) {
	// Mock Gemini client (nil for now, will test without LLM first)
	parser := NewSmartTimeParser(nil)

	// Set reference time for testing
	referenceTime := time.Date(2025, 10, 12, 20, 0, 0, 0, parser.location)

	tests := []struct {
		name          string
		input         string
		expectHour    int
		expectMinute  int
		expectDayDiff int // 0=today, 1=tomorrow, 2=day after tomorrow
		expectMessage string
		shouldError   bool
	}{
		// Relative time tests
		{
			name:          "5 menit lagi",
			input:         "ingatkan 5 menit lagi minum obat",
			expectHour:    20,
			expectMinute:  5,
			expectDayDiff: 0,
			expectMessage: "minum obat",
			shouldError:   false,
		},
		{
			name:          "2 jam lagi",
			input:         "ingatkan 2 jam lagi meeting",
			expectHour:    22,
			expectMinute:  0,
			expectDayDiff: 0,
			expectMessage: "meeting",
			shouldError:   false,
		},

		// Tomorrow tests
		{
			name:          "besok jam 15:30",
			input:         "ingatkan besok jam 15:30 konsultasi",
			expectHour:    15,
			expectMinute:  30,
			expectDayDiff: 1,
			expectMessage: "konsultasi",
			shouldError:   false,
		},
		{
			name:          "besok pagi",
			input:         "ingatkan besok pagi ke kampus",
			expectHour:    7,
			expectMinute:  0,
			expectDayDiff: 1,
			expectMessage: "ke kampus",
			shouldError:   false,
		},
		{
			name:          "besok sore",
			input:         "besok sore meeting",
			expectHour:    15,
			expectMinute:  0,
			expectDayDiff: 1,
			expectMessage: "meeting",
			shouldError:   false,
		},

		// Day after tomorrow tests
		{
			name:          "lusa jam 9 pagi",
			input:         "lusa jam 9 pagi bangun",
			expectHour:    9,
			expectMinute:  0,
			expectDayDiff: 2,
			expectMessage: "bangun",
			shouldError:   false,
		},

		// Today tests
		{
			name:          "hari ini jam 21:00",
			input:         "hari ini jam 21:00 solat",
			expectHour:    21,
			expectMinute:  0,
			expectDayDiff: 0,
			expectMessage: "solat",
			shouldError:   false,
		},
		{
			name:          "jam 20.30",
			input:         "jam 20.30 makan malam",
			expectHour:    20,
			expectMinute:  30,
			expectDayDiff: 0,
			expectMessage: "makan malam",
			shouldError:   false,
		},

		// Time format variations
		{
			name:          "14:00 format",
			input:         "14:00 minum obat",
			expectHour:    14,
			expectMinute:  0,
			expectDayDiff: 1, // Past time, should move to tomorrow
			expectMessage: "minum obat",
			shouldError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseSmartTime(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check hour and minute
			if result.TargetTime.Hour() != tt.expectHour {
				t.Errorf("Expected hour %d, got %d", tt.expectHour, result.TargetTime.Hour())
			}

			if result.TargetTime.Minute() != tt.expectMinute {
				t.Errorf("Expected minute %d, got %d", tt.expectMinute, result.TargetTime.Minute())
			}

			// Check day difference
			expectedDay := referenceTime.AddDate(0, 0, tt.expectDayDiff).Day()
			if result.TargetTime.Day() != expectedDay {
				t.Errorf("Expected day %d, got %d", expectedDay, result.TargetTime.Day())
			}

			// Check pre-notification (should be 5 minutes before)
			expectedPreNotify := result.TargetTime.Add(-5 * time.Minute)
			if !result.PreNotifyTime.Equal(expectedPreNotify) {
				t.Errorf("Pre-notification time incorrect: expected %v, got %v",
					expectedPreNotify.Format("15:04"), result.PreNotifyTime.Format("15:04"))
			}

			// Check midnight notification for tomorrow/lusa
			if tt.expectDayDiff > 0 {
				if result.MidnightNotifyTime == nil {
					t.Error("Expected midnight notification but got nil")
				} else {
					if result.MidnightNotifyTime.Hour() != 0 || result.MidnightNotifyTime.Minute() != 0 {
						t.Errorf("Midnight notification should be at 00:00, got %02d:%02d",
							result.MidnightNotifyTime.Hour(), result.MidnightNotifyTime.Minute())
					}
				}
			}

			t.Logf("✅ Parsed '%s' → %s (method: %s)",
				tt.input,
				result.TargetTime.Format("2006-01-02 15:04"),
				result.ParseMethod)
		})
	}
}

func TestSmartTimeParser_FuzzyMatching(t *testing.T) {
	parser := NewSmartTimeParser(nil)

	tests := []struct {
		name        string
		input       string
		expectHour  int
		shouldParse bool
	}{
		{
			name:        "typo: besk (besok)",
			input:       "besk jam 10",
			expectHour:  10,
			shouldParse: true,
		},
		{
			name:        "typo: har ini (hari ini)",
			input:       "har ini jam 21",
			expectHour:  21,
			shouldParse: true,
		},
		{
			name:        "typo: ingetin (ingatkan)",
			input:       "ingetin jam 8 pagi",
			expectHour:  8,
			shouldParse: true,
		},
		{
			name:        "typo: mnit (menit)",
			input:       "10 mnit lagi",
			expectHour:  -1, // relative time
			shouldParse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseSmartTime(tt.input)

			if tt.shouldParse {
				if err != nil {
					t.Errorf("Expected to parse typo but got error: %v", err)
					return
				}

				if tt.expectHour >= 0 && result.TargetTime.Hour() != tt.expectHour {
					t.Errorf("Expected hour %d, got %d", tt.expectHour, result.TargetTime.Hour())
				}

				t.Logf("✅ Parsed typo '%s' successfully (method: %s)", tt.input, result.ParseMethod)
			}
		})
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"besok", "besk", 1},
		{"besok", "bsk", 2},
		{"hari", "har", 1},
		{"menit", "mnit", 1},
		{"ingatkan", "ingetin", 3},
	}

	for _, tt := range tests {
		t.Run(tt.s1+"_"+tt.s2, func(t *testing.T) {
			distance := levenshteinDistance(tt.s1, tt.s2)
			if distance != tt.expected {
				t.Errorf("Expected distance %d, got %d for '%s' and '%s'",
					tt.expected, distance, tt.s1, tt.s2)
			}
		})
	}
}

func TestExtractTimeFromText(t *testing.T) {
	tests := []struct {
		input        string
		expectHour   int
		expectMinute int
		expectFound  bool
	}{
		{"jam 20:30", 20, 30, true},
		{"20:30", 20, 30, true},
		{"20.30", 20, 30, true},
		{"jam 14", 14, 0, true},
		{"besok jam 15:45", 15, 45, true},
		{"invalid time", 0, 0, false},
		{"jam 25:00", 0, 0, false}, // invalid hour
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			hour, minute, found := extractTimeFromText(tt.input)

			if found != tt.expectFound {
				t.Errorf("Expected found=%v, got found=%v", tt.expectFound, found)
			}

			if found {
				if hour != tt.expectHour || minute != tt.expectMinute {
					t.Errorf("Expected %02d:%02d, got %02d:%02d",
						tt.expectHour, tt.expectMinute, hour, minute)
				}
			}
		})
	}
}

func TestExtractDayPeriodHour(t *testing.T) {
	tests := []struct {
		input      string
		expectHour int
	}{
		{"besok pagi", DefaultMorningHour},
		{"hari ini sore", DefaultAfternoonHour},
		{"malam ini", DefaultEveningHour},
		{"besok malem", DefaultEveningHour},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			hour := extractDayPeriodHour(tt.input)
			if hour != tt.expectHour {
				t.Errorf("Expected hour %d, got %d", tt.expectHour, hour)
			}
		})
	}
}

func TestExtractMessage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ingatkan besok jam 15:00 meeting penting", "meeting penting"},
		{"jam 20:30 solat isya", "solat isya"},
		{"5 menit lagi minum obat", "minum obat"},
		{"besok pagi ke kampus", "ke kampus"},
		{"ingatkan saya untuk", "Reminder"}, // no message
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractMessage(tt.input, "")
			if result != tt.expected {
				t.Errorf("Expected message '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
