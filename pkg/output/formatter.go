package output

import (
	"fmt"
	"strings"

	"github.com/fatih/color"

	"github.com/ideaspaper/projector/pkg/models"
)

// Formatter handles output formatting
type Formatter struct {
	colored bool

	// Colors
	nameColor    *color.Color
	pathColor    *color.Color
	tagColor     *color.Color
	kindColor    *color.Color
	successColor *color.Color
	errorColor   *color.Color
	warnColor    *color.Color
	infoColor    *color.Color
}

// NewFormatter creates a new formatter
func NewFormatter(colored bool) *Formatter {
	return &Formatter{
		colored:      colored,
		nameColor:    color.New(color.FgWhite, color.Bold),
		pathColor:    color.New(color.FgCyan),
		tagColor:     color.New(color.FgMagenta),
		kindColor:    color.New(color.FgYellow),
		successColor: color.New(color.FgGreen),
		errorColor:   color.New(color.FgRed),
		warnColor:    color.New(color.FgYellow),
		infoColor:    color.New(color.FgBlue),
	}
}

// FormatProject formats a single project for display
func (f *Formatter) FormatProject(p *models.Project, showPath bool) string {
	var sb strings.Builder

	// Project name
	if f.colored {
		sb.WriteString(f.nameColor.Sprint(p.Name))
	} else {
		sb.WriteString(p.Name)
	}

	// Tags
	if len(p.Tags) > 0 {
		sb.WriteString(" ")
		if f.colored {
			sb.WriteString(f.tagColor.Sprintf("[%s]", strings.Join(p.Tags, ", ")))
		} else {
			sb.WriteString(fmt.Sprintf("[%s]", strings.Join(p.Tags, ", ")))
		}
	}

	// Disabled indicator
	if !p.Enabled {
		if f.colored {
			sb.WriteString(f.warnColor.Sprint(" (disabled)"))
		} else {
			sb.WriteString(" (disabled)")
		}
	}

	// Path
	if showPath {
		sb.WriteString("\n  ")
		if f.colored {
			sb.WriteString(f.pathColor.Sprint(p.RootPath))
		} else {
			sb.WriteString(p.RootPath)
		}
	}

	return sb.String()
}

// FormatProjectList formats a list of projects
func (f *Formatter) FormatProjectList(projects []*models.Project, showPath bool, grouped bool) string {
	if len(projects) == 0 {
		return f.FormatInfo("No projects found.")
	}

	var sb strings.Builder

	if grouped {
		// Group by kind
		groups := make(map[models.ProjectKind][]*models.Project)
		for _, p := range projects {
			groups[p.Kind] = append(groups[p.Kind], p)
		}

		kindOrder := []models.ProjectKind{
			models.KindFavorite,
			models.KindGit,
			models.KindSVN,
			models.KindMercurial,
			models.KindVSCode,
			models.KindAny,
		}

		for _, kind := range kindOrder {
			ps, ok := groups[kind]
			if !ok || len(ps) == 0 {
				continue
			}

			// Group header
			header := f.getKindHeader(kind)
			if f.colored {
				sb.WriteString(f.kindColor.Sprint(header))
			} else {
				sb.WriteString(header)
			}
			sb.WriteString("\n")

			for _, p := range ps {
				sb.WriteString("  ")
				sb.WriteString(f.FormatProject(p, showPath))
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	} else {
		for _, p := range projects {
			sb.WriteString(f.FormatProject(p, showPath))
			sb.WriteString("\n")
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// FormatProjectCompact formats a project in a compact single-line format
func (f *Formatter) FormatProjectCompact(p *models.Project, index int) string {
	var sb strings.Builder

	// Index
	if f.colored {
		sb.WriteString(f.infoColor.Sprintf("[%d]", index))
	} else {
		sb.WriteString(fmt.Sprintf("[%d]", index))
	}
	sb.WriteString(" ")

	// Name
	if f.colored {
		sb.WriteString(f.nameColor.Sprint(p.Name))
	} else {
		sb.WriteString(p.Name)
	}

	// Path (truncated)
	sb.WriteString(" - ")
	path := p.RootPath
	if len(path) > 50 {
		path = "..." + path[len(path)-47:]
	}
	if f.colored {
		sb.WriteString(f.pathColor.Sprint(path))
	} else {
		sb.WriteString(path)
	}

	return sb.String()
}

// getKindHeader returns the header for a project kind
func (f *Formatter) getKindHeader(kind models.ProjectKind) string {
	switch kind {
	case models.KindFavorite:
		return "Favorites"
	case models.KindGit:
		return "Git Repositories"
	case models.KindSVN:
		return "SVN Repositories"
	case models.KindMercurial:
		return "Mercurial Repositories"
	case models.KindVSCode:
		return "VS Code Workspaces"
	case models.KindAny:
		return "Other Projects"
	default:
		return "Projects"
	}
}

// FormatSuccess formats a success message
func (f *Formatter) FormatSuccess(msg string) string {
	if f.colored {
		return f.successColor.Sprint("✓ " + msg)
	}
	return "✓ " + msg
}

// FormatError formats an error message
func (f *Formatter) FormatError(msg string) string {
	if f.colored {
		return f.errorColor.Sprint("✗ " + msg)
	}
	return "✗ " + msg
}

// FormatWarning formats a warning message
func (f *Formatter) FormatWarning(msg string) string {
	if f.colored {
		return f.warnColor.Sprint("⚠ " + msg)
	}
	return "⚠ " + msg
}

// FormatInfo formats an info message
func (f *Formatter) FormatInfo(msg string) string {
	if f.colored {
		return f.infoColor.Sprint("ℹ " + msg)
	}
	return "ℹ " + msg
}
