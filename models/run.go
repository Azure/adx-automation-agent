package models

// Run is the data structure of A01 run
type Run struct {
	ID       int                    `json:"id,omitempty"`
	Name     string                 `json:"name"`
	Settings map[string]interface{} `json:"settings"`
	Details  map[string]string      `json:"details"`
}
