package autonomia

import (
	"strings"
)

type Mode string

const (
	ModeGeneral Mode = "general"
	ModeCase    Mode = "case"
)

type Entity struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Candidate struct {
	Entity   Entity   `json:"entity"`
	Reason   string   `json:"reason"`
	Evidence []string `json:"evidence,omitempty"`
}

type SessionState struct {
	Mode            Mode   `json:"mode"`
	LastModeHandoff string `json:"last_mode_handoff,omitempty"`
	SelectedEntity  Entity `json:"selected_entity,omitempty"`
}

type Context struct {
	Title      string   `json:"title"`
	Subtitle   string   `json:"subtitle"`
	PanelTitle string   `json:"panel_title"`
	Memory     string   `json:"memory,omitempty"`
	Highlights []string `json:"highlights,omitempty"`
}

type Response struct {
	Mode             Mode         `json:"mode"`
	Text             string       `json:"text"`
	Actions          []string     `json:"actions"`
	Context          Context      `json:"context"`
	CaseContext      *Entity      `json:"case_context,omitempty"`
	ForecastBlocked  bool         `json:"forecast_blocked,omitempty"`
	GeneralFraming   bool         `json:"general_framing,omitempty"`
	HandoffReason    string       `json:"handoff_reason,omitempty"`
	Shortlist        []Candidate  `json:"shortlist,omitempty"`
	Guardrail        string       `json:"guardrail,omitempty"`
	TurnType         string       `json:"turn_type,omitempty"`
	NextSessionState SessionState `json:"next_session_state"`
}

var primaryFocus = Entity{Type: "project", ID: "000060-0001", Label: "Consultoría Regulatoria"}

var shortlist = []Candidate{
	{
		Entity:   primaryFocus,
		Reason:   "Pendiente visible con anomalía temporal; conviene revisar antes de tratarlo como mora reciente.",
		Evidence: []string{"1 pendiente visible", "brecha temporal 6.2 años", "subconjunto activo y cobrable"},
	},
	{
		Entity:   Entity{Type: "segment", ID: "active-projects", Label: "Proyectos activos"},
		Reason:   "Subuniverso operativo para separar actividad vigente de histórico cerrado.",
		Evidence: []string{"2 proyectos activos", "3 cargos en activos", "1 pendiente visible"},
	},
	{
		Entity:   Entity{Type: "data-quality", ID: "governance-gaps", Label: "Vacíos de gobernanza"},
		Reason:   "Riesgos de seguimiento por falta de responsables y campos de vencimiento.",
		Evidence: []string{"9 proyectos sin responsable", "sin due_date", "sin vínculo pago→cargo"},
	},
}

func InitialState() SessionState {
	return SessionState{
		Mode:            ModeGeneral,
		LastModeHandoff: "Inicio en panorama general",
	}
}

func Bootstrap() Response {
	state := InitialState()
	return Response{
		Mode:             ModeGeneral,
		Text:             "Ya dejé preparada la simulación con dos modos: panorama general y análisis individual. El modo inicial arranca desde el universo analizado de la simulación actual. Hoy la fuente embebida contiene 1 entidad raíz, 18 proyectos, 79 cargos, 79 documentos y 78 pagos. Primero sostengo alcance, cobertura, temporalidad y shortlist comparativa; solo bajo a caso cuando lo confirmes.",
		Actions:          []string{"Seguir analizando", "Abrir análisis individual", "Pausar análisis"},
		Context:          generalContext(state.LastModeHandoff),
		GeneralFraming:   true,
		Shortlist:        shortlist,
		NextSessionState: state,
	}
}

func HandleMessage(state SessionState, message string) Response {
	state = ensureState(state)
	normalized := normalize(message)

	switch {
	case isForecastPrompt(normalized):
		resp := Response{
			Mode:             ModeGeneral,
			Text:             "Con esta fuente no puedo estimar recuperación futura con rigor. No tengo due_date, tampoco monto pendiente completo por cargo/documento ni un vínculo directo pago → obligación. Lo que sí puedo darte es una lectura honesta de cobro histórico observado, cargos abiertos visibles y focos que ameritan drill-down.",
			Actions:          []string{"Seguir analizando", "Abrir análisis individual", "Pausar análisis"},
			Context:          generalContext(state.LastModeHandoff),
			ForecastBlocked:  true,
			GeneralFraming:   true,
			Shortlist:        shortlist,
			TurnType:         "guardrail",
			NextSessionState: state,
		}
		return resp
	case state.Mode == ModeGeneral && isGreeting(normalized):
		return generalSocialResponse(state, socialText(normalized))
	case state.Mode == ModeGeneral && isRepairPrompt(normalized):
		return generalRepairResponse(state)
	case state.Mode == ModeGeneral && strings.Contains(normalized, "que universo estoy viendo"):
		return generalResponse(state, "Estás viendo el panorama general del universo analizado de la simulación actual. El alcance real es el dataset embebido, que hoy parte de 1 entidad raíz y no de una cartera multi-cliente completa. La cobertura sí permite leer proyectos, acuerdos, cargos, documentos y pagos, además de segmentar por activo/cobrable, tipo, área, responsable y año. Los límites importantes son claros: no hay due_date, no hay monto por cargo/documento y no existe vínculo directo pago→cargo.", true)
	case state.Mode == ModeGeneral && strings.Contains(normalized, "que patrones generales ves aqui"):
		return generalResponse(state, "A nivel general veo tres capas. Primero, la composición está distribuida en 18 proyectos, 19 acuerdos, 79 cargos, 79 documentos y 78 pagos. Segundo, la temporalidad observable muestra actividad administrativa por años: cargos 2022: 62 · 2021: 10 · 2024: 3 · 2023: 2 y pagos 2021: 47 · 2020: 26 · 2023: 3 · 2022: 2. Tercero, aparece una shortlist comparativa: Consultoría Regulatoria, Proyectos activos y Vacíos de gobernanza. Todavía no convierto esa comparación en recomendación operativa única.", true)
	case state.Mode == ModeGeneral && strings.Contains(normalized, "que foco amerita drill-down"):
		return generalShortlistResponse(state, "La shortlist de drill-down queda así: 1) Consultoría Regulatoria, por pendiente visible con anomalía temporal; 2) Proyectos activos, para separar actividad vigente de histórico; 3) Vacíos de gobernanza, por riesgos de seguimiento y falta de campos críticos. Esto no es todavía una orden de cobro: sigo en panorama general hasta que confirmes bajar a caso.")
	case state.Mode == ModeGeneral && isOperationalPriorityPrompt(normalized):
		resp := generalShortlistResponse(state, "No respondería “a quién cobrar primero” como una recomendación operativa única con esta fuente. Lo correcto es mantenerlo como shortlist analítica: Consultoría Regulatoria requiere validación por pendiente visible y anomalía temporal; Proyectos activos sirve para aislar el frente vigente; Vacíos de gobernanza explica riesgos de seguimiento. Para convertir esto en acción de cobro necesito confirmación de caso y validación interna de cargo/documento.")
		resp.Guardrail = "priorizacion_operativa"
		resp.TurnType = "guardrail"
		return resp
	case state.Mode == ModeGeneral && (strings.Contains(normalized, "abramos el analisis individual del foco principal") || strings.Contains(normalized, "abrir analisis individual")):
		next := state
		next.Mode = ModeCase
		next.SelectedEntity = primaryFocus
		next.LastModeHandoff = "Foco elegido desde el panorama general: Consultoría Regulatoria"
		return Response{
			Mode:             ModeCase,
			Text:             "Del panorama general, el foco más claro es Consultoría Regulatoria. Lo elijo porque concentra el pendiente visible dentro del subconjunto activo y cobrable y porque además arrastra señales que pueden ser de conciliación y no solo de mora. Ya en análisis individual, vuelvo a la pregunta correcta: qué evidencia real tengo sobre este caso y qué acción concreta conviene después.",
			Actions:          []string{"Seguir analizando", "Volver al panorama general", "Pausar este caso"},
			Context:          caseContext(next.LastModeHandoff, primaryFocus),
			CaseContext:      &primaryFocus,
			HandoffReason:    next.LastModeHandoff,
			TurnType:         "handoff",
			NextSessionState: next,
		}
	case state.Mode == ModeCase && strings.Contains(normalized, "que evidencia hay y que accion sugeririas"):
		state.SelectedEntity = ensureEntity(state.SelectedEntity)
		return Response{
			Mode:             ModeCase,
			Text:             "Ya en análisis individual, la evidencia útil está concentrada en Consultoría Regulatoria. Veo 3 cargos dentro del subconjunto activo y cobrable, con 1 pendiente visible y una cobertura cargo→documento de 79/79. Además, el pendiente focal aparece como cargo 3189 ligado a Consultoría Regulatoria, y arrastra una brecha temporal de 6.2 años. La acción sugerida es concreta: validar internamente cargo y documento, confirmar si el abierto sigue vigente y, solo si se sostiene, recién después preparar una gestión puntual sobre ese foco.",
			Actions:          []string{"Seguir analizando", "Volver al panorama general", "Pausar este caso"},
			Context:          caseContext(state.LastModeHandoff, state.SelectedEntity),
			CaseContext:      &state.SelectedEntity,
			TurnType:         "analysis",
			NextSessionState: state,
		}
	case state.Mode == ModeCase && strings.Contains(normalized, "volvamos al panorama general"):
		next := state
		next.Mode = ModeGeneral
		entity := ensureEntity(state.SelectedEntity)
		next.LastModeHandoff = "Regreso al panorama general con memoria del caso revisado: " + entity.Label
		return generalResponse(next, "En panorama general, el sujeto correcto vuelve a ser el universo analizado de esta simulación. Conservo memoria del caso revisado para no perder continuidad, pero ya no estoy hablando desde el caso sino desde alcance, composición, temporalidad y focos comparables.", true)
	case state.Mode == ModeGeneral && strings.Contains(normalized, "volvamos al panorama general"):
		return generalResponse(state, "Ya estoy en panorama general del universo analizado.", true)
	default:
		if state.Mode == ModeCase {
			state.SelectedEntity = ensureEntity(state.SelectedEntity)
			return Response{
				Mode:             ModeCase,
				Text:             "Sigo en análisis individual del foco Consultoría Regulatoria. Si quieres, puedo profundizar evidencia, urgencia o acción recomendada sin volver todavía al panorama general.",
				Actions:          []string{"Seguir analizando", "Volver al panorama general", "Pausar este caso"},
				Context:          caseContext(state.LastModeHandoff, state.SelectedEntity),
				CaseContext:      &state.SelectedEntity,
				TurnType:         "fallback",
				NextSessionState: state,
			}
		}
		return generalResponse(state, "Sigo en panorama general del universo analizado. Si quieres, puedo describir alcance, patrones, focos o abrir análisis individual del foco principal.", true)
	}
}

func generalResponse(state SessionState, text string, framing bool) Response {
	state.Mode = ModeGeneral
	return Response{
		Mode:             ModeGeneral,
		Text:             text,
		Actions:          []string{"Seguir analizando", "Abrir análisis individual", "Pausar análisis"},
		Context:          generalContext(state.LastModeHandoff),
		GeneralFraming:   framing,
		Shortlist:        shortlist,
		TurnType:         "analysis",
		NextSessionState: state,
	}
}

func generalSocialResponse(state SessionState, text string) Response {
	resp := generalResponse(state, text, true)
	resp.TurnType = "social"
	resp.Actions = []string{"Ver universo", "Ver focos", "Pausar análisis"}
	return resp
}

func generalRepairResponse(state SessionState) Response {
	resp := generalResponse(state, "Puedo aclararte cualquiera de estas tres cosas: el universo que estoy viendo, los patrones generales o los focos para drill-down. Si te refieres a otra cosa, reformúlame la pregunta y lo tomo desde ahí.", true)
	resp.TurnType = "repair"
	resp.Actions = []string{"Ver universo", "Ver patrones generales", "Ver focos"}
	return resp
}

func generalShortlistResponse(state SessionState, text string) Response {
	resp := generalResponse(state, text, true)
	resp.Shortlist = shortlist
	return resp
}

func generalContext(memory string) Context {
	return Context{
		Title:      "Contexto del panorama",
		Subtitle:   "Sujeto actual: universo analizado de la simulación",
		PanelTitle: "Agenda general",
		Memory:     memory,
		Highlights: []string{
			"Alcance: panorama general de la simulación",
			"Cobertura: dataset embebido actual con 1 entidad raíz",
			"Límites: sin due_date, sin monto por cargo/documento y sin vínculo pago→cargo",
		},
	}
}

func caseContext(memory string, entity Entity) Context {
	entity = ensureEntity(entity)
	return Context{
		Title:      "Contexto del caso",
		Subtitle:   "Fuente delimitada: panalbit.sqlite",
		PanelTitle: "Agenda individual",
		Memory:     memory,
		Highlights: []string{
			"Caso focalizado: " + entity.Label,
			"Cliente: Nicolas, Hickle and Conroy",
			"Motivo: " + memory,
		},
	}
}

func ensureState(state SessionState) SessionState {
	if state.Mode == "" {
		return InitialState()
	}
	if state.Mode == ModeCase {
		state.SelectedEntity = ensureEntity(state.SelectedEntity)
	}
	if state.LastModeHandoff == "" {
		state.LastModeHandoff = "Inicio en panorama general"
	}
	return state
}

func ensureEntity(entity Entity) Entity {
	if entity.Label == "" {
		return primaryFocus
	}
	return entity
}

func isGreeting(normalized string) bool {
	switch normalized {
	case "hola", "buenas", "buen dia", "buenos dias", "buenas tardes", "buenas noches", "como estas", "que tal", "gracias", "ok", "vale", "dale", "perfecto":
		return true
	default:
		return false
	}
}

func socialText(normalized string) string {
	switch normalized {
	case "como estas", "que tal":
		return "Bien, listo para ayudarte. Ahora mismo estoy en panorama general; si quieres, te resumo el universo o vamos directo a comparar focos."
	case "gracias":
		return "De nada. Sigo listo para ayudarte desde el panorama general cuando quieras continuar."
	case "ok", "vale", "dale", "perfecto":
		return "Perfecto. Mantengo el panorama general listo para seguir cuando me indiques por dónde avanzar."
	default:
		return "Hola. Estoy listo para ayudarte desde el panorama general. Si quieres, empezamos por el alcance o por los focos visibles."
	}
}

func isRepairPrompt(normalized string) bool {
	switch normalized {
	case "que", "como", "no entendi", "no entiendo", "explicame", "a que te refieres":
		return true
	default:
		return false
	}
}

func isForecastPrompt(normalized string) bool {
	return strings.Contains(normalized, "cuanto voy a recuperar este mes")
}

func isOperationalPriorityPrompt(normalized string) bool {
	return strings.Contains(normalized, "a quien") && (strings.Contains(normalized, "cobrar primero") || strings.Contains(normalized, "le cobro primero"))
}

func normalize(s string) string {
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "¿", "", "?", "",
		"¡", "", "!", "", ",", "", ".", "", ";", "", ":", "", "\n", " ",
	)
	return strings.Join(strings.Fields(strings.ToLower(replacer.Replace(s))), " ")
}
