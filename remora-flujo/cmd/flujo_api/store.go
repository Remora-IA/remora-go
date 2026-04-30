package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"remora-flujo/handoff"
)

const convDir = "temp/api_conversations"

// Conversation es la red comunicacional entre el usuario y N frameworks.
// Frameworks declara los frameworks activos en orden (round-robin).
// Models permite override del modelo por framework (ej {"alfa":"llama-4-scout"}).
// Si un framework no aparece, se usa el modelo declarado en su manifest.
// UserAnswerCount cuenta respuestas del usuario; lo usan las reglas de composición.
type Conversation struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Frameworks      []string          `json:"frameworks"`
	Models          map[string]string `json:"models,omitempty"`
	UserAnswerCount int               `json:"user_answer_count"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// Message es una entrada de la conversación. role="user" o role="framework".
// Cuando role="framework", Framework indica el framework dueño y QuestionID
// referencia la pregunta en la cola (handoff.QuestionsQueue).
// Resources son inputs no-textuales del usuario (imágenes, archivos).
type Message struct {
	ID         string             `json:"id"`
	Role       string             `json:"role"`
	Framework  string             `json:"framework,omitempty"`
	Content    string             `json:"content"`
	QuestionID string             `json:"question_id,omitempty"`
	AskVia     string             `json:"ask_via,omitempty"`
	Resources  []MessageResource  `json:"resources,omitempty"`
	Timestamp  time.Time          `json:"timestamp"`
}

// MessageResource es un recurso adjunto al mensaje del usuario. Path es
// absoluto y vive bajo temp/api_conversations/<conv>/uploads/.
type MessageResource struct {
	Type     string `json:"type"`           // "image" | "audio" | "file"
	Path     string `json:"path"`           // absoluto
	Name     string `json:"name,omitempty"` // nombre original
	MimeType string `json:"mime,omitempty"`
}

func convPath(id string) string      { return filepath.Join(convDir, id) }
func metaPath(id string) string      { return filepath.Join(convPath(id), "meta.json") }
func messagesPath(id string) string  { return filepath.Join(convPath(id), "messages.json") }
func queuePath(id string) string     { return filepath.Join(convPath(id), "questions_queue.json") }
func uploadsDir(id string) string    { return filepath.Join(convPath(id), "uploads") }

// storeResources copia cada resource al directorio uploads/ de la conversación
// y devuelve los MessageResource con paths absolutos al copy. Esto da
// trazabilidad: aunque el origen desaparezca, la conv conserva sus inputs.
//
// Si el path es relativo, se interpreta relativo a CWD. Si el archivo no
// existe, devuelve error de inmediato (no copiamos placeholders).
func storeResources(convID string, in []MessageResource) ([]MessageResource, error) {
	if len(in) == 0 {
		return nil, nil
	}
	dir := uploadsDir(convID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	out := make([]MessageResource, 0, len(in))
	for i, r := range in {
		if r.Path == "" {
			return nil, fmt.Errorf("resource[%d]: path requerido", i)
		}
		src, err := os.Open(r.Path)
		if err != nil {
			return nil, fmt.Errorf("resource[%d]: %w", i, err)
		}
		name := r.Name
		if name == "" {
			name = filepath.Base(r.Path)
		}
		stamp := fmt.Sprintf("%d_%s", time.Now().UnixNano(), name)
		dstPath := filepath.Join(dir, stamp)
		dst, err := os.Create(dstPath)
		if err != nil {
			src.Close()
			return nil, fmt.Errorf("resource[%d]: %w", i, err)
		}
		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			dst.Close()
			return nil, fmt.Errorf("resource[%d]: %w", i, err)
		}
		src.Close()
		dst.Close()

		t := r.Type
		if t == "" {
			t = "file"
		}
		abs, _ := filepath.Abs(dstPath)
		out = append(out, MessageResource{
			Type:     t,
			Path:     abs,
			Name:     name,
			MimeType: r.MimeType,
		})
	}
	return out, nil
}

func loadConv(id string) (*Conversation, error) {
	data, err := os.ReadFile(metaPath(id))
	if err != nil {
		return nil, err
	}
	var c Conversation
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func saveConv(c *Conversation) error {
	if err := os.MkdirAll(convPath(c.ID), 0755); err != nil {
		return err
	}
	c.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath(c.ID), append(data, '\n'), 0644)
}

func listConvs() ([]Conversation, error) {
	entries, err := os.ReadDir(convDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Conversation{}, nil
		}
		return nil, err
	}
	out := []Conversation{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if c, err := loadConv(e.Name()); err == nil {
			out = append(out, *c)
		}
	}
	return out, nil
}

func deleteConv(id string) error {
	return os.RemoveAll(convPath(id))
}

func loadMessages(convID string) ([]Message, error) {
	data, err := os.ReadFile(messagesPath(convID))
	if err != nil {
		if os.IsNotExist(err) {
			return []Message{}, nil
		}
		return nil, err
	}
	var msgs []Message
	if err := json.Unmarshal(data, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func saveMessages(convID string, msgs []Message) error {
	if err := os.MkdirAll(convPath(convID), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(messagesPath(convID), append(data, '\n'), 0644)
}

func appendMessage(convID string, m Message) error {
	msgs, err := loadMessages(convID)
	if err != nil {
		return err
	}
	msgs = append(msgs, m)
	return saveMessages(convID, msgs)
}

func loadQueue(convID string) (*handoff.QuestionsQueue, error) {
	return handoff.LoadQuestionsQueue(queuePath(convID))
}

func saveQueue(convID string, q *handoff.QuestionsQueue) error {
	return handoff.SaveQuestionsQueue(queuePath(convID), q)
}

func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
