package charlie

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// AuditLogPath is where every --apply operation leaves a trail. We keep it
// under framework-charlie/temp/ (gitignored) so the history survives reflog
// corruption like the one seen in v0.1.6.
const AuditLogPath = "framework-charlie/temp/applied.jsonl"

// appendAudit records one line in the audit log. Best-effort; we never block
// an operation because audit failed, but we still surface the error path for
// tests. Fields are arbitrary strings for forward compatibility.
func appendAudit(op string, fields map[string]string) {
	entry := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"op":        op,
		"fields":    fields,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	full := filepath.Join(RepoRoot, AuditLogPath)
	_ = os.MkdirAll(filepath.Dir(full), 0o755)
	f, err := os.OpenFile(full, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}
