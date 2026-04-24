package flowguard

import (
	"fmt"
	"strings"
)

const consolePreviewLimit = 220

func formatConsoleValue(value any) string {
	switch v := value.(type) {
	case string:
		return previewConsoleString(v)
	case []byte:
		return previewConsoleString(string(v))
	default:
		return previewConsoleString(fmt.Sprintf("%v", value))
	}
}

func previewConsoleString(value string) string {
	sanitized := strings.ReplaceAll(value, "\r", "\\r")
	sanitized = strings.ReplaceAll(sanitized, "\n", "\\n")

	if len(sanitized) <= consolePreviewLimit {
		return sanitized
	}

	return fmt.Sprintf("%s...(truncated, %d chars)", sanitized[:consolePreviewLimit], len(sanitized))
}
