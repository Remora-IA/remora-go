package credentials

import (
	"encoding/json"
	"strings"

	"channel/vault"
)

type SMTPBundle struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	User      string `json:"user"`
	Pass      string `json:"pass"`
	From      string `json:"from,omitempty"`
	DefaultTo string `json:"default_to,omitempty"`
}

type CapabilityScope struct {
	ConvID  string `json:"conv_id"`
	BaseDir string `json:"base_dir,omitempty"`
}

type SMTPStatus struct {
	Capability    string          `json:"capability"`
	Scope         CapabilityScope `json:"scope"`
	Present       bool            `json:"present"`
	Readable      bool            `json:"readable"`
	Complete      bool            `json:"complete"`
	Ready         bool            `json:"ready"`
	MissingFields []string        `json:"missing_fields,omitempty"`
	Error         string          `json:"error,omitempty"`
}

func LoadSMTP(baseDir, convID string) (SMTPBundle, SMTPStatus) {
	status := SMTPStatus{
		Capability: "credentials.smtp",
		Scope: CapabilityScope{
			ConvID: strings.TrimSpace(convID),
		},
	}
	if status.Scope.ConvID == "" {
		status.Scope.ConvID = "default"
	}
	if strings.TrimSpace(baseDir) == "" {
		baseDir = vault.DefaultBaseDir()
	}
	status.Scope.BaseDir = baseDir

	raw, err := vault.Get(baseDir, convID, status.Capability)
	if err != nil {
		if err == vault.ErrNotFound {
			status.Error = "No hay credenciales SMTP guardadas en el vault."
			return SMTPBundle{}, status
		}
		status.Error = err.Error()
		return SMTPBundle{}, status
	}
	status.Present = true

	var bundle SMTPBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		status.Error = "credentials.smtp inválidas: " + err.Error()
		return SMTPBundle{}, status
	}
	status.Readable = true
	bundle.ApplyDefaults()

	if strings.TrimSpace(bundle.Host) == "" {
		status.MissingFields = append(status.MissingFields, "host")
	}
	if strings.TrimSpace(bundle.User) == "" {
		status.MissingFields = append(status.MissingFields, "user")
	}
	if strings.TrimSpace(bundle.Pass) == "" {
		status.MissingFields = append(status.MissingFields, "pass")
	}
	status.Complete = len(status.MissingFields) == 0
	status.Ready = status.Complete
	if !status.Complete {
		status.Error = "Credenciales SMTP incompletas (falta " + strings.Join(status.MissingFields, ", ") + ")."
	}
	return bundle, status
}

func SaveSMTP(baseDir, convID string, bundle SMTPBundle) error {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = vault.DefaultBaseDir()
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		return err
	}
	return vault.Set(baseDir, convID, "credentials.smtp", data)
}

func DeleteSMTP(baseDir, convID string) error {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = vault.DefaultBaseDir()
	}
	return vault.Delete(baseDir, convID, "credentials.smtp")
}

func (b *SMTPBundle) ApplyDefaults() {
	if strings.TrimSpace(b.Port) == "" {
		b.Port = "587"
	}
	if strings.TrimSpace(b.From) == "" {
		b.From = b.User
	}
}
