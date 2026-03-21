package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ASCII art logo for freshMango.
// Line 1 contains the leaf/stem "_)" which is rendered in green;
// the remaining lines form the text and are rendered in accent/amber.
const logo = `     _)
  __               _      __  __
 / _|_ _ ___  ___ | |__  |  \/  |__ _ _ _  __ _ ___
|  _| '_/ -_)(_-< | '_ \ | |\/| / _` + "`" + ` | ' \/ _` + "`" + ` / _ \
|_| |_| \___)/___||_||_| |_|  |_\__,_|_||_\__, \___/
                                           |___/`

// logoPlain is the raw logo without any ANSI styling.
const logoPlain = `     _)
  __               _      __  __
 / _|_ _ ___  ___ | |__  |  \/  |__ _ _ _  __ _ ___
|  _| '_/ -_)(_-< | '_ \ | |\/| / _` + "`" + ` | ' \/ _` + "`" + ` / _ \
|_| |_| \___)/___||_||_| |_|  |_\__,_|_||_\__, \___/
                                           |___/`

// RenderLogo returns the ASCII logo with Lipgloss coloring:
// the leaf/stem on line 1 in green, the rest in accent/amber.
func RenderLogo() string {
	lines := strings.Split(logo, "\n")
	styled := make([]string, len(lines))

	leafStyle := lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	textStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)

	for i, line := range lines {
		if i == 0 {
			// First line is the leaf/stem — render in green
			styled[i] = leafStyle.Render(line)
		} else {
			styled[i] = textStyle.Render(line)
		}
	}

	return strings.Join(styled, "\n")
}

// LogoPlain returns the plain monochrome ASCII logo (for README or fallback).
func LogoPlain() string {
	return logoPlain
}
