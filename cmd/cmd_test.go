package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ideaspaper/projector/pkg/models"
	"github.com/ideaspaper/projector/pkg/storage"
)

// testSetup creates a temporary directory with test data and returns cleanup function
func testSetup(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "projector-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create test projects.json (favorites)
	favorites := []*models.Project{
		{Name: "favorite1", RootPath: "/path/to/favorite1", Enabled: true, Tags: []string{"Work"}},
		{Name: "favorite2", RootPath: "/path/to/favorite2", Enabled: true, Tags: []string{"Personal"}},
	}
	favoritesData, _ := json.MarshalIndent(favorites, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "projects.json"), favoritesData, 0644)

	// Create test cache.json
	cache := storage.CachedProjects{
		Git: []*models.Project{
			{Name: "git-repo1", RootPath: "/path/to/git1", Enabled: true},
			{Name: "git-repo2", RootPath: "/path/to/git2", Enabled: true},
		},
		SVN: []*models.Project{
			{Name: "svn-repo1", RootPath: "/path/to/svn1", Enabled: true},
		},
		Mercurial: []*models.Project{
			{Name: "hg-repo1", RootPath: "/path/to/hg1", Enabled: true},
		},
		VSCode: []*models.Project{
			{Name: "vscode-ws1", RootPath: "/path/to/vscode1", Enabled: true},
		},
		Any: []*models.Project{
			{Name: "any-folder1", RootPath: "/path/to/any1", Enabled: true},
		},
	}
	cacheData, _ := json.MarshalIndent(cache, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "cache.json"), cacheData, 0644)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestLoadProjectsWithTypeFilters(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	store, err := storage.NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	tests := []struct {
		name          string
		showFavorites bool
		showGit       bool
		showSVN       bool
		showMercurial bool
		showVSCode    bool
		showAny       bool
		wantCount     int
		wantNames     []string
	}{
		{
			name:      "show all (no filter)",
			wantCount: 8,
		},
		{
			name:          "show only favorites",
			showFavorites: true,
			wantCount:     2,
			wantNames:     []string{"favorite1", "favorite2"},
		},
		{
			name:      "show only git",
			showGit:   true,
			wantCount: 2,
			wantNames: []string{"git-repo1", "git-repo2"},
		},
		{
			name:      "show only svn",
			showSVN:   true,
			wantCount: 1,
			wantNames: []string{"svn-repo1"},
		},
		{
			name:          "show only mercurial",
			showMercurial: true,
			wantCount:     1,
			wantNames:     []string{"hg-repo1"},
		},
		{
			name:       "show only vscode",
			showVSCode: true,
			wantCount:  1,
			wantNames:  []string{"vscode-ws1"},
		},
		{
			name:      "show only any",
			showAny:   true,
			wantCount: 1,
			wantNames: []string{"any-folder1"},
		},
		{
			name:          "show favorites and git",
			showFavorites: true,
			showGit:       true,
			wantCount:     4,
			wantNames:     []string{"favorite1", "favorite2", "git-repo1", "git-repo2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var allProjects []*models.Project

			// Determine which types to show (same logic as in open.go and select.go)
			showAll := !tt.showFavorites && !tt.showGit && !tt.showSVN && !tt.showMercurial && !tt.showVSCode && !tt.showAny

			// Load favorites
			if showAll || tt.showFavorites {
				projects, err := store.LoadProjects()
				if err != nil {
					t.Fatalf("failed to load projects: %v", err)
				}
				allProjects = append(allProjects, projects.Projects...)
			}

			// Load cached auto-detected projects
			if showAll || tt.showGit || tt.showSVN || tt.showMercurial || tt.showVSCode || tt.showAny {
				cache, err := store.LoadCache()
				if err != nil {
					t.Fatalf("failed to load cache: %v", err)
				}
				if showAll || tt.showGit {
					allProjects = append(allProjects, cache.Git...)
				}
				if showAll || tt.showSVN {
					allProjects = append(allProjects, cache.SVN...)
				}
				if showAll || tt.showMercurial {
					allProjects = append(allProjects, cache.Mercurial...)
				}
				if showAll || tt.showVSCode {
					allProjects = append(allProjects, cache.VSCode...)
				}
				if showAll || tt.showAny {
					allProjects = append(allProjects, cache.Any...)
				}
			}

			if len(allProjects) != tt.wantCount {
				t.Errorf("got %d projects, want %d", len(allProjects), tt.wantCount)
			}

			if tt.wantNames != nil {
				gotNames := make(map[string]bool)
				for _, p := range allProjects {
					gotNames[p.Name] = true
				}
				for _, name := range tt.wantNames {
					if !gotNames[name] {
						t.Errorf("expected project %q not found", name)
					}
				}
			}
		})
	}
}

func TestLoadProjectsWithTagFilter(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	store, err := storage.NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Load all projects
	allProjects, err := store.LoadAllProjects()
	if err != nil {
		t.Fatalf("failed to load all projects: %v", err)
	}

	tests := []struct {
		name      string
		tag       string
		wantCount int
		wantNames []string
	}{
		{
			name:      "filter by Work tag",
			tag:       "Work",
			wantCount: 1,
			wantNames: []string{"favorite1"},
		},
		{
			name:      "filter by Personal tag",
			tag:       "Personal",
			wantCount: 1,
			wantNames: []string{"favorite2"},
		},
		{
			name:      "filter by non-existent tag",
			tag:       "NonExistent",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := make([]*models.Project, 0)
			for _, p := range allProjects {
				if p.HasTag(tt.tag) {
					filtered = append(filtered, p)
				}
			}

			if len(filtered) != tt.wantCount {
				t.Errorf("got %d projects, want %d", len(filtered), tt.wantCount)
			}

			if tt.wantNames != nil {
				gotNames := make(map[string]bool)
				for _, p := range filtered {
					gotNames[p.Name] = true
				}
				for _, name := range tt.wantNames {
					if !gotNames[name] {
						t.Errorf("expected project %q not found", name)
					}
				}
			}
		})
	}
}

func TestLoadProjectsCombinedFilters(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	store, err := storage.NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Test: show only favorites with Work tag
	t.Run("favorites with Work tag", func(t *testing.T) {
		// Load only favorites
		projects, err := store.LoadProjects()
		if err != nil {
			t.Fatalf("failed to load projects: %v", err)
		}

		// Filter by tag
		filtered := make([]*models.Project, 0)
		for _, p := range projects.Projects {
			if p.HasTag("Work") {
				filtered = append(filtered, p)
			}
		}

		if len(filtered) != 1 {
			t.Errorf("got %d projects, want 1", len(filtered))
		}
		if len(filtered) > 0 && filtered[0].Name != "favorite1" {
			t.Errorf("got project %q, want favorite1", filtered[0].Name)
		}
	})
}
