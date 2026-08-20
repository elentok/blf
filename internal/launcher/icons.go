package launcher

import "strings"

// CurrencyIcons maps ISO currency codes to dedicated Nerd Font glyphs.
// Currencies not in this map fall back to IconRoleCurrency.
var CurrencyIcons = map[string]string{
	"USD": "", // nf-fa-usd
	"EUR": "", // nf-fa-eur
	"GBP": "", // nf-fa-gbp
	"ILS": "", // nf-fa-ils
}

// appIconPatterns maps lowercase substrings to Nerd Font glyphs.
// Checked in order; first match wins.
var appIconPatterns = []struct{ sub, glyph string }{
	{"firefox", "󰈹"},            // nf-md-firefox
	{"chrome", "󰊯"},             // nf-md-google-chrome
	{"safari", "󰀹"},             // nf-md-apple-safari
	{"spotify", "󰓇"},            // nf-md-spotify
	{"steam", ""},              // nf-fa-steam
	{"keepass", "󰌆"},            // nf-md-key-variant
	{"1password", "󰌆"},          // nf-md-key-variant
	{"passwords", "󰌆"},          // nf-md-key-variant
	{"visual studio code", "󰨞"}, // nf-md-visual-studio-code
	{"vscode", "󰨞"},             // nf-md-visual-studio-code
	{"terminal", "󰆍"},           // nf-md-console
	{"slack", "󰒱"},              // nf-md-slack
	{"obsidian", "󱓧"},           // nf-md-obsidian
	{"notion", ""},
	{"finder", "󰀶"},             // nf-md-folder
	{"mail", "󰇮"},               // nf-md-email
	{"calendar", "󰃭"},           // nf-md-calendar
	{"settings", "󰒓"},           // nf-md-cog
	{"system preferences", "󰒓"}, // nf-md-cog
	{"whatsapp", ""},           // nf-fa-whatsapp
	{"telegram", ""},           // nf-fa-telegram
	{"chatgpt", "󰭻"},            // nf-md-robot
	{"prism launcher", "󰍳"},     // nf-md-minecraft
	{"prusa", "󰹛"},              // nf-md-printer-3d
	{"freecad", ""},            // nf-md-cube-outline
	{"openscad", ""},           // nf-md-cube-scan
	{"prisma", ""},             // nf-md-database
	{"contacts", "󰛋"},           // nf-md-contacts
	{"vlc", "󰕼"},
	{"book", ""},
	{"messages", ""},
}

// AppIconGlyph returns the Nerd Font glyph for an app by title substring match,
// or "" if no pattern matches.
func AppIconGlyph(title string) string {
	lower := strings.ToLower(title)
	for _, p := range appIconPatterns {
		if strings.Contains(lower, p.sub) {
			return p.glyph
		}
	}
	return ""
}

// Icons maps semantic IconRole values to display strings.
// Nerd Font glyphs are used when available; otherwise fall back to ASCII.
var Icons = map[IconRole]string{
	IconRoleCalc:      "󰃬 ", // nf-md-calculator
	IconRoleUnit:      "󱉸 ", // nf-md-ruler
	IconRoleCurrency:  "󰀧 ", // nf-md-currency-usd
	IconRoleApp:       "󰀻 ", // nf-md-apps
	IconRoleScript:    "󰐊 ", // nf-md-play
	IconRoleHistory:   "󰋚 ", // nf-md-history
	IconRoleError:     "󰀪 ", // nf-md-alert
	IconRoleLoading:   "󰔟 ", // nf-md-loading
	IconRoleSettings:  "󰒓 ", // nf-md-cog
	IconRoleDirectory: "󰉋 ", // nf-md-folder
	IconRoleCommand:   "󰆍 ", // nf-md-console (same glyph used for "terminal" apps below)
	IconRoleSnippet:   "󰆒 ", // nf-md-content-copy
	IconRoleAI:        "󰧑 ", // nf-md-creation
	IconRoleImprove:   "󰚰 ", // nf-md-auto-fix
}

// ASCIIIcons is a plain-text fallback for environments without Nerd Fonts.
var ASCIIIcons = map[IconRole]string{
	IconRoleCalc:      "= ",
	IconRoleUnit:      "~ ",
	IconRoleCurrency:  "$ ",
	IconRoleApp:       "> ",
	IconRoleScript:    "! ",
	IconRoleHistory:   "* ",
	IconRoleError:     "x ",
	IconRoleLoading:   ". ",
	IconRoleSettings:  "* ",
	IconRoleDirectory: "d ",
	IconRoleCommand:   "% ",
	IconRoleSnippet:   "c ",
	IconRoleAI:        "@ ",
	IconRoleImprove:   "^ ",
}

// Icon returns the icon string for a given role.
// If useNerdFont is false, ASCII fallbacks are used.
func Icon(role IconRole, useNerdFont bool) string {
	if useNerdFont {
		if s, ok := Icons[role]; ok {
			return s
		}
	}
	if s, ok := ASCIIIcons[role]; ok {
		return s
	}
	return "  "
}
