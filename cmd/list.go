package cmd

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ideaspaper/projector/pkg/config"
	"github.com/ideaspaper/projector/pkg/models"
	"github.com/ideaspaper/projector/pkg/output"
	"github.com/ideaspaper/projector/pkg/scanner"
	"github.com/ideaspaper/projector/pkg/storage"
)

var (
	// list command flags
	listTag       string
	listShowPath  bool
	listGrouped   bool
	listAll       bool
	listFavorites bool
	listGit       bool
	listSVN       bool
	listMercurial bool
	listVSCode    bool
	listAny       bool
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved and detected projects",
	Long: `List all projects, including saved favorites and auto-detected repositories.

Examples:
  # List all projects
  projector list

  # List only favorites
  projector list --favorites

  # List only git repositories
  projector list --git

  # Filter by tag
  projector list --tag Work

  # Show project paths
  projector list --path

  # Group by project type
  projector list --grouped`,
	Aliases: []string{"ls"},
	RunE:    runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listTag, "tag", "t", "", "filter projects by tag")
	listCmd.Flags().BoolVarP(&listShowPath, "path", "p", false, "show project paths")
	listCmd.Flags().BoolVarP(&listGrouped, "grouped", "g", false, "group projects by type")
	listCmd.Flags().BoolVarP(&listAll, "all", "a", false, "include disabled projects")
	listCmd.Flags().BoolVar(&listFavorites, "favorites", false, "show only favorites")
	listCmd.Flags().BoolVar(&listGit, "git", false, "show only git repositories")
	listCmd.Flags().BoolVar(&listSVN, "svn", false, "show only svn repositories")
	listCmd.Flags().BoolVar(&listMercurial, "mercurial", false, "show only mercurial repositories")
	listCmd.Flags().BoolVar(&listVSCode, "vscode", false, "show only vscode workspaces")
	listCmd.Flags().BoolVar(&listAny, "any", false, "show only any-folder projects")
}

func runList(cmd *cobra.Command, args []string) error {
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

	var allProjects []*models.Project

	// Determine which types to show
	showAll := !listFavorites && !listGit && !listSVN && !listMercurial && !listVSCode && !listAny

	// Load favorites
	if showAll || listFavorites {
		projects, err := store.LoadProjects()
		if err != nil {
			return fmt.Errorf("failed to load projects: %w", err)
		}
		allProjects = append(allProjects, projects.Projects...)
	}

	// Load cached auto-detected projects
	if showAll || listGit || listSVN || listMercurial || listVSCode || listAny {
		cache, err := store.LoadCache()
		if err == nil {
			if showAll || listGit {
				allProjects = append(allProjects, cache.Git...)
			}
			if showAll || listSVN {
				allProjects = append(allProjects, cache.SVN...)
			}
			if showAll || listMercurial {
				allProjects = append(allProjects, cache.Mercurial...)
			}
			if showAll || listVSCode {
				allProjects = append(allProjects, cache.VSCode...)
			}
			if showAll || listAny {
				allProjects = append(allProjects, cache.Any...)
			}
		}
	}

	// Filter by enabled
	if !listAll {
		filtered := make([]*models.Project, 0)
		for _, p := range allProjects {
			if p.Enabled {
				filtered = append(filtered, p)
			}
		}
		allProjects = filtered
	}

	// Filter by tag
	if listTag != "" {
		filtered := make([]*models.Project, 0)
		for _, p := range allProjects {
			if p.HasTag(listTag) {
				filtered = append(filtered, p)
			}
		}
		allProjects = filtered
	}

	// Check for invalid paths if configured
	if cfg.CheckInvalidPaths {
		for _, p := range allProjects {
			if _, err := os.Stat(p.RootPath); os.IsNotExist(err) {
				p.Enabled = false
			}
		}
	}

	// Sort projects
	sortProjects(allProjects, cfg.SortList)

	// Override grouping from flag or config
	grouped := listGrouped || cfg.GroupList

	// Format and display
	formatter := output.NewFormatter(!noColor && cfg.ShowColors)
	fmt.Println(formatter.FormatProjectList(allProjects, listShowPath, grouped))

	return nil
}

// sortProjects sorts projects according to the specified order
func sortProjects(projects []*models.Project, order config.SortOrder) {
	switch order {
	case config.SortByName:
		sort.Slice(projects, func(i, j int) bool {
			return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
		})
	case config.SortByPath:
		sort.Slice(projects, func(i, j int) bool {
			return strings.ToLower(projects[i].RootPath) < strings.ToLower(projects[j].RootPath)
		})
	case config.SortBySaved, config.SortByRecent:
		// Keep original order for saved/recent
	}
}

// selectCmd allows interactive project selection
var selectCmd = &cobra.Command{
	Use:   "select",
	Short: "Interactively select a project",
	Long: `Interactively select a project from the list.

This command displays a numbered list of projects and allows you to
select one by entering its number.

Examples:
  # Select a project interactively
  projector select`,
	RunE: runSelect,
}

func init() {
	rootCmd.AddCommand(selectCmd)
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

	// Load all projects
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

	if len(allProjects) == 0 {
		fmt.Println("No projects found.")
		return nil
	}

	// Sort by name
	sortProjects(allProjects, config.SortByName)

	// Open /dev/tty for interactive output (works even when stdout is redirected)
	var tty *os.File
	if runtime.GOOS == "windows" {
		tty, err = os.OpenFile("CON", os.O_WRONLY, 0)
	} else {
		tty, err = os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	}
	if err != nil {
		// Fallback to stdout if /dev/tty is not available
		tty = os.Stdout
	} else {
		defer tty.Close()
		// Force color output since we're writing to a terminal
		if cfg.ShowColors && !noColor {
			color.NoColor = false
		}
	}

	// Display list to tty
	formatter := output.NewFormatter(!noColor && cfg.ShowColors)
	fmt.Fprintln(tty, "Select a project:")
	fmt.Fprintln(tty)
	for i, p := range allProjects {
		fmt.Fprintln(tty, formatter.FormatProjectCompact(p, i))
	}
	fmt.Fprintln(tty)

	// Read selection (prompt to tty)
	fmt.Fprint(tty, "Enter project number: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	input = strings.TrimSpace(input)
	index, err := strconv.Atoi(input)
	if err != nil || index < 0 || index >= len(allProjects) {
		return fmt.Errorf("invalid selection: %s", input)
	}

	selected := allProjects[index]

	// Output the selected project path to stdout
	fmt.Println(selected.RootPath)

	return nil
}

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan [paths...]",
	Short: "Scan directories for projects",
	Long: `Scan directories for Git, SVN, Mercurial repositories, VS Code workspaces,
or any folder.

Examples:
  # Scan for git repositories in ~/projects
  projector scan --git ~/projects

  # Scan for all types in configured base folders
  projector scan --all

  # Scan for git repos with custom depth
  projector scan --git --depth 5 ~/code`,
	RunE: runScan,
}

var (
	scanGit       bool
	scanSVN       bool
	scanMercurial bool
	scanVSCode    bool
	scanAny       bool
	scanAll       bool
	scanDepth     int
)

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().BoolVar(&scanGit, "git", false, "scan for git repositories")
	scanCmd.Flags().BoolVar(&scanSVN, "svn", false, "scan for svn repositories")
	scanCmd.Flags().BoolVar(&scanMercurial, "mercurial", false, "scan for mercurial repositories")
	scanCmd.Flags().BoolVar(&scanVSCode, "vscode", false, "scan for vscode workspaces")
	scanCmd.Flags().BoolVar(&scanAny, "any", false, "scan for any folder")
	scanCmd.Flags().BoolVarP(&scanAll, "all", "a", false, "scan for all types")
	scanCmd.Flags().IntVarP(&scanDepth, "depth", "d", 0, "maximum scan depth (0 = use config default)")
}

func runScan(cmd *cobra.Command, args []string) error {
	// Validate depth flag
	if scanDepth < 0 {
		return fmt.Errorf("--depth must be a non-negative integer, got %d", scanDepth)
	}

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

	// Determine what to scan
	if !scanGit && !scanSVN && !scanMercurial && !scanVSCode && !scanAny && !scanAll {
		scanAll = true
	}

	formatter := output.NewFormatter(!noColor && cfg.ShowColors)
	cache := &storage.CachedProjects{}

	// Scan Git
	if scanAll || scanGit {
		baseFolders := cfg.GitBaseFolders
		if len(args) > 0 {
			baseFolders = args
		}
		if len(baseFolders) > 0 {
			s := scanner.NewScanner(scanner.ScannerGit)
			s.SetBaseFolders(baseFolders)
			s.SetIgnoredFolders(cfg.GitIgnoredFolders)
			depth := cfg.GitMaxDepth
			if scanDepth > 0 {
				depth = scanDepth
			}
			s.SetMaxDepth(depth)
			s.SetIgnoreWithinProjects(cfg.IgnoreProjectsWithinProjects)
			s.SetSupportSymlinks(cfg.SupportSymlinks)

			projects, err := s.Scan()
			if err != nil {
				fmt.Println(formatter.FormatWarning(fmt.Sprintf("Error scanning Git repositories: %v", err)))
			} else {
				cache.Git = projects
				fmt.Println(formatter.FormatInfo(fmt.Sprintf("Found %d Git repositories", len(projects))))
			}
		}
	}

	// Scan SVN
	if scanAll || scanSVN {
		baseFolders := cfg.SVNBaseFolders
		if len(args) > 0 {
			baseFolders = args
		}
		if len(baseFolders) > 0 {
			s := scanner.NewScanner(scanner.ScannerSVN)
			s.SetBaseFolders(baseFolders)
			s.SetIgnoredFolders(cfg.SVNIgnoredFolders)
			depth := cfg.SVNMaxDepth
			if scanDepth > 0 {
				depth = scanDepth
			}
			s.SetMaxDepth(depth)

			projects, err := s.Scan()
			if err != nil {
				fmt.Println(formatter.FormatWarning(fmt.Sprintf("Error scanning SVN repositories: %v", err)))
			} else {
				cache.SVN = projects
				fmt.Println(formatter.FormatInfo(fmt.Sprintf("Found %d SVN repositories", len(projects))))
			}
		}
	}

	// Scan Mercurial
	if scanAll || scanMercurial {
		baseFolders := cfg.MercurialBaseFolders
		if len(args) > 0 {
			baseFolders = args
		}
		if len(baseFolders) > 0 {
			s := scanner.NewScanner(scanner.ScannerMercurial)
			s.SetBaseFolders(baseFolders)
			s.SetIgnoredFolders(cfg.MercurialIgnoredFolders)
			depth := cfg.MercurialMaxDepth
			if scanDepth > 0 {
				depth = scanDepth
			}
			s.SetMaxDepth(depth)

			projects, err := s.Scan()
			if err != nil {
				fmt.Println(formatter.FormatWarning(fmt.Sprintf("Error scanning Mercurial repositories: %v", err)))
			} else {
				cache.Mercurial = projects
				fmt.Println(formatter.FormatInfo(fmt.Sprintf("Found %d Mercurial repositories", len(projects))))
			}
		}
	}

	// Scan VSCode
	if scanAll || scanVSCode {
		baseFolders := cfg.VSCodeBaseFolders
		if len(args) > 0 {
			baseFolders = args
		}
		if len(baseFolders) > 0 {
			s := scanner.NewScanner(scanner.ScannerVSCode)
			s.SetBaseFolders(baseFolders)
			s.SetIgnoredFolders(cfg.VSCodeIgnoredFolders)
			depth := cfg.VSCodeMaxDepth
			if scanDepth > 0 {
				depth = scanDepth
			}
			s.SetMaxDepth(depth)

			projects, err := s.Scan()
			if err != nil {
				fmt.Println(formatter.FormatWarning(fmt.Sprintf("Error scanning VS Code workspaces: %v", err)))
			} else {
				cache.VSCode = projects
				fmt.Println(formatter.FormatInfo(fmt.Sprintf("Found %d VS Code workspaces", len(projects))))
			}
		}
	}

	// Scan Any
	if scanAll || scanAny {
		baseFolders := cfg.AnyBaseFolders
		if len(args) > 0 {
			baseFolders = args
		}
		if len(baseFolders) > 0 {
			s := scanner.NewScanner(scanner.ScannerAny)
			s.SetBaseFolders(baseFolders)
			s.SetIgnoredFolders(cfg.AnyIgnoredFolders)
			depth := cfg.AnyMaxDepth
			if scanDepth > 0 {
				depth = scanDepth
			}
			s.SetMaxDepth(depth)

			projects, err := s.Scan()
			if err != nil {
				fmt.Println(formatter.FormatWarning(fmt.Sprintf("Error scanning folders: %v", err)))
			} else {
				cache.Any = projects
				fmt.Println(formatter.FormatInfo(fmt.Sprintf("Found %d folders", len(projects))))
			}
		}
	}

	// Save cache
	if cfg.CacheProjectsBetweenSessions {
		if err := store.SaveCache(cache); err != nil {
			return fmt.Errorf("failed to save cache: %w", err)
		}
		fmt.Println(formatter.FormatSuccess("Cache updated"))
	}

	return nil
}
