package handoff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type QuestionsQueue struct {
	CurrentSpeaker string           `json:"current_speaker"` // "echo" | "alfa"
	AlfaQuestions  []QueuedQuestion `json:"alfa_questions"`
	EchoQuestions  []QueuedQuestion `json:"echo_questions"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type QueuedQuestion struct {
	ID            string    `json:"id"`
	Text          string    `json:"text"`
	Status        string    `json:"status"` // "pending" | "answered"
	AskedAt       time.Time `json:"asked_at,omitempty"`
	AnsweredAt    time.Time `json:"answered_at,omitempty"`
	AnswerPreview string    `json:"answer_preview,omitempty"`
}

const (
	SpeakerEcho      = "echo"
	SpeakerAlfa      = "alfa"
	QuestionPending  = "pending"
	QuestionAnswered = "answered"
)

var defaultQueuePath = "temp/questions_queue.json"

func NewQuestionsQueue() *QuestionsQueue {
	return &QuestionsQueue{
		CurrentSpeaker: SpeakerEcho,
		AlfaQuestions:  []QueuedQuestion{},
		EchoQuestions:  []QueuedQuestion{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

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
	return &q, nil
}

func SaveQuestionsQueue(path string, q *QuestionsQueue) error {
	if path == "" {
		path = defaultQueuePath
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	q.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// GetNextAlfaQuestion returns the first pending Alfa question, or false if none
func (q *QuestionsQueue) GetNextAlfaQuestion() (QueuedQuestion, bool) {
	for _, qq := range q.AlfaQuestions {
		if qq.Status == QuestionPending {
			return qq, true
		}
	}
	return QueuedQuestion{}, false
}

// GetNextEchoQuestion returns the first pending Echo question, or false if none
func (q *QuestionsQueue) GetNextEchoQuestion() (QueuedQuestion, bool) {
	for _, qq := range q.EchoQuestions {
		if qq.Status == QuestionPending {
			return qq, true
		}
	}
	return QueuedQuestion{}, false
}

// GetCurrentQuestion returns the question for whoever has the speaker
func (q *QuestionsQueue) GetCurrentQuestion() (QueuedQuestion, string, bool) {
	if q.CurrentSpeaker == SpeakerAlfa {
		qq, ok := q.GetNextAlfaQuestion()
		return qq, SpeakerAlfa, ok
	}
	qq, ok := q.GetNextEchoQuestion()
	return qq, SpeakerEcho, ok
}

// AddAlfaQuestion adds a question to Alfa's queue
func (q *QuestionsQueue) AddAlfaQuestion(text string) {
	id := generateQuestionID("a", len(q.AlfaQuestions)+1)
	q.AlfaQuestions = append(q.AlfaQuestions, QueuedQuestion{
		ID:     id,
		Text:   text,
		Status: QuestionPending,
	})
}

// AddEchoQuestion adds a question to Echo's queue
func (q *QuestionsQueue) AddEchoQuestion(text string) {
	id := generateQuestionID("e", len(q.EchoQuestions)+1)
	q.EchoQuestions = append(q.EchoQuestions, QueuedQuestion{
		ID:     id,
		Text:   text,
		Status: QuestionPending,
	})
}

// MarkQuestionAnswered marks a question as answered
func (q *QuestionsQueue) MarkQuestionAnswered(speaker string, questionID string, preview string) {
	if speaker == SpeakerEcho {
		for i := range q.EchoQuestions {
			if q.EchoQuestions[i].ID == questionID {
				q.EchoQuestions[i].Status = QuestionAnswered
				q.EchoQuestions[i].AnsweredAt = time.Now()
				q.EchoQuestions[i].AnswerPreview = preview
				break
			}
		}
		return
	}

	for i := range q.AlfaQuestions {
		if q.AlfaQuestions[i].ID == questionID {
			q.AlfaQuestions[i].Status = QuestionAnswered
			q.AlfaQuestions[i].AnsweredAt = time.Now()
			q.AlfaQuestions[i].AnswerPreview = preview
			break
		}
	}
}

// SetSpeaker changes who has the speaking turn
func (q *QuestionsQueue) SetSpeaker(speaker string) {
	q.CurrentSpeaker = speaker
}

// HasPendingQuestions checks if there are pending questions for a speaker
func (q *QuestionsQueue) HasPendingQuestions(speaker string) bool {
	if speaker == SpeakerAlfa {
		_, ok := q.GetNextAlfaQuestion()
		return ok
	}
	_, ok := q.GetNextEchoQuestion()
	return ok
}

// PendingCount returns how many questions are pending for a speaker
func (q *QuestionsQueue) PendingCount(speaker string) int {
	count := 0
	questions := q.AlfaQuestions
	if speaker == SpeakerEcho {
		questions = q.EchoQuestions
	}
	for _, qq := range questions {
		if qq.Status == QuestionPending {
			count++
		}
	}
	return count
}

// AskQuestion marks a question as asked (user was prompted)
func (q *QuestionsQueue) AskQuestion(speaker string, questionID string) {
	if speaker == SpeakerEcho {
		for i := range q.EchoQuestions {
			if q.EchoQuestions[i].ID == questionID {
				q.EchoQuestions[i].AskedAt = time.Now()
				break
			}
		}
		return
	}

	for i := range q.AlfaQuestions {
		if q.AlfaQuestions[i].ID == questionID {
			q.AlfaQuestions[i].AskedAt = time.Now()
			break
		}
	}
}

func generateQuestionID(prefix string, num int) string {
	return prefix + "_" + fmt.Sprintf("%03d", num)
}

func init() {
	// placeholder
}
