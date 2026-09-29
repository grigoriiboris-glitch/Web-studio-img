package comfyui

type workflowRequest struct {
	Prompt map[string]any `json:"prompt"`
	ClientID string `json:"client_id,omitempty"`
}

type promptResponse struct {
	PromptID string `json:"prompt_id"`
	Number int `json:"number"`
	NodeErrors map[string]any `json:"node_errors,omitempty"`
}

type uploadResponse struct {
	Name string `json:"name"`
	Subfolder string `json:"subfolder"`
	Type string `json:"type"`
}

type historyEntry struct {
	Prompt map[string]any `json:"prompt,omitempty"`
	Outputs map[string]nodeOutput `json:"outputs,omitempty"`
	Status *historyStatus `json:"status,omitempty"`
}

type historyStatus struct {
	StatusStr string `json:"status_str,omitempty"`
	Completed bool `json:"completed,omitempty"`
	Messages [][]any `json:"messages,omitempty"`
}

type nodeOutput struct {
	Images []outputImage `json:"images,omitempty"`
}

type outputImage struct {
	Filename string `json:"filename"`
	Subfolder string `json:"subfolder,omitempty"`
	Type string `json:"type,omitempty"`
}
