// contactos.go: cliente liviano para invocar el binario `frameworkcontactos`
// desde el orquestador. Mantiene el contrato canónico contact.lookup sin
// acoplar el backend a un schema específico.
package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
)

type contactsLookupResult struct {
	Found              bool   `json:"found"`
	Value              string `json:"value"`
	Source             string `json:"source"`
	VerifiedAt         string `json:"verified_at"`
	MissingCapability  string `json:"missing_capability"`
	ProviderHint       string `json:"provider_hint"`
	Error              string `json:"error"`
}

// resolveContactos localiza el binario frameworkcontactos y su cwd.
// Resolución: env REMORA_CONTACTOS_BIN, o ../framework-contactos/frameworkcontactos
// relativo a REMORA_ROOT.
func resolveContactos() (string, string, error) {
	root := envOr("REMORA_ROOT", envOr("CHANNEL_BASE_DIR", "/workspace"))
	cwd := filepath.Join(root, "framework-contactos")
	bin := envOr("REMORA_CONTACTOS_BIN", filepath.Join(cwd, "frameworkcontactos"))
	return bin, cwd, nil
}

// contactosLookup invoca `frameworkcontactos lookup` y parsea la respuesta.
// El profile se toma de REMORA_PROFILE.
func contactosLookup(entityType, entityRef, channel string) (*contactsLookupResult, error) {
	if channel == "" {
		channel = "email"
	}
	bin, cwd, err := resolveContactos()
	if err != nil {
		return nil, err
	}
	profile := envOr("REMORA_PROFILE", "default")
	cmd := exec.Command(bin,
		"lookup",
		"--profile", profile,
		"--entity-type", entityType,
		"--entity-ref", entityRef,
		"--channel", channel,
	)
	cmd.Dir = cwd
	out, runErr := cmd.Output()
	// El binario imprime JSON tanto en éxito como en "not found";
	// solo es error si stdout no es JSON parseable.
	var res contactsLookupResult
	if jerr := json.Unmarshal(out, &res); jerr != nil {
		stderr := ""
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return nil, fmt.Errorf("contactos respuesta inválida: %v stderr=%s out=%s", jerr, stderr, string(out))
	}
	return &res, nil
}
