package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/anpan/projector/pkg/config"
	"github.com/anpan/projector/pkg/models"
	"github.com/anpan/projector/pkg/output"
	"github.com/anpan/projector/pkg/storage"
)

var (
	openNewWindow bool
	openEditor    string
)

// openCmd represents the open command
var openCmd = &cobra.Command{
	Use:   "open [project-name]",
	Short: "Open a project in your editor",
	Long: `Open a project in your configured editor (default: VS Code).

If no project name is provided, an interactive selection is shown.

Examples:
  # Open a project by name
  projector open myproject

  # Open in a new window
  projector open myproject --new-window

  # Open with a specific editor
  projector open myproject --editor vim`,
	Args: cobra.MaximumNArgs(1),
	RunE: runOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)

	openCmd.Flags().BoolVarP(&openNewWindow, "new-window", "n", false, "open in a new window")
	openCmd.Flags().StringVarP(&openEditor, "editor", "e", "", "editor to use (overrides config)")
}

func runOpen(cmd *cobra.Command, args []string) error {
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
	var allProjects []*models.Project

	projects, err := store.LoadProjects()
	if err != nil {
		return fmt.Errorf("failed to load projects: %w", err)
	}
	allProjects = append(allProjects, projects.Projects...)

	// Load cache
	cache, _ := store.LoadCache()
	if cache != nil {
		allProjects = append(allProjects, cache.Git...)
		allProjects = append(allProjects, cache.SVN...)
		allProjects = append(allProjects, cache.Mercurial...)
		allProjects = append(allProjects, cache.VSCode...)
		allProjects = append(allProjects, cache.Any...)
	}

	// Filter enabled only
	filtered := make([]*models.Project, 0)
	for _, p := range allProjects {
		if p.Enabled {
			filtered = append(filtered, p)
		}
	}
	allProjects = filtered

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
				fmt.Println(formatter.FormatWarning(fmt.Sprintf("Multiple projects match '%s':", projectName)))
				for _, p := range matches {
					fmt.Printf("  - %s (%s)\n", p.Name, p.RootPath)
				}
				return fmt.Errorf("please be more specific")
			} else {
				return fmt.Errorf("project '%s' not found", projectName)
			}
		}
	} else {
		// Interactive selection
		selectedProject, err = selectProjectInteractive(allProjects, cfg)
		if err != nil {
			return err
		}
	}

	// Verify path exists
	if _, err := os.Stat(selectedProject.RootPath); os.IsNotExist(err) {
		return fmt.Errorf("project path does not exist: %s", selectedProject.RootPath)
	}

	// Determine editor
	editor := openEditor
	if editor == "" {
		editor = cfg.Editor
	}

	// Open project
	formatter := output.NewFormatter(!noColor && cfg.ShowColors)
	fmt.Println(formatter.FormatInfo(fmt.Sprintf("Opening '%s' in %s...", selectedProject.Name, editor)))

	return openInEditor(selectedProject.RootPath, editor, openNewWindow || cfg.OpenInNewWindow)
}

// selectProjectInteractive shows an interactive selection menu
func selectProjectInteractive(projects []*models.Project, cfg *config.Config) (*models.Project, error) {
	// Sort by name
	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})

	formatter := output.NewFormatter(!noColor && cfg.ShowColors)
	fmt.Println("Select a project to open:")
	fmt.Println()

	for i, p := range projects {
		fmt.Println(formatter.FormatProjectCompact(p, i))
	}
	fmt.Println()

	fmt.Print("Enter project number (or 'q' to quit): ")
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

// openInEditor opens a path in the specified editor
func openInEditor(path, editor string, newWindow bool) error {
	var cmd *exec.Cmd

	switch editor {
	case "code", "vscode":
		args := []string{path}
		if newWindow {
			args = append([]string{"--new-window"}, args...)
		}
		cmd = exec.Command("code", args...)

	case "cursor":
		args := []string{path}
		if newWindow {
			args = append([]string{"--new-window"}, args...)
		}
		cmd = exec.Command("cursor", args...)

	case "subl", "sublime":
		args := []string{path}
		if newWindow {
			args = append([]string{"--new-window"}, args...)
		}
		cmd = exec.Command("subl", args...)

	case "atom":
		args := []string{path}
		if newWindow {
			args = append([]string{"--new-window"}, args...)
		}
		cmd = exec.Command("atom", args...)

	case "vim", "nvim":
		cmd = exec.Command(editor, path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

	case "emacs":
		cmd = exec.Command("emacs", path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

	case "idea", "intellij":
		cmd = exec.Command("idea", path)

	case "webstorm":
		cmd = exec.Command("webstorm", path)

	case "goland":
		cmd = exec.Command("goland", path)

	case "pycharm":
		cmd = exec.Command("pycharm", path)

	case "open":
		// macOS open command
		cmd = exec.Command("open", path)

	case "xdg-open":
		// Linux open command
		cmd = exec.Command("xdg-open", path)

	case "explorer":
		// Windows Explorer
		cmd = exec.Command("explorer", path)

	default:
		// Try to run the editor directly
		cmd = exec.Command(editor, path)
	}

	// For GUI editors, don't wait
	if editor != "vim" && editor != "nvim" && editor != "emacs" {
		if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
			return cmd.Start()
		}
	}

	return cmd.Run()
}
