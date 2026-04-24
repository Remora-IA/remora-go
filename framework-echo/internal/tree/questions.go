package tree

import "fmt"

// GenerateQuestions genera preguntas contextuales basadas en el tipo y título del nodo
// Estas son preguntas BASE. La IA puede complementar pero el GO garantiza un mínimo útil.
func GenerateQuestions(node *Node) []string {
	switch node.Type {
	case TypeAxiom:
		// Los axiomas ya están validados por observación, no necesitan preguntas
		return []string{}

	case TypeTheory:
		return generateTheoryQuestions(node)

	case TypeTask:
		return generateTaskQuestions(node)

	case TypePain:
		return generatePainQuestions(node)

	case TypeOpportunity:
		return generateOpportunityQuestions(node)

	default:
		return []string{}
	}
}

func generateTheoryQuestions(node *Node) []string {
	base := []string{
		fmt.Sprintf("Respecto a '%s': ¿esto calza con lo que pasa realmente?", node.Title),
		fmt.Sprintf("¿Qué parte de '%s' se repite y qué parte cambia caso a caso?", node.Title),
		fmt.Sprintf("¿Qué tendría que pasar para que '%s' deje de ser cierto?", node.Title),
	}
	return base
}

func generateTaskQuestions(node *Node) []string {
	base := []string{
		fmt.Sprintf("Para la tarea '%s': ¿cuánto tiempo le toma realizarla?", node.Title),
		fmt.Sprintf("¿Con qué frecuencia realiza '%s'? (diario/semanal/mensual)", node.Title),
		fmt.Sprintf("Cuando hace '%s', ¿qué parte le sale fluida y qué parte se siente pesada?", node.Title),
		fmt.Sprintf("¿Qué información necesita tener a mano para '%s' y dónde la busca hoy?", node.Title),
		fmt.Sprintf("¿Qué pasa cuando '%s' se retrasa o sale mal?", node.Title),
	}
	return base
}

func generatePainQuestions(node *Node) []string {
	base := []string{
		fmt.Sprintf("Sobre el problema '%s': ¿con qué frecuencia ocurre?", node.Title),
		fmt.Sprintf("¿Qué impacto concreto tiene '%s' en su trabajo diario?", node.Title),
		fmt.Sprintf("¿Qué hace hoy para convivir con '%s'?", node.Title),
		fmt.Sprintf("Si '%s' desapareciera mañana, ¿qué cambiaría primero?", node.Title),
	}
	return base
}

func generateOpportunityQuestions(node *Node) []string {
	base := []string{
		fmt.Sprintf("Para la oportunidad '%s': ¿qué dolor confirmado resuelve exactamente?", node.Title),
		fmt.Sprintf("¿Qué tendría que ser cierto para que '%s' encaje en la forma actual de trabajar?", node.Title),
		fmt.Sprintf("¿Qué señales indicarían que '%s' obligaría al usuario a adaptarse demasiado?", node.Title),
	}
	return base
}
