package launcher

import (
	"os"
	"runtime"
	"strings"

	"github.com/elentok/blf/internal/fuzzyfinder"
)

type settingPane struct {
	name  string
	path  string
	glyph string
}

var settingPanes = []settingPane{
	{"Apple ID", "/System/Library/PreferencePanes/AppleIDPrefPane.prefPane", "󰀸"},   // nf-md-apple
	{"Appearance", "/System/Library/PreferencePanes/Appearance.prefPane", "󰸌"},      // nf-md-palette
	{"Accessibility", "/System/Library/PreferencePanes/UniversalAccessPref.prefPane", "󰚶"}, // nf-md-human
	{"Battery", "/System/Library/PreferencePanes/Battery.prefPane", "󰁹"},            // nf-md-battery
	{"Bluetooth", "/System/Library/PreferencePanes/Bluetooth.prefPane", "󰂯"},        // nf-md-bluetooth
	{"Date & Time", "/System/Library/PreferencePanes/DateAndTime.prefPane", "󰥔"},    // nf-md-clock-outline
	{"Displays", "/System/Library/PreferencePanes/Displays.prefPane", "󰍺"},          // nf-md-monitor
	{"Dock & Menu Bar", "/System/Library/PreferencePanes/Dock.prefPane", "󰀻"},       // nf-md-dock-window
	{"Energy Saver", "/System/Library/PreferencePanes/EnergySaver.prefPane", "󰤄"},   // nf-md-leaf
	{"Extensions", "/System/Library/PreferencePanes/Extensions.prefPane", "󰏗"},      // nf-md-puzzle
	{"Family Sharing", "/System/Library/PreferencePanes/FamilySharingPrefPane.prefPane", "󰉋"}, // nf-md-account-supervisor
	{"Internet Accounts", "/System/Library/PreferencePanes/InternetAccounts.prefPane", "󰴢"}, // nf-md-web
	{"Keyboard", "/System/Library/PreferencePanes/Keyboard.prefPane", "󰌌"},          // nf-md-keyboard
	{"Language & Region", "/System/Library/PreferencePanes/Localization.prefPane", "󰗊"}, // nf-md-translate
	{"Mouse", "/System/Library/PreferencePanes/Mouse.prefPane", "󰍽"},                // nf-md-mouse
	{"Network", "/System/Library/PreferencePanes/Network.prefPane", "󰛳"},            // nf-md-ip-network
	{"Notifications", "/System/Library/PreferencePanes/Notifications.prefPane", "󰂚"}, // nf-md-bell
	{"Passwords", "/System/Library/PreferencePanes/Passwords.prefPane", "󰌆"},        // nf-md-key-variant
	{"Printers & Scanners", "/System/Library/PreferencePanes/PrintAndScan.prefPane", "󰐪"}, // nf-md-printer
	{"Privacy & Security", "/System/Library/PreferencePanes/Security.prefPane", "󰒃"}, // nf-md-shield-lock
	{"Screen Time", "/System/Library/PreferencePanes/ScreenTime.prefPane", "󱑁"},     // nf-md-timer
	{"Sharing", "/System/Library/PreferencePanes/SharingPref.prefPane", "󰒋"},        // nf-md-share
	{"Software Update", "/System/Library/PreferencePanes/SoftwareUpdate.prefPane", "󰚰"}, // nf-md-update
	{"Sound", "/System/Library/PreferencePanes/Sound.prefPane", "󰕾"},                // nf-md-volume-high
	{"Spotlight", "/System/Library/PreferencePanes/Spotlight.prefPane", "󰍉"},        // nf-md-magnify
	{"Startup Disk", "/System/Library/PreferencePanes/StartupDisk.prefPane", "󰋊"},   // nf-md-harddisk
	{"Time Machine", "/System/Library/PreferencePanes/TimeMachine.prefPane", "󰔩"},   // nf-md-backup-restore
	{"Touch ID & Password", "/System/Library/PreferencePanes/TouchID.prefPane", "󰵃"}, // nf-md-fingerprint
	{"Trackpad", "/System/Library/PreferencePanes/Trackpad.prefPane", "󰟸"},          // nf-md-gesture-tap
	{"Users & Groups", "/System/Library/PreferencePanes/Accounts.prefPane", "󰀄"},    // nf-md-account-multiple
	{"Wallet & Apple Pay", "/System/Library/PreferencePanes/Wallet.prefPane", "󰠃"},  // nf-md-wallet
	{"Wallpaper", "/System/Library/PreferencePanes/DesktopScreenEffectsPref.prefPane", "󰸌"}, // nf-md-image
}

// SettingsProvider fuzzy-matches macOS System Settings panes.
type SettingsProvider struct {
	weight float64
	panes  []settingPane
	names  []string
}

var _ Provider = (*SettingsProvider)(nil)

func NewSettingsProvider(weight float64) *SettingsProvider {
	if runtime.GOOS != "darwin" {
		return &SettingsProvider{weight: weight}
	}
	available := make([]settingPane, 0, len(settingPanes))
	for _, p := range settingPanes {
		if _, err := os.Stat(p.path); err == nil {
			available = append(available, p)
		}
	}
	names := make([]string, len(available))
	for i, p := range available {
		names[i] = p.name
	}
	return &SettingsProvider{weight: weight, names: names, panes: available}
}

func (p *SettingsProvider) Query(input string) []Result {
	if Classify(input) == Computational {
		return nil
	}
	q := strings.TrimSpace(input)
	if q == "" {
		return nil
	}

	matches := fuzzyfinder.Find(q, p.names)
	results := make([]Result, 0, len(matches))
	for _, m := range matches {
		pane := p.panes[m.Index]
		lowerName := strings.ToLower(pane.name)
		lowerQ := strings.ToLower(q)
		results = append(results, Result{
			Title:         pane.name,
			Subtitle:      "(settings)",
			Icon:          IconRoleSettings,
			IconGlyph:     pane.glyph,
			Source:        "settings",
			Weight:        p.weight,
			FuzzyScore:    m.Score,
			MatchRanges:   m.MatchedIndexes,
			IsExactMatch:  lowerName == lowerQ,
			IsPrefixMatch: strings.HasPrefix(lowerName, lowerQ),
			Action:        Action{Type: ActionOpen, Target: pane.path},
		})
	}
	return results
}

// LookupResult implements TargetLookupProvider by finding the settings pane
// whose path matches action.Target.
func (p *SettingsProvider) LookupResult(action Action) (Result, bool) {
	if action.Type != ActionOpen {
		return Result{}, false
	}
	for _, pane := range p.panes {
		if pane.path == action.Target {
			return Result{
				Title:     pane.name,
				Subtitle:  "(settings)",
				Icon:      IconRoleSettings,
				IconGlyph: pane.glyph,
				Action:    Action{Type: ActionOpen, Target: pane.path},
			}, true
		}
	}
	return Result{}, false
}
