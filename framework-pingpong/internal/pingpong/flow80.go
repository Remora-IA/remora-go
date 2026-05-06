package pingpong

import (
	"fmt"
	"path/filepath"
)

type TutorState struct {
	Mode            string   `json:"mode"`
	Say             string   `json:"say"`
	Awaiting        string   `json:"awaiting"`
	AllowedCommands []string `json:"allowed_commands"`
	Current         ViewStep `json:"current,omitempty"`
	Progress        string   `json:"progress,omitempty"`
}

type ViewStep struct {
	Batch       string `json:"batch,omitempty"`
	Step        string `json:"step,omitempty"`
	File        string `json:"file,omitempty"`
	Instruction string `json:"instruction,omitempty"`
}

func (c *Client) Next() (*Result, error) {
	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}
	state, err := tutorState(p)
	if err != nil {
		return nil, err
	}
	return &Result{
		Success: true,
		Message: state.Say,
		Data:    state,
	}, nil
}

func (c *Client) Check(fileOverride string) (*Result, error) {
	verify, err := c.Verify(fileOverride)
	if err != nil {
		return nil, err
	}
	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}
	state, stateErr := tutorState(p)
	if stateErr == nil {
		state.Awaiting = "tutor_judgment"
		state.AllowedCommands = []string{"accept", "next", "peek", "search", "symbols"}
	}
	return &Result{
		Success: true,
		Message: "Check completo. Juzgá con data.verify.data.inspection. Si el paso actual está cumplido, ejecutá ./pingpong accept.",
		Data: map[string]interface{}{
			"state":           state,
			"verify":          verify,
			"action_required": "judge_current_step_only",
			"rule":            "No elijas IDs. Si está cumplido, usa accept. Si no, explica solo el bloqueo del paso actual.",
		},
	}, nil
}

func (c *Client) Accept() (*Result, error) {
	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}
	if !p.Active {
		return nil, fmt.Errorf("no hay proyecto activo. Usa 'start --goal' primero")
	}
	current, err := currentStepForProgress(p)
	if err != nil {
		return nil, err
	}
	_, err = c.Done(fmt.Sprintf("%d", current.ID))
	if err != nil {
		return nil, err
	}
	p2, loadErr := c.loadOrCreate()
	if loadErr == nil && p2.Active {
		if state, stateErr := tutorState(p2); stateErr == nil {
			return &Result{
				Success: true,
				Message: state.Say,
				Data: map[string]interface{}{
					"accepted": true,
					"state":    state,
				},
			}, nil
		}
	}
	return &Result{
		Success: true,
		Message: "Todos los pasos fueron aceptados. Pasá a la fase final con run.",
		Data: map[string]interface{}{
			"accepted":     true,
			"completedAll": true,
		},
	}, nil
}

func tutorState(p *Progress) (TutorState, error) {
	if !p.Active {
		return TutorState{Mode: "inactive", Say: "No hay proyecto activo.", Awaiting: "start", AllowedCommands: []string{"start"}}, nil
	}
	current, err := currentStepForProgress(p)
	if err != nil {
		return TutorState{}, err
	}
	batchInfo := buildBatchInfo(p)
	mode := "normal"
	stepLabel := fmt.Sprintf("%d/%d", batchInfo.CurrentBatchStep, len(batchInfo.Steps))
	if p.Detour != nil {
		mode = "detour"
		stepLabel = fmt.Sprintf("%d/%d", p.Detour.CurrentStep, len(p.Detour.Steps))
	} else if p.InMinitest {
		mode = "mini-test"
	}
	fileLabel := ""
	fileName := ""
	if current.File != "" {
		fileName = filepath.Base(current.File)
		fileLabel = fmt.Sprintf(" [%s]", fileName)
	}
	prefix := "Paso"
	if mode == "detour" {
		prefix = "Sub-paso"
	} else if mode == "mini-test" {
		prefix = "Mini-test paso"
	}
	say := fmt.Sprintf("%s %s%s: %s", prefix, stepLabel, fileLabel, current.Instruction)
	return TutorState{
		Mode:            mode,
		Say:             say,
		Awaiting:        "user_code_change",
		AllowedCommands: []string{"check"},
		Current: ViewStep{
			Batch:       fmt.Sprintf("%d/%d", batchInfo.Index, batchInfo.TotalBatches),
			Step:        stepLabel,
			File:        fileName,
			Instruction: current.Instruction,
		},
		Progress: overallProgress(p),
	}, nil
}
