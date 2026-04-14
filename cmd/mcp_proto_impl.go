package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	mcpcore "github.com/panyam/mcpkit/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/panyam/slyds/gen/go/slyds/v1"
	"github.com/panyam/slyds/core"
)

// SlydsServiceImpl implements the proto-generated SlydsServiceMCPServer and
// SlydsServiceMCPResourceServer interfaces by delegating to a Workspace.
// This is the proto equivalent of the hand-written handlers in mcp_tools.go
// and mcp_resources.go.
type SlydsServiceImpl struct {
	// ws is resolved from request context via workspaceFromContext, not stored.
	// Keeping it here would break multi-tenant deployments where the workspace
	// is per-request. But for compatibility with the generated interface
	// (which passes plain context.Context), we resolve from context in each method.
}

// deck opens a deck from the workspace on the request context.
func (s *SlydsServiceImpl) deck(ctx context.Context, name string) (*core.Deck, error) {
	ws := workspaceFromContext(ctx)
	if ws == nil {
		return nil, status.Error(codes.Internal, "no workspace on context")
	}
	d, err := ws.OpenDeck(name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "deck %q: %v", name, err)
	}
	return d, nil
}

// workspace returns the workspace from request context.
func (s *SlydsServiceImpl) workspace(ctx context.Context) (Workspace, error) {
	ws := workspaceFromContext(ctx)
	if ws == nil {
		return nil, status.Error(codes.Internal, "no workspace on context")
	}
	return ws, nil
}

// --- Tool implementations ---

func (s *SlydsServiceImpl) ListDecks(ctx context.Context, req *pb.ListDecksRequest) (*pb.ListDecksResponse, error) {
	ws, err := s.workspace(ctx)
	if err != nil {
		return nil, err
	}
	refs, err := ws.ListDecks()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list decks: %v", err)
	}
	var decks []*pb.DeckSummary
	for _, ref := range refs {
		d, err := ws.OpenDeck(ref.Name)
		if err != nil {
			continue
		}
		count, _ := d.SlideCount()
		decks = append(decks, &pb.DeckSummary{
			Name:   ref.Name,
			Title:  d.Title(),
			Theme:  d.Theme(),
			Slides: int32(count),
		})
	}
	return &pb.ListDecksResponse{Decks: decks}, nil
}

func (s *SlydsServiceImpl) CreateDeck(ctx context.Context, req *pb.CreateDeckRequest) (*pb.DeckDescription, error) {
	ws, err := s.workspace(ctx)
	if err != nil {
		return nil, err
	}
	theme := "default"
	if req.Theme != nil {
		theme = *req.Theme
	}
	slides := int32(3)
	if req.Slides != nil {
		slides = *req.Slides
	}
	d, err := ws.CreateDeck(req.Name, req.Title, theme, int(slides))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return s.describeDeck(d)
}

func (s *SlydsServiceImpl) DescribeDeck(ctx context.Context, req *pb.DeckRequest) (*pb.DeckDescription, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	return s.describeDeck(d)
}

func (s *SlydsServiceImpl) ListSlides(ctx context.Context, req *pb.DeckRequest) (*pb.ListSlidesResponse, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	desc, err := s.describeDeck(d)
	if err != nil {
		return nil, err
	}
	return &pb.ListSlidesResponse{Slides: desc.Slides}, nil
}

func (s *SlydsServiceImpl) ReadSlide(ctx context.Context, req *pb.ReadSlideRequest) (*pb.SlideReadResult, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	pos, err := s.resolvePosition(d, req.Slide, req.Position)
	if err != nil {
		return nil, err
	}
	content, err := d.GetSlideContent(pos)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read slide %d: %v", pos, err)
	}
	ver, _ := d.SlideVersion(pos)
	deckVer, _ := d.DeckVersion()
	return &pb.SlideReadResult{
		Content:     content,
		Version:     ver,
		DeckVersion: deckVer,
	}, nil
}

func (s *SlydsServiceImpl) EditSlide(ctx context.Context, req *pb.EditSlideRequest) (*pb.SlideEditResult, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	pos, err := s.resolvePosition(d, req.Slide, req.Position)
	if err != nil {
		return nil, err
	}
	// Optimistic version check
	if req.ExpectedVersion != nil && *req.ExpectedVersion != "" && *req.ExpectedVersion != "latest" {
		currentVer, err := d.SlideVersion(pos)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "version check: %v", err)
		}
		if currentVer != *req.ExpectedVersion {
			currentContent, _ := d.GetSlideContent(pos)
			deckVer, _ := d.DeckVersion()
			detail := &pb.VersionConflictDetail{
				CurrentVersion: currentVer,
				CurrentContent: &currentContent,
				DeckVersion:    &deckVer,
			}
			st, _ := status.New(codes.Aborted, "version conflict").WithDetails(detail)
			return nil, st.Err()
		}
	}
	if err := d.EditSlideContent(pos, req.Content); err != nil {
		return nil, status.Errorf(codes.Internal, "edit slide: %v", err)
	}
	newVer, _ := d.SlideVersion(pos)
	deckVer, _ := d.DeckVersion()
	return &pb.SlideEditResult{
		Version:     newVer,
		DeckVersion: deckVer,
		Position:    int32(pos),
	}, nil
}

func (s *SlydsServiceImpl) QuerySlide(ctx context.Context, req *pb.QuerySlideRequest) (*pb.QuerySlideResponse, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	opts := core.QueryOpts{}
	if req.Html != nil {
		opts.HTML = *req.Html
	}
	if req.Attr != nil {
		opts.Attr = *req.Attr
	}
	if req.Count != nil {
		opts.Count = *req.Count
	}
	if req.Set != nil {
		opts.Set = req.Set
	}
	if req.SetHtml != nil {
		opts.SetHTML = req.SetHtml
	}
	if req.SetAttr != nil {
		opts.SetAttr = req.SetAttr
	}
	if req.Append != nil {
		opts.Append = req.Append
	}
	if req.Remove != nil {
		opts.Remove = *req.Remove
	}
	if req.All != nil {
		opts.All = *req.All
	}
	results, err := d.Query(req.Slide, req.Selector, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query: %v", err)
	}
	// results is []core.QueryResult which is []string for text queries
	data, _ := json.Marshal(results)
	var strs []string
	json.Unmarshal(data, &strs)
	return &pb.QuerySlideResponse{Results: strs}, nil
}

func (s *SlydsServiceImpl) AddSlide(ctx context.Context, req *pb.AddSlideRequest) (*pb.AddSlideResponse, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	// Optimistic deck version check
	if req.ExpectedDeckVersion != nil && *req.ExpectedDeckVersion != "" && *req.ExpectedDeckVersion != "latest" {
		currentDeckVer, err := d.DeckVersion()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "deck version: %v", err)
		}
		if currentDeckVer != *req.ExpectedDeckVersion {
			detail := &pb.VersionConflictDetail{
				CurrentVersion: currentDeckVer,
				DeckVersion:    &currentDeckVer,
			}
			st, _ := status.New(codes.Aborted, "deck version conflict").WithDetails(detail)
			return nil, st.Err()
		}
	}
	layout := "content"
	if req.Layout != nil {
		layout = *req.Layout
	}
	title := ""
	if req.Title != nil {
		title = *req.Title
	}
	finalSlug, slideID, err := d.InsertSlide(int(req.Position), req.Name, layout, title)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "insert slide: %v", err)
	}
	deckVer, _ := d.DeckVersion()
	return &pb.AddSlideResponse{
		Slug:        finalSlug,
		SlideId:     slideID,
		DeckVersion: deckVer,
		Position:    req.Position,
	}, nil
}

func (s *SlydsServiceImpl) RemoveSlide(ctx context.Context, req *pb.RemoveSlideRequest) (*pb.RemoveSlideResponse, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	// Optimistic deck version check
	if req.ExpectedDeckVersion != nil && *req.ExpectedDeckVersion != "" && *req.ExpectedDeckVersion != "latest" {
		currentDeckVer, err := d.DeckVersion()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "deck version: %v", err)
		}
		if currentDeckVer != *req.ExpectedDeckVersion {
			detail := &pb.VersionConflictDetail{
				CurrentVersion: currentDeckVer,
				DeckVersion:    &currentDeckVer,
			}
			st, _ := status.New(codes.Aborted, "deck version conflict").WithDetails(detail)
			return nil, st.Err()
		}
	}
	filename, err := d.ResolveSlide(req.Slide)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "resolve slide: %v", err)
	}
	if err := d.RemoveSlide(filename); err != nil {
		return nil, status.Errorf(codes.Internal, "remove slide: %v", err)
	}
	deckVer, _ := d.DeckVersion()
	count, _ := d.SlideCount()
	return &pb.RemoveSlideResponse{
		DeckVersion: deckVer,
		SlideCount:  int32(count),
		RemovedFile: filename,
	}, nil
}

func (s *SlydsServiceImpl) CheckDeck(ctx context.Context, req *pb.DeckRequest) (*pb.CheckDeckResponse, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	result, err := d.Check()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "check: %v", err)
	}
	var pbIssues []*pb.Issue
	for _, issue := range result.Issues {
		pbIssues = append(pbIssues, &pb.Issue{
			Type:   issue.Type.String(),
			Slide:  issue.Slide,
			Detail: issue.Detail,
		})
	}
	return &pb.CheckDeckResponse{Issues: pbIssues}, nil
}

func (s *SlydsServiceImpl) BuildDeck(ctx context.Context, req *pb.DeckRequest) (*pb.BuildDeckResponse, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	result, err := d.Build()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build: %v", err)
	}
	return &pb.BuildDeckResponse{
		Html:     result.HTML,
		Warnings: result.Warnings,
	}, nil
}

// --- Resource implementations ---

func (s *SlydsServiceImpl) GetServerInfo(ctx context.Context, req *pb.ServerInfoRequest) (*pb.ServerInfo, error) {
	ws := workspaceFromContext(ctx)
	themes := core.AvailableThemeNames()
	layouts, _ := core.ListLayouts()
	info := &pb.ServerInfo{
		Name:    "slyds",
		Version: Version,
		Themes:  themes,
		Layouts: layouts,
	}
	if lw, ok := ws.(*LocalWorkspace); ok {
		root := lw.Root()
		info.DeckRoot = &root
	}
	return info, nil
}

func (s *SlydsServiceImpl) GetDeckList(ctx context.Context, req *pb.DeckListRequest) (*pb.ListDecksResponse, error) {
	return s.ListDecks(ctx, &pb.ListDecksRequest{})
}

func (s *SlydsServiceImpl) GetDeck(ctx context.Context, req *pb.GetDeckResourceRequest) (*pb.DeckDescription, error) {
	d, err := s.deck(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	return s.describeDeck(d)
}

func (s *SlydsServiceImpl) GetSlideList(ctx context.Context, req *pb.GetSlideListResourceRequest) (*pb.ListSlidesResponse, error) {
	d, err := s.deck(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	desc, err := s.describeDeck(d)
	if err != nil {
		return nil, err
	}
	return &pb.ListSlidesResponse{Slides: desc.Slides}, nil
}

func (s *SlydsServiceImpl) GetSlideContent(ctx context.Context, req *pb.GetSlideContentResourceRequest) (*pb.SlideContentResource, error) {
	d, err := s.deck(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(req.N)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid slide number %q", req.N)
	}
	content, err := d.GetSlideContent(n)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "slide %d: %v", n, err)
	}
	return &pb.SlideContentResource{Content: content}, nil
}

func (s *SlydsServiceImpl) GetDeckConfig(ctx context.Context, req *pb.GetDeckResourceRequest) (*pb.DeckConfigResource, error) {
	d, err := s.deck(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	data, err := d.FS.ReadFile(".slyds.yaml")
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "no .slyds.yaml in deck %q", req.Name)
	}
	return &pb.DeckConfigResource{Content: string(data)}, nil
}

func (s *SlydsServiceImpl) GetAgentGuide(ctx context.Context, req *pb.GetDeckResourceRequest) (*pb.AgentGuideResource, error) {
	d, err := s.deck(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	data, err := d.FS.ReadFile("AGENT.md")
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "no AGENT.md in deck %q", req.Name)
	}
	return &pb.AgentGuideResource{Content: string(data)}, nil
}

// --- Helpers ---

// describeDeck converts a core.Deck into a proto DeckDescription.
func (s *SlydsServiceImpl) describeDeck(d *core.Deck) (*pb.DeckDescription, error) {
	desc, err := d.Describe()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "describe: %v", err)
	}
	var slides []*pb.SlideDescription
	for _, sd := range desc.Slides {
		slides = append(slides, &pb.SlideDescription{
			Position: int32(sd.Position),
			File:     sd.File,
			SlideId:  sd.SlideID,
			Slug:     sd.Slug,
			Layout:   sd.Layout,
			Title:    sd.Title,
			Words:    int32(sd.Words),
			HasNotes: sd.HasNotes,
			Images:   int32(sd.Images),
			Version:  sd.Version,
		})
	}
	deckVer, _ := d.DeckVersion()
	themes := core.AvailableThemeNames()
	layouts, _ := core.ListLayouts()
	return &pb.DeckDescription{
		Title:            d.Title(),
		Theme:            d.Theme(),
		SlideCount:       int32(desc.SlideCount),
		DeckVersion:      deckVer,
		LayoutsUsed:      desc.LayoutsUsed,
		Slides:           slides,
		ThemesAvailable:  themes,
		LayoutsAvailable: layouts,
	}, nil
}

// resolvePosition turns the (slide, position) parameter pair into a 1-based
// position, same logic as the hand-written resolveSlidePosition.
func (s *SlydsServiceImpl) resolvePosition(d *core.Deck, slide *string, position *int32) (int, error) {
	if slide != nil && *slide != "" {
		filename, err := d.ResolveSlide(*slide)
		if err != nil {
			return 0, status.Errorf(codes.NotFound, "resolve slide: %v", err)
		}
		slides, err := d.SlideFilenames()
		if err != nil {
			return 0, status.Errorf(codes.Internal, "slide filenames: %v", err)
		}
		for i, s := range slides {
			if s == filename {
				return i + 1, nil
			}
		}
		return 0, status.Errorf(codes.NotFound, "resolved slide %q not in deck ordering", *slide)
	}
	if position != nil && *position >= 1 {
		return int(*position), nil
	}
	return 0, status.Error(codes.InvalidArgument, "either 'slide' or 'position' is required")
}

// --- Completer implementations ---

// CompleteName returns deck names matching the partial input.
func (s *SlydsServiceImpl) CompleteName(ctx mcpcore.PromptContext, _ mcpcore.CompletionRef, arg mcpcore.CompletionArgument) (mcpcore.CompletionResult, error) {
	ws := workspaceFromContext(ctx)
	if ws == nil {
		return mcpcore.CompletionResult{}, nil
	}
	refs, err := ws.ListDecks()
	if err != nil {
		return mcpcore.CompletionResult{}, nil
	}
	prefix := strings.ToLower(arg.Value)
	var matches []string
	for _, ref := range refs {
		if prefix == "" || strings.HasPrefix(strings.ToLower(ref.Name), prefix) {
			matches = append(matches, ref.Name)
		}
	}
	return mcpcore.CompletionResult{
		Values:  matches,
		Total:   len(matches),
		HasMore: false,
	}, nil
}

// CompleteN returns slide position numbers matching the partial input.
func (s *SlydsServiceImpl) CompleteN(ctx mcpcore.PromptContext, _ mcpcore.CompletionRef, arg mcpcore.CompletionArgument) (mcpcore.CompletionResult, error) {
	ws := workspaceFromContext(ctx)
	if ws == nil {
		return mcpcore.CompletionResult{}, nil
	}
	// Use the first deck as default — same heuristic as hand-written completions.
	refs, err := ws.ListDecks()
	if err != nil || len(refs) == 0 {
		return mcpcore.CompletionResult{}, nil
	}
	d, err := ws.OpenDeck(refs[0].Name)
	if err != nil {
		return mcpcore.CompletionResult{}, nil
	}
	count, err := d.SlideCount()
	if err != nil || count == 0 {
		return mcpcore.CompletionResult{}, nil
	}
	prefix := arg.Value
	var matches []string
	for i := 1; i <= count; i++ {
		s := fmt.Sprintf("%d", i)
		if prefix == "" || strings.HasPrefix(s, prefix) {
			matches = append(matches, s)
		}
	}
	return mcpcore.CompletionResult{
		Values:  matches,
		Total:   len(matches),
		HasMore: false,
	}, nil
}
