package main

import "strings"

func (s *server) providerNameForCapability(capability string) string {
	_, providerName, _ := s.findProviderForCapability(capability)
	if providerName != "" {
		return providerName
	}
	return defaultProviderNameForCapability(capability)
}

func defaultProviderNameForCapability(capability string) string {
	switch strings.TrimSpace(capability) {
	case "analysis.configure":
		return "radar"
	case "focus.complete_cycle", "focus.branch_test":
		return "foco"
	case "contact.lookup":
		return "sabio"
	case "action.fix.resolve_gaps_conversational", "action.fix.apply":
		return "mecanico"
	case "credentials.cpanel.connect", "credentials.smtp.check":
		return "hosting"
	default:
		return ""
	}
}
