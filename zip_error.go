package main

import "strings"

// 7-Zip uses exit code 2 for both password failures and other fatal errors.
// Match its messages separately from paths, which can contain the same words.
func isPasswordDiagnostic(detail string) bool {
	for _, line := range strings.Split(detail, "\n") {
		message, _, _ := strings.Cut(strings.TrimSpace(line), " : ")
		switch message {
		case "Cannot open encrypted archive. Wrong password?",
			"Headers Error in encrypted archive. Wrong password?",
			"ERROR: Wrong password",
			"ERROR: Data Error in encrypted file. Wrong password?",
			"ERROR: CRC Failed in encrypted file. Wrong password?":
			return true
		}
	}
	return false
}
