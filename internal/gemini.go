package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// GeminiClient handles communication with Google Gemini API
type GeminiClient struct {
	APIKey string
	Model  string
}

// GeminiRequest represents the request body for Gemini API
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiResponse represents the response from Gemini API
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient(apiKey string) *GeminiClient {
	return &GeminiClient{
		APIKey: apiKey,
		Model:  "gemini-2.5-flash", // Latest free model
	}
}

// CallGemini sends a prompt to Gemini API and returns the response
func (g *GeminiClient) CallGemini(prompt string) (string, error) {
	// Use v1 API instead of v1beta
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models/%s:generateContent?key=%s",
		g.Model, g.APIKey)

	// Add WhatsApp formatting instructions as system context
	enhancedPrompt := formatPromptForWhatsApp(prompt)

	payload := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: enhancedPrompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshaling JSON: %w", err)
	}

	log.Printf("🤖 Calling Gemini API with model: %s", g.Model)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	// Handle errors
	if resp.StatusCode == 400 {
		log.Printf("⚠️  Gemini API key not valid or not provided")
		return "Maaf, Gemini API key belum dikonfigurasi. 😅\n\n💡 Admin: Dapatkan API key gratis di https://aistudio.google.com/app/apikey\n\nSementara saya bisa bantu dengan:\n✅ Reminder - ketik: 'ingatkan saya [sesuatu]'\n✅ Info waktu - ketik: 'jam berapa'", nil
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Gemini API error: %s", string(body))
		return "", fmt.Errorf("Gemini API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini API")
	}

	result := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
	log.Printf("✅ Gemini Response received: %s", truncateStr(result, 100))

	return result, nil
}

// truncateStr truncates a string to specified length with ellipsis
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// formatPromptForWhatsApp wraps user prompt with WhatsApp formatting instructions
func formatPromptForWhatsApp(userPrompt string) string {
	systemPrompt := `You are a helpful AI assistant responding via WhatsApp. Format your responses using WhatsApp markdown:

FORMATTING RULES:
- Use *bold* for important words, titles, or emphasis (e.g., *Important:*, *Note:*)
- Use _italic_ for subtle emphasis or quotes
- Use ~strikethrough~ for corrections or deprecated info
- Use ` + "`code`" + ` for technical terms, commands, or short code
- Use numbered lists (1., 2., 3.) or bullet points (•) for clarity
- Keep paragraphs short (2-3 lines max) for readability
- Use emojis sparingly for context (✅, ❌, 💡, ⚠️, 📝, etc.)
- Avoid long walls of text - break into digestible chunks

STYLE GUIDELINES:
- Be conversational but informative
- Use formatting to highlight key information
- Make responses scannable and easy to read on mobile
- Use line breaks between sections

Now respond to this user message:

`
	return systemPrompt + userPrompt
}
