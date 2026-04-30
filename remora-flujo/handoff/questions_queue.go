package handoff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// QuestionsQueue es la cola estándar de preguntas pendientes para el usuario.
// Es agnóstica al framework: cualquier framework declarado en Frameworks puede
// encolar preguntas. La API REST consume esta cola para mostrarle UNA pregunta
// a la vez al usuario y enrutar su respuesta de vuelta al framework dueño.
//
// Nota: las helpers AddAlfaQuestion / AddEchoQuestion / GetNextAlfaQuestion /
// GetNextEchoQuestion / MarkQuestionAnswered(speaker, ...) / HasPendingQuestions /
// PendingCount / AskQuestion(speaker, ...) se mantienen para compatibilidad con
// el CLI `flujo` (cmd/flujo). Internamente delegan al modelo nuevo.
type QuestionsQueue struct {
	Frameworks     []string         `json:"frameworks"`
	CurrentSpeaker string           `json:"current_speaker"`
	Questions      []QueuedQuestion `json:"questions"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`

	// Legacy: persistido por versiones anteriores. Se migra en Load y se vacía al guardar.
	LegacyAlfa []QueuedQuestion `json:"alfa_questions,omitempty"`
	LegacyEcho []QueuedQuestion `json:"echo_questions,omitempty"`
}

// QueuedQuestion es una pregunta de un framework hacia el usuario.
type QueuedQuestion struct {
	ID         string    `json:"id"`
	Framework  string    `json:"framework"`              // "echo", "alfa", "whatsapp", ...
	ExternalID string    `json:"external_id,omitempty"`  // ID nativo del framework, ej "th_001", "oq_001"
	Text       string    `json:"text"`
	AskVia     string    `json:"ask_via,omitempty"`      // "" = directa | "echo" = reformula vía Echo
	Status     string    `json:"status"`                 // pending | asked | answered
	Answer     string    `json:"answer,omitempty"`
	AskedAt    time.Time `json:"asked_at,omitempty"`
	AnsweredAt time.Time `json:"answered_at,omitempty"`
}

// Estados y nombres canónicos.
const (
	SpeakerEcho      = "echo"
	SpeakerAlfa      = "alfa"
	QuestionPending  = "pending"
	QuestionAsked    = "asked"
	QuestionAnswered = "answered"
)

var defaultQueuePath = "temp/questions_queue.json"

// NewQuestionsQueue crea una cola vacía con los frameworks activos declarados.
// Si no se pasan frameworks, asume el flujo histórico ["echo","alfa"].
func NewQuestionsQueue(frameworks ...string) *QuestionsQueue {
	if len(frameworks) == 0 {
		frameworks = []string{SpeakerEcho, SpeakerAlfa}
	}
	return &QuestionsQueue{
		Frameworks:     append([]string(nil), frameworks...),
		CurrentSpeaker: frameworks[0],
		Questions:      []QueuedQuestion{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// LoadQuestionsQueue lee la cola desde disco. Si el archivo no existe devuelve
// una cola vacía (no error). Migra silenciosamente el formato legacy.
func LoadQuestionsQueue(path string) (*QuestionsQueue, error) {
	if path == "" {
		path = defaultQueuePath
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewQuestionsQueue(), nil
	}
	if err != nil {
		return nil, err
	}
	var q QuestionsQueue
	if err := json.Unmarshal(data, &q); err != nil {
		return nil, err
	}
	q.migrateLegacy()
	if len(q.Frameworks) == 0 {
		q.Frameworks = []string{SpeakerEcho, SpeakerAlfa}
	}
	if q.CurrentSpeaker == "" {
		q.CurrentSpeaker = q.Frameworks[0]
	}
	if q.Questions == nil {
		q.Questions = []QueuedQuestion{}
	}
	return &q, nil
}

// SaveQuestionsQueue persiste la cola en disco creando el directorio si falta.
func SaveQuestionsQueue(path string, q *QuestionsQueue) error {
	if path == "" {
		path = defaultQueuePath
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	q.UpdatedAt = time.Now()
	q.LegacyAlfa = nil
	q.LegacyEcho = nil
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// migrateLegacy convierte los slices alfa_questions/echo_questions del formato
// anterior al slice unificado Questions.
func (q *QuestionsQueue) migrateLegacy() {
	if len(q.LegacyAlfa) == 0 && len(q.LegacyEcho) == 0 {
		return
	}
	for _, qq := range q.LegacyEcho {
		if qq.Framework == "" {
			qq.Framework = SpeakerEcho
		}
		q.Questions = append(q.Questions, qq)
	}
	for _, qq := range q.LegacyAlfa {
		if qq.Framework == "" {
			qq.Framework = SpeakerAlfa
		}
		q.Questions = append(q.Questions, qq)
	}
	q.LegacyAlfa = nil
	q.LegacyEcho = nil
}

// AddQuestion encola una pregunta nueva del framework indicado y devuelve su
// ID interno de cola. externalID es opcional: si está set, se persiste para
// que el driver dueño pueda mapear de vuelta al ID nativo del framework.
func (q *QuestionsQueue) AddQuestion(framework, externalID, text, askVia string) string {
	if framework == "" {
		framework = q.CurrentSpeaker
	}
	id := generateQuestionID(framework, q.nextSeq(framework))
	q.Questions = append(q.Questions, QueuedQuestion{
		ID:         id,
		Framework:  framework,
		ExternalID: externalID,
		Text:       text,
		AskVia:     askVia,
		Status:     QuestionPending,
	})
	return id
}

// GetNextPending devuelve la primera pregunta pendiente (FIFO global).
func (q *QuestionsQueue) GetNextPending() (QueuedQuestion, bool) {
	for _, qq := range q.Questions {
		if qq.Status == QuestionPending {
			return qq, true
		}
	}
	return QueuedQuestion{}, false
}

// GetNextPendingFor devuelve la primera pregunta pendiente del framework dado.
func (q *QuestionsQueue) GetNextPendingFor(framework string) (QueuedQuestion, bool) {
	for _, qq := range q.Questions {
		if qq.Status == QuestionPending && qq.Framework == framework {
			return qq, true
		}
	}
	return QueuedQuestion{}, false
}

// MarkAnswered marca por ID una pregunta como respondida.
func (q *QuestionsQueue) MarkAnswered(questionID, answer string) bool {
	for i := range q.Questions {
		if q.Questions[i].ID == questionID {
			q.Questions[i].Status = QuestionAnswered
			q.Questions[i].Answer = answer
			q.Questions[i].AnsweredAt = time.Now()
			return true
		}
	}
	return false
}

// MarkAsked marca una pregunta como ya formulada al usuario (sin respuesta aún).
func (q *QuestionsQueue) MarkAsked(questionID string) bool {
	for i := range q.Questions {
		if q.Questions[i].ID == questionID {
			if q.Questions[i].Status == QuestionPending {
				q.Questions[i].Status = QuestionAsked
			}
			q.Questions[i].AskedAt = time.Now()
			return true
		}
	}
	return false
}

// HasPending indica si hay alguna pregunta pendiente sin importar framework.
func (q *QuestionsQueue) HasPending() bool {
	_, ok := q.GetNextPending()
	return ok
}

// HasPendingFor indica si el framework dado tiene preguntas pendientes.
func (q *QuestionsQueue) HasPendingFor(framework string) bool {
	_, ok := q.GetNextPendingFor(framework)
	return ok
}

// PendingCountFor cuenta preguntas pendientes para un framework.
func (q *QuestionsQueue) PendingCountFor(framework string) int {
	count := 0
	for _, qq := range q.Questions {
		if qq.Status == QuestionPending && qq.Framework == framework {
			count++
		}
	}
	return count
}

// SetSpeaker cambia el framework con la palabra.
func (q *QuestionsQueue) SetSpeaker(framework string) {
	q.CurrentSpeaker = framework
}

// GetCurrentQuestion devuelve la próxima pregunta para quien tenga la palabra.
// Devuelve también el framework por conveniencia.
func (q *QuestionsQueue) GetCurrentQuestion() (QueuedQuestion, string, bool) {
	if q.CurrentSpeaker == "" {
		qq, ok := q.GetNextPending()
		return qq, qq.Framework, ok
	}
	qq, ok := q.GetNextPendingFor(q.CurrentSpeaker)
	if ok {
		return qq, q.CurrentSpeaker, true
	}
	// Fallback: cualquier pendiente.
	qq, ok = q.GetNextPending()
	return qq, qq.Framework, ok
}

func (q *QuestionsQueue) nextSeq(framework string) int {
	count := 0
	for _, qq := range q.Questions {
		if qq.Framework == framework {
			count++
		}
	}
	return count + 1
}

func generateQuestionID(framework string, num int) string {
	prefix := framework
	if len(prefix) > 4 {
		prefix = prefix[:4]
	}
	if prefix == "" {
		prefix = "q"
	}
	return fmt.Sprintf("%s_%03d", prefix, num)
}

// ---------------------------------------------------------------------------
// Helpers de compatibilidad con el CLI `flujo` (cmd/flujo). NO usar en código
// nuevo: prefiere AddQuestion / GetNextPendingFor / MarkAnswered.
// ---------------------------------------------------------------------------

// AddAlfaQuestion: legacy. Equivale a AddQuestion("alfa", "", text, "").
func (q *QuestionsQueue) AddAlfaQuestion(text string) {
	q.AddQuestion(SpeakerAlfa, "", text, "")
}

// AddEchoQuestion: legacy. Equivale a AddQuestion("echo", "", text, "").
func (q *QuestionsQueue) AddEchoQuestion(text string) {
	q.AddQuestion(SpeakerEcho, "", text, "")
}

// GetNextAlfaQuestion: legacy.
func (q *QuestionsQueue) GetNextAlfaQuestion() (QueuedQuestion, bool) {
	return q.GetNextPendingFor(SpeakerAlfa)
}

// GetNextEchoQuestion: legacy.
func (q *QuestionsQueue) GetNextEchoQuestion() (QueuedQuestion, bool) {
	return q.GetNextPendingFor(SpeakerEcho)
}

// MarkQuestionAnswered: legacy. El parámetro speaker se ignora porque el ID es
// único; se mantiene la firma para no romper callers.
func (q *QuestionsQueue) MarkQuestionAnswered(_ string, questionID string, preview string) {
	q.MarkAnswered(questionID, preview)
}

// HasPendingQuestions: legacy.
func (q *QuestionsQueue) HasPendingQuestions(speaker string) bool {
	return q.HasPendingFor(speaker)
}

// PendingCount: legacy.
func (q *QuestionsQueue) PendingCount(speaker string) int {
	return q.PendingCountFor(speaker)
}

// AskQuestion: legacy. Marca como asked.
func (q *QuestionsQueue) AskQuestion(_ string, questionID string) {
	q.MarkAsked(questionID)
}
