package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ideaspaper/projector/pkg/config"
	"github.com/ideaspaper/projector/pkg/models"
	"github.com/ideaspaper/projector/pkg/output"
	"github.com/ideaspaper/projector/pkg/storage"
)

var (
	selectTag string
)

// selectCmd represents the select command
var selectCmd = &cobra.Command{
	Use:   "select [project-name]",
	Short: "Select a project and output its path",
	Long: `Select a project and output its path to stdout.

If no project name is provided, an interactive selection is shown.
This is useful for scripting and shell integration.

Examples:
  # Interactive selection
  projector select

  # Select by name
  projector select myproject

  # Filter interactive selection by tag
  projector select --tag Work

Shell function for cd:
  pjcd() {
    local dir
    dir=$(projector select)
    [ -n "$dir" ] && [ -d "$dir" ] && cd "$dir"
  }`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSelect,
}

func init() {
	rootCmd.AddCommand(selectCmd)

	selectCmd.Flags().StringVarP(&selectTag, "tag", "t", "", "filter projects by tag")
}

func runSelect(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize storage
	store, err := storage.NewStorage(cfg.GetProjectsLocation())
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Collect all projects
	allProjects, err := store.LoadAllProjects()
	if err != nil {
		return fmt.Errorf("failed to load projects: %w", err)
	}

	// Filter enabled only
	filtered := make([]*models.Project, 0)
	for _, p := range allProjects {
		if p.Enabled {
			filtered = append(filtered, p)
		}
	}
	allProjects = filtered

	// Filter by tag if specified
	if selectTag != "" {
		filtered := make([]*models.Project, 0)
		for _, p := range allProjects {
			if p.HasTag(selectTag) {
				filtered = append(filtered, p)
			}
		}
		allProjects = filtered
	}

	if len(allProjects) == 0 {
		return fmt.Errorf("no projects found")
	}

	// Find project
	var selectedProject *models.Project

	if len(args) > 0 {
		projectName := args[0]

		// First try exact match
		for _, p := range allProjects {
			if strings.EqualFold(p.Name, projectName) {
				selectedProject = p
				break
			}
		}

		// If no exact match, try partial match
		if selectedProject == nil {
			var matches []*models.Project
			for _, p := range allProjects {
				if strings.Contains(strings.ToLower(p.Name), strings.ToLower(projectName)) {
					matches = append(matches, p)
				}
			}

			if len(matches) == 1 {
				selectedProject = matches[0]
			} else if len(matches) > 1 {
				// Multiple matches - show selection
				formatter := output.NewFormatter(!noColor && cfg.ShowColors)
				fmt.Fprintln(os.Stderr, formatter.FormatWarning(fmt.Sprintf("Multiple projects match '%s':", projectName)))
				for _, p := range matches {
					fmt.Fprintf(os.Stderr, "  - %s (%s)\n", p.Name, p.RootPath)
				}
				return fmt.Errorf("please be more specific")
			} else {
				return fmt.Errorf("project '%s' not found", projectName)
			}
		}
	} else {
		// Interactive selection
		selectedProject, err = selectProjectForSelect(allProjects, cfg)
		if err != nil {
			return err
		}
	}

	// Verify path exists
	if _, err := os.Stat(selectedProject.RootPath); os.IsNotExist(err) {
		return fmt.Errorf("project path does not exist: %s", selectedProject.RootPath)
	}

	// Output the path to stdout
	fmt.Println(selectedProject.RootPath)
	return nil
}

// selectProjectForSelect shows an interactive selection menu for the select command
// It writes prompts to stderr so only the path goes to stdout
func selectProjectForSelect(projects []*models.Project, cfg *config.Config) (*models.Project, error) {
	// Sort according to config
	sortProjects(projects, cfg.SortList)

	formatter := output.NewFormatter(!noColor && cfg.ShowColors)
	fmt.Fprintln(os.Stderr, "Select a project:")
	fmt.Fprintln(os.Stderr)

	for i, p := range projects {
		fmt.Fprintln(os.Stderr, formatter.FormatProjectCompact(p, i))
	}
	fmt.Fprintln(os.Stderr)

	fmt.Fprint(os.Stderr, "Enter project number (or 'q' to quit): ")
	var input string
	fmt.Scanln(&input)

	if input == "q" || input == "Q" {
		os.Exit(0)
	}

	var index int
	if _, err := fmt.Sscanf(input, "%d", &index); err != nil {
		return nil, fmt.Errorf("invalid selection")
	}

	if index < 0 || index >= len(projects) {
		return nil, fmt.Errorf("invalid selection: index out of range")
	}

	return projects[index], nil
}
