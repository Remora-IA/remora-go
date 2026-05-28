// contactos.go: cliente liviano para invocar Sabio como proveedor canónico de contact.lookup.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type contactsLookupResult struct {
	Found             bool   `json:"found"`
	Value             string `json:"value"`
	Source            string `json:"source"`
	VerifiedAt        string `json:"verified_at"`
	MissingCapability string `json:"missing_capability"`
	ProviderHint      string `json:"provider_hint"`
	Error             string `json:"error"`
}

type contactsStoreResult struct {
	Success    bool   `json:"success"`
	Error      string `json:"error"`
	EntityType string `json:"entity_type"`
	EntityRef  string `json:"entity_ref"`
	Channel    string `json:"channel"`
	Value      string `json:"value"`
	Source     string `json:"source"`
}

func resolveSabioContacts() (string, string, error) {
	root := resolveRemoraRoot()
	cwd := filepath.Join(root, "framework-sabio")
	bin := envOr("REMORA_SABIO_BIN", filepath.Join(cwd, "frameworksabio"))
	return bin, cwd, nil
}

func contactosLookup(entityType, entityRef, channel string) (*contactsLookupResult, error) {
	return contactosLookupProfile(envOr("REMORA_PROFILE", "default"), entityType, entityRef, channel)
}

func contactosLookupProfile(profile, entityType, entityRef, channel string) (*contactsLookupResult, error) {
	if channel == "" {
		channel = "email"
	}
	bin, cwd, err := resolveSabioContacts()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin,
		"contact-lookup",
		"--profile", profile,
		"--entity-type", entityType,
		"--entity-ref", entityRef,
		"--channel", channel,
	)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	out, runErr := cmd.Output()
	var res contactsLookupResult
	if jerr := json.Unmarshal(out, &res); jerr != nil {
		stderr := ""
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return nil, fmt.Errorf("sabio contact-lookup respuesta inválida: %v stderr=%s out=%s", jerr, stderr, string(out))
	}
	return &res, nil
}

func contactosStoreProfile(profile, entityType, entityRef, channel, value, source string) (*contactsStoreResult, error) {
	if channel == "" {
		channel = "email"
	}
	if source == "" {
		source = "flow_user_input"
	}
	bin, cwd, err := resolveSabioContacts()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin,
		"contact-store",
		"--profile", profile,
		"--entity-type", entityType,
		"--entity-ref", entityRef,
		"--channel", channel,
		"--value", value,
		"--source", source,
	)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	out, runErr := cmd.Output()
	var res contactsStoreResult
	if jerr := json.Unmarshal(out, &res); jerr != nil {
		stderr := ""
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return nil, fmt.Errorf("sabio contact-store respuesta inválida: %v stderr=%s out=%s", jerr, stderr, string(out))
	}
	if !res.Success {
		if res.Error == "" {
			res.Error = "sabio contact-store falló"
		}
		return &res, fmt.Errorf("%s", res.Error)
	}
	return &res, nil
}
