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

type Window struct {
	LastFocusedAt float64
	SessionName   string
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
	LastFocusedAt float64 `json:"last_focused_at"`
	SessionName   string  `json:"session_name"`
}
