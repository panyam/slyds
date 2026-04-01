package cmd

import (
	"encoding/json"
	"path/filepath"
	"sort"

	"github.com/panyam/slyds/internal/layout"
	"github.com/panyam/slyds/internal/scaffold"
	"github.com/spf13/cobra"
)

// IntrospectSchemaVersion is bumped when the JSON shape changes incompatibly.
const IntrospectSchemaVersion = "1"

// IntrospectDocument is the machine-readable output of `slyds introspect`.
type IntrospectDocument struct {
	Version         string `json:"version"`
	SchemaVersion   string `json:"schema_version"`
	RootResolution  string `json:"root_resolution"`
	Deck            *DeckContext `json:"deck,omitempty"`
	Layouts         []IntrospectLayout `json:"layouts"`
	ThemesBuiltin   []string           `json:"themes_builtin"`
	Commands        []AgentCommandInfo `json:"commands"`
}

// DeckContext is present when introspect runs inside or below a deck directory.
type DeckContext struct {
	Root         string `json:"root"`
	ResolvedFrom string `json:"resolved_from"`
	Manifest     *IntrospectManifest `json:"manifest,omitempty"`
}

// IntrospectManifest summarizes .slyds.yaml when present.
type IntrospectManifest struct {
	Theme       string `json:"theme"`
	Title       string `json:"title"`
	HasSources  bool   `json:"has_sources"`
	ModulesDir  string `json:"modules_dir,omitempty"`
}

// IntrospectLayout is one structural layout with slot names for agent targeting.
type IntrospectLayout struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Slots       []string `json:"slots"`
	SlotSelector string  `json:"slot_selector_pattern"`
}

// AgentCommandInfo describes a CLI entry point agents should use.
type AgentCommandInfo struct {
	Name        string   `json:"name"`
	Synopsis    string   `json:"synopsis"`
	Example     string   `json:"example"`
}

var introspectCmd = &cobra.Command{
	Use:   "introspect [dir]",
	Short: "Emit machine-readable capabilities (layouts, slots, themes, commands)",
	Long: `Introspect prints JSON (default) describing slyds version, built-in layouts
with data-slot names, built-in themes, and recommended CLI commands for agents.

If dir is inside a presentation (contains index.html when walking up), deck
context and manifest fields are included.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		doc, err := buildIntrospectDocument(dir)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	},
}

// buildIntrospectDocument assembles the introspection payload for dir (may be non-deck).
func buildIntrospectDocument(dir string) (*IntrospectDocument, error) {
	doc := &IntrospectDocument{
		Version:        Version,
		SchemaVersion:  IntrospectSchemaVersion,
		RootResolution: `A presentation root is a directory containing index.html. Commands that accept [dir] find the nearest ancestor with index.html starting from the given path.`,
		Layouts:        nil,
		ThemesBuiltin:  availableThemeNames(),
		Commands:       agentCommandCatalog(),
	}

	reg, err := layout.LoadRegistry()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(reg.Layouts))
	for n := range reg.Layouts {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		e := reg.Layouts[name]
		doc.Layouts = append(doc.Layouts, IntrospectLayout{
			Name:           name,
			Description:    e.Description,
			Slots:          append([]string(nil), e.Slots...),
			SlotSelector:   `[data-slot="NAME"] — replace NAME with a slot from "slots"`,
		})
	}

	root, err := findRootIn(dir)
	if err != nil {
		return doc, nil
	}
	absDir, _ := filepath.Abs(dir)
	manifest, manErr := scaffold.ReadManifest(root)
	var im *IntrospectManifest
	if manErr == nil && manifest != nil {
		im = &IntrospectManifest{
			Theme:      manifest.Theme,
			Title:      manifest.Title,
			HasSources: manifest.HasSources(),
			ModulesDir: manifest.ModulesDir,
		}
	} else if manErr != nil && manErr != scaffold.ErrManifestNotFound {
		return nil, manErr
	}
	doc.Deck = &DeckContext{
		Root:         root,
		ResolvedFrom: absDir,
		Manifest:     im,
	}
	return doc, nil
}

func agentCommandCatalog() []AgentCommandInfo {
	return []AgentCommandInfo{
		{Name: "init", Synopsis: "scaffold a new deck", Example: `slyds init "Title" -n 5 --theme default`},
		{Name: "update", Synopsis: "refresh engine/theme files", Example: `slyds update`},
		{Name: "serve", Synopsis: "dev server", Example: `slyds serve -p 3000`},
		{Name: "build", Synopsis: "flatten to dist/index.html", Example: `slyds build`},
		{Name: "add", Synopsis: "append slide", Example: `slyds add "Topic" --layout content`},
		{Name: "insert", Synopsis: "insert slide at position", Example: `slyds insert 2 "Topic" --layout two-col --title "My title"`},
		{Name: "rm", Synopsis: "remove slide", Example: `slyds rm 3`},
		{Name: "mv", Synopsis: "reorder slide", Example: `slyds mv 2 5`},
		{Name: "ls", Synopsis: "list slides", Example: `slyds ls`},
		{Name: "describe", Synopsis: "structured deck summary", Example: `slyds describe --json`},
		{Name: "introspect", Synopsis: "this command — capabilities JSON", Example: `slyds introspect`},
		{Name: "check", Synopsis: "validate deck", Example: `slyds check`},
		{Name: "query", Synopsis: "CSS selector read/write on slide HTML", Example: `slyds query 1 '[data-slot="body"]' --set-html "<p>...</p>"`},
		{Name: "query --batch", Synopsis: "apply multiple writes atomically", Example: `slyds query --batch ops.json`},
		{Name: "slugify", Synopsis: "rename slides from h1", Example: `slyds slugify`},
	}
}

func init() {
	rootCmd.AddCommand(introspectCmd)
}
