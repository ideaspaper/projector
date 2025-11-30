package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/anpan/projector/pkg/config"
	"github.com/anpan/projector/pkg/models"
	"github.com/anpan/projector/pkg/output"
	"github.com/anpan/projector/pkg/storage"
)

// Editor constants for supported editors
const (
	EditorCode     = "code"
	EditorVSCode   = "vscode"
	EditorCursor   = "cursor"
	EditorSublime  = "subl"
	EditorSublAlt  = "sublime"
	EditorAtom     = "atom"
	EditorVim      = "vim"
	EditorNeoVim   = "nvim"
	EditorEmacs    = "emacs"
	EditorIdea     = "idea"
	EditorIntelliJ = "intellij"
	EditorWebStorm = "webstorm"
	EditorGoLand   = "goland"
	EditorPyCharm  = "pycharm"
	EditorOpen     = "open"     // macOS
	EditorXdgOpen  = "xdg-open" // Linux
	EditorExplorer = "explorer" // Windows
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
	case EditorCode, EditorVSCode:
		args := []string{path}
		if newWindow {
			args = append([]string{"--new-window"}, args...)
		}
		cmd = exec.Command(EditorCode, args...)

	case EditorCursor:
		args := []string{path}
		if newWindow {
			args = append([]string{"--new-window"}, args...)
		}
		cmd = exec.Command(EditorCursor, args...)

	case EditorSublime, EditorSublAlt:
		args := []string{path}
		if newWindow {
			args = append([]string{"--new-window"}, args...)
		}
		cmd = exec.Command(EditorSublime, args...)

	case EditorAtom:
		args := []string{path}
		if newWindow {
			args = append([]string{"--new-window"}, args...)
		}
		cmd = exec.Command(EditorAtom, args...)

	case EditorVim, EditorNeoVim:
		cmd = exec.Command(editor, path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

	case EditorEmacs:
		cmd = exec.Command(EditorEmacs, path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

	case EditorIdea, EditorIntelliJ:
		cmd = exec.Command(EditorIdea, path)

	case EditorWebStorm:
		cmd = exec.Command(EditorWebStorm, path)

	case EditorGoLand:
		cmd = exec.Command(EditorGoLand, path)

	case EditorPyCharm:
		cmd = exec.Command(EditorPyCharm, path)

	case EditorOpen:
		// macOS open command
		cmd = exec.Command(EditorOpen, path)

	case EditorXdgOpen:
		// Linux open command
		cmd = exec.Command(EditorXdgOpen, path)

	case EditorExplorer:
		// Windows Explorer
		cmd = exec.Command(EditorExplorer, path)

	default:
		// Try to run the editor directly
		cmd = exec.Command(editor, path)
	}

	// For GUI editors, don't wait
	if editor != EditorVim && editor != EditorNeoVim && editor != EditorEmacs {
		return cmd.Start()
	}

	return cmd.Run()
}
