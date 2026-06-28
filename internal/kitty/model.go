package kitty

type OSWindow struct {
	ID          int
	IsActive    bool
	LastFocused bool
	Tabs        []Tab
}

type Tab struct {
	ID          int
	IsActive    bool
	IsFocused   bool
	Title       string
	SessionName string
	Windows     []Window
}

type ForegroundProcess struct {
	PID     int
	Cmdline []string
	Cwd     string
}

type Window struct {
	ID                        int
	IsActive                  bool
	Cmdline                   []string
	ForegroundProcesses       []ForegroundProcess
	Title                     string
	SessionName               string
	LastReportedCmdline       string
	HasActivitySinceLastFocus bool
	LastFocusedAt             float64
	Cwd                       string
	UserVars                  map[string]string
}

type Session struct {
	Name          string
	Path          string
	TabCount      int
	IsActive      bool
	LastFocusedAt float64
}

type rawOSWindow struct {
	ID          int      `json:"id"`
	IsActive    bool     `json:"is_active"`
	LastFocused bool     `json:"last_focused"`
	Tabs        []rawTab `json:"tabs"`
	TabsAlt     []rawTab `json:"tabs:"`
}

type rawTab struct {
	ID          int         `json:"id"`
	IsActive    bool        `json:"is_active"`
	IsFocused   bool        `json:"is_focused"`
	Title       string      `json:"title"`
	SessionName string      `json:"session_name"`
	Windows     []rawWindow `json:"windows"`
}

type rawWindow struct {
	ID                        int                    `json:"id"`
	IsActive                  bool                   `json:"is_active"`
	Cmdline                   []string               `json:"cmdline"`
	ForegroundProcesses       []rawForegroundProcess `json:"foreground_processes"`
	Title                     string                 `json:"title"`
	SessionName               string                 `json:"session_name"`
	LastReportedCmdline       string                 `json:"last_reported_cmdline"`
	HasActivitySinceLastFocus bool                   `json:"has_activity_since_last_focus"`
	LastFocusedAt             float64                `json:"last_focused_at"`
	Cwd                       string                 `json:"cwd"`
	UserVars                  map[string]string      `json:"user_vars"`
}

type rawForegroundProcess struct {
	PID     int      `json:"pid"`
	Cmdline []string `json:"cmdline"`
	Cwd     string   `json:"cwd"`
}
