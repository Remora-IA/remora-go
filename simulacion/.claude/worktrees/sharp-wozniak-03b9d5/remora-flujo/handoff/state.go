package handoff

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Role string

const (
	RoleEcho  Role = "echo"
	RoleAlfa  Role = "alfa"
	RoleBravo Role = "bravo"
)

type Status string

const (
	StatusOff Status = "off"
	StatusOn  Status = "on"
)

type EventType string

const (
	EventStarted          EventType = "started"
	EventEchoReadyForAlfa EventType = "echo_ready_for_alfa"
	EventEchoWaitingUser  EventType = "echo_waiting_user"
	EventEchoUserAnswered EventType = "echo_user_answered"
	EventAlfaReadyBravo   EventType = "alfa_ready_for_bravo"
	EventAlfaNeedsEcho    EventType = "alfa_needs_echo"
	EventAlfaCededToEcho  EventType = "alfa_ceded_to_echo"
	EventAlfaAsksQuestion EventType = "alfa_asks_question"
	EventBravoNeedsEcho   EventType = "bravo_needs_echo"
	EventBravoDone        EventType = "bravo_done"
	EventError            EventType = "error"
)

type RoleState struct {
	Status Status    `json:"status"`
	OnRuns int       `json:"on_runs"`
	Prompt string    `json:"prompt,omitempty"`
	Since  time.Time `json:"since,omitempty"`
}

type Event struct {
	At      time.Time `json:"at"`
	Role    Role      `json:"role"`
	Type    EventType `json:"type"`
	Message string    `json:"message,omitempty"`
}

type State struct {
	Version int                `json:"version"`
	Roles   map[Role]RoleState `json:"roles"`
	Events  []Event            `json:"events"`
	Meta    map[string]string  `json:"meta,omitempty"`
}

func NewState() *State {
	return &State{
		Version: 1,
		Roles: map[Role]RoleState{
			RoleEcho:  {Status: StatusOff},
			RoleAlfa:  {Status: StatusOff},
			RoleBravo: {Status: StatusOff},
		},
		Events: []Event{},
		Meta:   map[string]string{},
	}
}

func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NewState(), nil
	}
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.Roles == nil {
		state.Roles = NewState().Roles
	}
	if state.Meta == nil {
		state.Meta = map[string]string{}
	}
	return &state, nil
}

func Save(path string, state *State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func (s *State) Start(role Role, prompt string) {
	rs := s.Roles[role]
	rs.Status = StatusOn
	rs.OnRuns++
	rs.Prompt = prompt
	rs.Since = time.Now()
	s.Roles[role] = rs
	s.Events = append(s.Events, Event{At: time.Now(), Role: role, Type: EventStarted, Message: prompt})
}

func (s *State) Done(role Role, event EventType, message string) {
	rs := s.Roles[role]
	rs.Status = StatusOff
	rs.Since = time.Time{}
	s.Roles[role] = rs
	s.Events = append(s.Events, Event{At: time.Now(), Role: role, Type: event, Message: message})
}

func (s *State) LastEvent() (Event, bool) {
	if len(s.Events) == 0 {
		return Event{}, false
	}
	return s.Events[len(s.Events)-1], true
}

func (s *State) NextRole() (Role, string, bool) {
	last, ok := s.LastEvent()
	if !ok {
		return RoleEcho, "flujo_nuevo", true
	}
	switch last.Type {
	case EventEchoReadyForAlfa:
		return RoleAlfa, "echo_listo_para_alfa", true
	case EventAlfaReadyBravo:
		return RoleBravo, "alfa_listo_para_bravo", true
	case EventAlfaNeedsEcho:
		return RoleEcho, "alfa_necesita_pregunta", true
	case EventAlfaCededToEcho:
		return RoleEcho, "echo_tiene_palabra", true
	case EventAlfaAsksQuestion:
		return RoleEcho, "alfa_pregunta", true
	case EventEchoUserAnswered:
		// Usuario respondió, decide quién tiene la palabra según cola
		queue, err := LoadQuestionsQueue("")
		if err == nil && queue != nil {
			if queue.CurrentSpeaker == SpeakerAlfa && queue.HasPendingQuestions(SpeakerAlfa) {
				return RoleAlfa, "respuesta_para_alfa", true
			}
		}
		return RoleEcho, "echo_continua", true
	case EventBravoNeedsEcho:
		return RoleEcho, "bravo_necesita_pregunta", true
	case EventBravoDone:
		return RoleEcho, "bravo_entrego_resultado_para_validar", true
	case EventError:
		return RoleEcho, "error_requiere_recuperacion", true
	case EventEchoWaitingUser:
		return "", "echo_espera_usuario", false
	case EventStarted:
		return "", fmt.Sprintf("%s_sigue_en_mando", last.Role), false
	default:
		return "", fmt.Sprintf("evento_no_mapeado:%s", last.Type), false
	}
}

func ParseRole(value string) (Role, error) {
	switch Role(value) {
	case RoleEcho, RoleAlfa, RoleBravo:
		return Role(value), nil
	default:
		return "", fmt.Errorf("rol desconocido: %s", value)
	}
}

func ParseEvent(value string) (EventType, error) {
	switch EventType(value) {
	case EventStarted, EventEchoReadyForAlfa, EventEchoWaitingUser, EventEchoUserAnswered,
		EventAlfaReadyBravo, EventAlfaNeedsEcho, EventAlfaCededToEcho, EventAlfaAsksQuestion,
		EventBravoNeedsEcho, EventBravoDone, EventError:
		return EventType(value), nil
	default:
		return "", fmt.Errorf("evento desconocido: %s", value)
	}
}
