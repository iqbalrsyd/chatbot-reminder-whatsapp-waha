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
}

// SendTextRequest represents the request body for sending text message
type SendTextRequest struct {
	ChatID  string `json:"chatId"`
	Text    string `json:"text"`
	Session string `json:"session"`
}

// NewWAHAClient creates a new WAHA client
func NewWAHAClient(baseURL, session string) *WAHAClient {
	return &WAHAClient{
		BaseURL: baseURL,
		Session: session,
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
