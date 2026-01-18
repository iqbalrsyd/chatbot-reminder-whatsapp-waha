package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// WAHAClient handles communication with WAHA API
type WAHAClient struct {
	BaseURL string
	Session string
	APIKey  string // API Key for authentication
}

// SendTextRequest represents the request body for sending text message
type SendTextRequest struct {
	ChatID  string `json:"chatId"`
	Text    string `json:"text"`
	Session string `json:"session"`
}

// NewWAHAClient creates a new WAHA client
func NewWAHAClient(baseURL, session, apiKey string) *WAHAClient {
	return &WAHAClient{
		BaseURL: baseURL,
		Session: session,
		APIKey:  apiKey,
	}
}

// SendText sends a text message to a WhatsApp number via WAHA API
func (w *WAHAClient) SendText(chatID, text string) error {
	url := fmt.Sprintf("%s/api/sendText", w.BaseURL)

	payload := SendTextRequest{
		ChatID:  chatID,
		Text:    text,
		Session: w.Session,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	log.Printf("📤 Sending message to %s via WAHA: %s", chatID, text)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add API key if available
	if w.APIKey != "" {
		req.Header.Set("X-Api-Key", w.APIKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("WAHA API returned status: %d", resp.StatusCode)
	}

	log.Printf("✅ Message sent successfully to %s", chatID)
	return nil
}
