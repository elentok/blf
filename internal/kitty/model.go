package kitty

type OSWindow struct {
	ID          int
	IsActive    bool
	LastFocused bool
	Tabs        []Tab
}

type Tab struct {
	ID        int
	IsActive  bool
	IsFocused bool
	Title     string
}

type Session struct {
	Name     string
	Path     string
	TabCount int
}

type rawOSWindow struct {
	ID          int      `json:"id"`
	IsActive    bool     `json:"is_active"`
	LastFocused bool     `json:"last_focused"`
	Tabs        []rawTab `json:"tabs"`
	TabsAlt     []rawTab `json:"tabs:"`
}

type rawTab struct {
	ID        int    `json:"id"`
	IsActive  bool   `json:"is_active"`
	IsFocused bool   `json:"is_focused"`
	Title     string `json:"title"`
}
