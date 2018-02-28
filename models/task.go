package models

// Task is the data model of a task in A01 system
type Task struct {
	Annotation    string                 `json:"annotation,omitempty"`
	Duration      int                    `json:"duration,omitempty"`
	ID            int                    `json:"id,omitempty"`
	Name          string                 `json:"name,omitempty"`
	Result        string                 `json:"result,omitempty"`
	ResultDetails map[string]interface{} `json:"result_details,omitempty"`
	RunID         int                    `json:"run_id,omitempty"`
	Settings      TaskSetting            `json:"settings,omitempty"`
	Status        string                 `json:"status,omitempty"`
}

// TaskSetting is the setting data model of A01Task
type TaskSetting struct {
	Version     string            `json:"ver,omitempty"`
	Execution   map[string]string `json:"execution,omitempty"`
	Classifier  map[string]string `json:"classifier,omitempty"`
	Miscellanea map[string]string `json:"misc,omitempty"`
}
