package auditdata

import (
	"encoding/json"
	"fmt"
	"os"
)

type Finding struct {
	ID          string                 `json:"id"`
	Rule        string                 `json:"rule"`
	Severity    string                 `json:"severity"`
	Endpoint    string                 `json:"endpoint"`
	RecordID    string                 `json:"record_id"`
	Field       string                 `json:"field,omitempty"`
	Message     string                 `json:"message"`
	Evidence    map[string]interface{} `json:"evidence,omitempty"`
	Suggestion  string                 `json:"suggestion,omitempty"`
	AutoFixable bool                   `json:"auto_fixable"`
	FixHint     map[string]interface{} `json:"fix_hint,omitempty"`
}

type Dataset struct {
	Endpoints map[string][]map[string]interface{} `json:"-"`
	Raw       map[string]interface{}              `json:"-"`
}

func LoadFindings(path string) ([]Finding, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseFindings(raw)
}

func ParseFindings(raw []byte) ([]Finding, error) {
	var wrap struct {
		Findings []Finding `json:"findings"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	return wrap.Findings, nil
}

func LoadDataset(path string) (*Dataset, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dataset: %w", err)
	}
	return ParseDataset(raw)
}

func ParseDataset(raw []byte) (*Dataset, error) {
	var top map[string]interface{}
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("parse dataset: %w", err)
	}
	endpoints := top
	if ep, ok := top["endpoints"].(map[string]interface{}); ok {
		endpoints = ep
	}
	if tables, ok := top["tables"].(map[string]interface{}); ok {
		endpoints = tables
	}
	out := &Dataset{Endpoints: map[string][]map[string]interface{}{}, Raw: top}
	for name, v := range endpoints {
		arr, ok := v.([]interface{})
		if !ok {
			continue
		}
		records := make([]map[string]interface{}, 0, len(arr))
		for _, e := range arr {
			if rec, ok := e.(map[string]interface{}); ok {
				records = append(records, rec)
			}
		}
		out.Endpoints[name] = records
	}
	return out, nil
}

func (d *Dataset) Save(path string) error {
	if epMap, ok := d.Raw["endpoints"].(map[string]interface{}); ok {
		for name, recs := range d.Endpoints {
			arr := make([]interface{}, 0, len(recs))
			for _, r := range recs {
				arr = append(arr, r)
			}
			epMap[name] = arr
		}
		d.Raw["endpoints"] = epMap
	} else if tableMap, ok := d.Raw["tables"].(map[string]interface{}); ok {
		for name, recs := range d.Endpoints {
			arr := make([]interface{}, 0, len(recs))
			for _, r := range recs {
				arr = append(arr, r)
			}
			tableMap[name] = arr
		}
		d.Raw["tables"] = tableMap
	} else {
		for name, recs := range d.Endpoints {
			arr := make([]interface{}, 0, len(recs))
			for _, r := range recs {
				arr = append(arr, r)
			}
			d.Raw[name] = arr
		}
	}
	data, err := json.MarshalIndent(d.Raw, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
