package launcher

// Icons maps semantic IconRole values to display strings.
// Nerd Font glyphs are used when available; otherwise fall back to ASCII.
var Icons = map[IconRole]string{
	IconRoleCalc:     "󰃬 ", // nf-md-calculator
	IconRoleUnit:     "󱉸 ", // nf-md-ruler
	IconRoleCurrency: "󰀧 ", // nf-md-currency-usd
	IconRoleApp:      "󰀻 ", // nf-md-apps
	IconRoleScript:   "󰐊 ", // nf-md-play
	IconRoleHistory:  "󰋚 ", // nf-md-history
	IconRoleError:    " ", // nf-fa-exclamation_circle
	IconRoleLoading:  " ", // nf-fa-spinner
}

// ASCIIIcons is a plain-text fallback for environments without Nerd Fonts.
var ASCIIIcons = map[IconRole]string{
	IconRoleCalc:     "= ",
	IconRoleUnit:     "~ ",
	IconRoleCurrency: "$ ",
	IconRoleApp:      "> ",
	IconRoleScript:   "! ",
	IconRoleHistory:  "* ",
	IconRoleError:    "x ",
	IconRoleLoading:  ". ",
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
