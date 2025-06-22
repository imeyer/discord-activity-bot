package bot

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// Discord ID format: 17-19 digit number
	discordIDRegex = regexp.MustCompile(`^\d{17,19}$`)
	
	// Maximum lengths based on Discord's limits
	maxUserIDLength    = 19
	maxChannelIDLength = 19
	maxServerIDLength  = 19
	maxMessageIDLength = 19
)

// ValidateDiscordID validates that an ID matches Discord's snowflake format
func ValidateDiscordID(id string, idType string) bool {
	if id == "" {
		return false
	}
	
	// Check length
	if len(id) > maxUserIDLength {
		return false
	}
	
	// Check format (numeric only)
	if !discordIDRegex.MatchString(id) {
		return false
	}
	
	return true
}

// SanitizeUserInput removes potentially dangerous characters
func SanitizeUserInput(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")
	
	// Ensure valid UTF-8
	if !utf8.ValidString(input) {
		// Remove invalid UTF-8 sequences
		input = strings.ToValidUTF8(input, "")
	}
	
	// Trim excessive whitespace
	input = strings.TrimSpace(input)
	
	return input
}

// ValidateMessageParams validates message parameters before database insertion
func ValidateMessageParams(userID, channelID, serverID, messageID string) error {
	if !ValidateDiscordID(userID, "user") {
		return fmt.Errorf("invalid user ID: %s", userID)
	}
	
	if !ValidateDiscordID(channelID, "channel") {
		return fmt.Errorf("invalid channel ID: %s", channelID)
	}
	
	if !ValidateDiscordID(serverID, "server") {
		return fmt.Errorf("invalid server ID: %s", serverID)
	}
	
	if !ValidateDiscordID(messageID, "message") {
		return fmt.Errorf("invalid message ID: %s", messageID)
	}
	
	return nil
}

// ValidateCommandInput validates slash command inputs
func ValidateCommandInput(input string, maxLength int) (string, error) {
	// Sanitize first
	input = SanitizeUserInput(input)
	
	// Check length
	if len(input) > maxLength {
		return "", fmt.Errorf("input exceeds maximum length of %d", maxLength)
	}
	
	// Only check for actual SQL injection patterns, not keywords
	// Since we use parameterized queries, this is defense in depth
	dangerousPatterns := []string{
		"';--",
		"'; DROP",
		"' OR '1'='1",
		"' OR 1=1--",
		"'; DELETE",
		"'; EXEC",
		"' UNION SELECT",
	}
	
	lowerInput := strings.ToLower(input)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerInput, strings.ToLower(pattern)) {
			return "", fmt.Errorf("potentially dangerous input pattern detected")
		}
	}
	
	return input, nil
}