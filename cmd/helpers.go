// Package cmd provides CLI command implementations for projector.
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ideaspaper/projector/pkg/config"
	"github.com/ideaspaper/projector/pkg/models"
	"github.com/ideaspaper/projector/pkg/storage"
)

// TypeFilter specifies which project types to include when loading projects.
type TypeFilter struct {
	Favorites bool
	Git       bool
	SVN       bool
	Mercurial bool
	VSCode    bool
	Any       bool
}

// ShowAll returns true if no specific type filter is set.
func (f TypeFilter) ShowAll() bool {
	return !f.Favorites && !f.Git && !f.SVN && !f.Mercurial && !f.VSCode && !f.Any
}

// LoadFilteredProjects loads projects from storage based on the given type filter.
// It returns all matching projects from both favorites and cache.
func LoadFilteredProjects(store *storage.Storage, filter TypeFilter) ([]*models.Project, error) {
	var allProjects []*models.Project
	showAll := filter.ShowAll()

	// Load favorites
	if showAll || filter.Favorites {
		projects, err := store.LoadProjects()
		if err != nil {
			return nil, fmt.Errorf("failed to load projects: %w", err)
		}
		allProjects = append(allProjects, projects.Projects...)
	}

	// Load cached auto-detected projects
	if showAll || filter.Git || filter.SVN || filter.Mercurial || filter.VSCode || filter.Any {
		cache, err := store.LoadCache()
		if err == nil {
			if showAll || filter.Git {
				allProjects = append(allProjects, cache.Git...)
			}
			if showAll || filter.SVN {
				allProjects = append(allProjects, cache.SVN...)
			}
			if showAll || filter.Mercurial {
				allProjects = append(allProjects, cache.Mercurial...)
			}
			if showAll || filter.VSCode {
				allProjects = append(allProjects, cache.VSCode...)
			}
			if showAll || filter.Any {
				allProjects = append(allProjects, cache.Any...)
			}
		}
	}

	return allProjects, nil
}

// FilterEnabled returns only enabled projects from the given list.
func FilterEnabled(projects []*models.Project) []*models.Project {
	filtered := make([]*models.Project, 0, len(projects))
	for _, p := range projects {
		if p.Enabled {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// FilterByTag returns only projects that have the specified tag.
func FilterByTag(projects []*models.Project, tag string) []*models.Project {
	if tag == "" {
		return projects
	}
	filtered := make([]*models.Project, 0)
	for _, p := range projects {
		if p.HasTag(tag) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// FindProjectByName finds a project by name with exact or partial matching.
// Returns the matched project and any error.
// If multiple partial matches are found, returns an error with the matches.
func FindProjectByName(projects []*models.Project, name string) (*models.Project, []*models.Project, error) {
	// First try exact match (case-insensitive)
	for _, p := range projects {
		if strings.EqualFold(p.Name, name) {
			return p, nil, nil
		}
	}

	// Try partial match
	var matches []*models.Project
	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(name)) {
			matches = append(matches, p)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil, nil
	} else if len(matches) > 1 {
		return nil, matches, fmt.Errorf("multiple projects match '%s'", name)
	}

	return nil, nil, fmt.Errorf("project '%s' not found", name)
}

// ReadUserInput reads a line of input from stdin, handling edge cases properly.
func ReadUserInput() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// logVerbose prints a message if verbose mode is enabled.
func logVerbose(cfg *config.Config, format string, args ...interface{}) {
	if verbose {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
	}
}
