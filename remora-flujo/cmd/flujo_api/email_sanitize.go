package main

import (
	"regexp"
	"strings"
)

// dedupeSubjectPrefix elimina prefijos namespace repetidos del subject.
// Ej: "Cobranza: Cobranza: Foo" → "Cobranza: Foo".
//     "Cobranza: Cobranza Empresa S.A." → "Cobranza: Empresa S.A."  (no, ese
//     caso no se toca: solo aplica si la palabra reaparece justo después
//     del primer ":").
//
// La regla es conservadora: identifica el primer prefijo "WORD:" y, si el
// resto empieza con "WORD" (mismo token, posiblemente seguido de ":"), lo
// remueve. Sirve sin importar el namespace concreto (no hardcodea
// "Cobranza:"; funciona también para "Re:", "FW:", etc).
func dedupeSubjectPrefix(subject string) string {
	s := strings.TrimSpace(subject)
	if s == "" {
		return s
	}
	colon := strings.IndexByte(s, ':')
	if colon <= 0 || colon > 32 {
		return s
	}
	prefix := strings.TrimSpace(s[:colon])
	rest := strings.TrimSpace(s[colon+1:])
	if prefix == "" || rest == "" {
		return s
	}
	lowerRest := strings.ToLower(rest)
	lowerPrefix := strings.ToLower(prefix)
	// "Cobranza: Cobranza: Foo" → quitar el segundo "Cobranza:".
	if strings.HasPrefix(lowerRest, lowerPrefix+":") {
		rest = strings.TrimSpace(rest[len(prefix)+1:])
	} else if strings.HasPrefix(lowerRest, lowerPrefix+" ") {
		// "Cobranza: Cobranza Empresa" → quitar la palabra duplicada.
		// Solo si después hay otra palabra; no tocar "Cobranza Empresa S.A."
		// si solo hay un prefijo.
		// Conservador: lo dejamos como está para evitar romper subjects válidos.
		_ = rest
	}
	return prefix + ": " + rest
}

// placeholderPattern matchea `[ALGO]` (corchetes simples con texto), que es
// el formato típico que dejan los LLMs cuando no completaron un campo.
// Ignora `[ ]` vacíos, URLs entre corchetes (http), y referencias de
// markdown tipo `[texto](url)`.
var placeholderPattern = regexp.MustCompile(`\[([^\]\n]{1,80})\]`)

// markdownLinkPattern matchea `[texto](url)` para excluirlos del check.
var markdownLinkPattern = regexp.MustCompile(`\[([^\]\n]{1,80})\]\([^)]+\)`)

// unresolvedPlaceholders devuelve los tokens dentro de `[...]` del body que
// parecen ser placeholders sin resolver. Excluye links markdown y URLs.
func unresolvedPlaceholders(body string) []string {
	if body == "" {
		return nil
	}
	// Remover links markdown antes de buscar placeholders sueltos.
	cleaned := markdownLinkPattern.ReplaceAllString(body, "")

	matches := placeholderPattern.FindAllStringSubmatch(cleaned, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, m := range matches {
		token := strings.TrimSpace(m[1])
		if token == "" {
			continue
		}
		// Heurística: si el contenido empieza con http/www es probablemente
		// una URL escapada, no un placeholder.
		lower := strings.ToLower(token)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "www.") {
			continue
		}
		if seen[token] {
			continue
		}
		seen[token] = true
		out = append(out, "["+token+"]")
	}
	return out
}
