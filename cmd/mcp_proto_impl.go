package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	mcpcore "github.com/panyam/mcpkit/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/panyam/slyds/core"
	slydsv1 "github.com/panyam/slyds/gen/go/slyds/v1"
)

// SlydsServiceImpl implements the proto-generated SlydsServiceMCPServer,
// SlydsServiceMCPResourceServer, and SlydsServiceMCPPromptServer interfaces
// by delegating to a Workspace resolved from request context.
type SlydsServiceImpl struct{}

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

func (s *SlydsServiceImpl) ListDecks(ctx mcpcore.ToolContext, req *slydsv1.ListDecksRequest) (*slydsv1.ListDecksResponse, error) {
	ws, err := s.workspace(ctx)
	if err != nil {
		return nil, err
	}
	refs, err := ws.ListDecks()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list decks: %v", err)
	}
	var decks []*slydsv1.DeckSummary
	for _, ref := range refs {
		d, err := ws.OpenDeck(ref.Name)
		if err != nil {
			continue
		}
		count, _ := d.SlideCount()
		decks = append(decks, &slydsv1.DeckSummary{
			Name:   ref.Name,
			Title:  d.Title(),
			Theme:  d.Theme(),
			Slides: int32(count),
		})
	}
	return &slydsv1.ListDecksResponse{Decks: decks}, nil
}

func (s *SlydsServiceImpl) CreateDeck(ctx mcpcore.ToolContext, req *slydsv1.CreateDeckRequest) (*slydsv1.DeckDescription, error) {
	if err := RequireWriteScope(ctx); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "%v", err)
	}
	ws, err := s.workspace(ctx)
	if err != nil {
		return nil, err
	}
	theme := ""
	if req.Theme != nil {
		theme = *req.Theme
	}
	if theme == "" {
		// Use generated elicitation helper.
		choice, action, elicitErr := slydsv1.ElicitThemeChoice(ctx, fmt.Sprintf("Choose a theme for %q:", req.Title))
		if elicitErr == nil && action == "accept" && choice != nil && choice.Theme != "" {
			theme = choice.Theme
		}
		if theme == "" {
			theme = "default"
		}
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

func (s *SlydsServiceImpl) DescribeDeck(ctx mcpcore.ToolContext, req *slydsv1.DeckRequest) (*slydsv1.DeckDescription, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	return s.describeDeck(d)
}

func (s *SlydsServiceImpl) ListSlides(ctx mcpcore.ToolContext, req *slydsv1.DeckRequest) (*slydsv1.ListSlidesResponse, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	desc, err := s.describeDeck(d)
	if err != nil {
		return nil, err
	}
	return &slydsv1.ListSlidesResponse{Slides: desc.Slides}, nil
}

func (s *SlydsServiceImpl) ReadSlide(ctx mcpcore.ToolContext, req *slydsv1.ReadSlideRequest) (*slydsv1.SlideReadResult, error) {
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
	return &slydsv1.SlideReadResult{
		Content:     content,
		Version:     ver,
		DeckVersion: deckVer,
	}, nil
}

func (s *SlydsServiceImpl) EditSlide(ctx mcpcore.ToolContext, req *slydsv1.EditSlideRequest) (*slydsv1.SlideEditResult, error) {
	if err := RequireWriteScope(ctx); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "%v", err)
	}
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
			detail := &slydsv1.VersionConflictDetail{
				CurrentVersion: currentVer,
				CurrentContent: &currentContent,
				DeckVersion:    &deckVer,
			}
			st, _ := status.New(codes.Aborted, "version conflict").WithDetails(detail)
			return nil, st.Err()
		}
	}
	if issues := core.LintSlideContent(req.Content); issues.HasErrors() {
		return nil, status.Errorf(codes.InvalidArgument, "rejected: %s", issues[0].Detail)
	}
	content, _ := core.SanitizeSlideContent(req.Content)
	if err := d.EditSlideContent(pos, content); err != nil {
		return nil, status.Errorf(codes.Internal, "edit slide: %v", err)
	}
	newVer, _ := d.SlideVersion(pos)
	deckVer, _ := d.DeckVersion()
	return &slydsv1.SlideEditResult{
		Version:     newVer,
		DeckVersion: deckVer,
		Position:    int32(pos),
	}, nil
}

func (s *SlydsServiceImpl) QuerySlide(ctx mcpcore.ToolContext, req *slydsv1.QuerySlideRequest) (*slydsv1.QuerySlideResponse, error) {
	// Require write scope only for mutation operations.
	isMutation := req.Set != nil || req.SetHtml != nil || req.SetAttr != nil || req.Append != nil || (req.Remove != nil && *req.Remove)
	if isMutation {
		if err := RequireWriteScope(ctx); err != nil {
			return nil, status.Errorf(codes.PermissionDenied, "%v", err)
		}
	}
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
	data, _ := json.Marshal(results)
	var strs []string
	json.Unmarshal(data, &strs)
	return &slydsv1.QuerySlideResponse{Results: strs}, nil
}

func (s *SlydsServiceImpl) AddSlide(ctx mcpcore.ToolContext, req *slydsv1.AddSlideRequest) (*slydsv1.AddSlideResponse, error) {
	if err := RequireWriteScope(ctx); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "%v", err)
	}
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
			detail := &slydsv1.VersionConflictDetail{
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
	return &slydsv1.AddSlideResponse{
		Slug:        finalSlug,
		SlideId:     slideID,
		DeckVersion: deckVer,
		Position:    req.Position,
	}, nil
}

func (s *SlydsServiceImpl) RemoveSlide(ctx mcpcore.ToolContext, req *slydsv1.RemoveSlideRequest) (*slydsv1.RemoveSlideResponse, error) {
	if err := RequireWriteScope(ctx); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "%v", err)
	}
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
			detail := &slydsv1.VersionConflictDetail{
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
	// Use generated elicitation helper.
	confirmation, action, elicitErr := slydsv1.ElicitRemoveSlideConfirmation(ctx,
		fmt.Sprintf("Remove slide %q from deck %q? This cannot be undone.", filename, req.Deck))
	if elicitErr == nil {
		if action == "decline" || action == "cancel" {
			return &slydsv1.RemoveSlideResponse{DeckVersion: "", RemovedFile: ""}, nil
		}
		if confirmation != nil && !confirmation.Confirm {
			return &slydsv1.RemoveSlideResponse{DeckVersion: "", RemovedFile: ""}, nil
		}
	}
	// ErrElicitationNotSupported: proceed without confirmation (backward compat).

	if err := d.RemoveSlide(filename); err != nil {
		return nil, status.Errorf(codes.Internal, "remove slide: %v", err)
	}
	deckVer, _ := d.DeckVersion()
	count, _ := d.SlideCount()
	return &slydsv1.RemoveSlideResponse{
		DeckVersion: deckVer,
		SlideCount:  int32(count),
		RemovedFile: filename,
	}, nil
}

func (s *SlydsServiceImpl) ImproveSlide(ctx mcpcore.ToolContext, req *slydsv1.ImproveSlideRequest) (*slydsv1.ImproveSlideResponse, error) {
	if err := RequireWriteScope(ctx); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "%v", err)
	}
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	pos, err := resolveSlidePosition(d, req.Slide, 0)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "resolve slide: %v", err)
	}
	content, err := d.GetSlideContent(pos)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read slide %d: %v", pos, err)
	}

	// Use generated sampling helper.
	sampleResult, err := slydsv1.SampleForImproveSlide(ctx, []mcpcore.SamplingMessage{
		{Role: "user", Content: mcpcore.Content{
			Type: "text",
			Text: fmt.Sprintf("Current slide HTML:\n\n%s\n\nInstruction: %s", content, req.Instruction),
		}},
	})
	if errors.Is(err, mcpcore.ErrSamplingNotSupported) {
		return nil, status.Error(codes.FailedPrecondition,
			"sampling not supported by this client — use edit_slide directly with your own content")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sampling failed: %v", err)
	}

	newContent := sampleResult.Content.Text
	if issues := core.LintSlideContent(newContent); issues.HasErrors() {
		return nil, status.Errorf(codes.InvalidArgument,
			"LLM-generated HTML failed lint: %s\n\nRaw output:\n%s", issues[0].Detail, newContent)
	}
	newContent, _ = core.SanitizeSlideContent(newContent)
	if err := d.EditSlideContent(pos, newContent); err != nil {
		return nil, status.Errorf(codes.Internal, "edit slide: %v", err)
	}

	ver, _ := d.SlideVersion(pos)
	deckVer, _ := d.DeckVersion()
	return &slydsv1.ImproveSlideResponse{
		Message:     fmt.Sprintf("Slide %d improved.", pos),
		Version:     ver,
		DeckVersion: deckVer,
	}, nil
}

func (s *SlydsServiceImpl) CheckDeck(ctx mcpcore.ToolContext, req *slydsv1.DeckRequest) (*slydsv1.CheckDeckResponse, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	result, err := d.Check()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "check: %v", err)
	}
	var pbIssues []*slydsv1.Issue
	for _, issue := range result.Issues {
		pbIssues = append(pbIssues, &slydsv1.Issue{
			Type:   issue.Type.String(),
			Slide:  issue.Slide,
			Detail: issue.Detail,
		})
	}
	return &slydsv1.CheckDeckResponse{
		SlideCount:       int32(result.SlideCount),
		InSync:           result.InSync,
		Issues:           pbIssues,
		EstimatedMinutes: result.EstimatedMinutes,
	}, nil
}

func (s *SlydsServiceImpl) BuildDeck(ctx mcpcore.ToolContext, req *slydsv1.DeckRequest) (*slydsv1.BuildDeckResponse, error) {
	d, err := s.deck(ctx, req.Deck)
	if err != nil {
		return nil, err
	}
	result, err := d.Build()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build: %v", err)
	}
	return &slydsv1.BuildDeckResponse{
		Html:     result.HTML,
		Warnings: result.Warnings,
	}, nil
}

// --- Resource implementations ---

func (s *SlydsServiceImpl) GetServerInfo(ctx mcpcore.ResourceContext, req *slydsv1.ServerInfoRequest) (*slydsv1.ServerInfo, error) {
	ws := workspaceFromContext(ctx)
	themes := ws.AvailableThemes()
	layouts, _ := core.ListLayouts()
	info := &slydsv1.ServerInfo{
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

func (s *SlydsServiceImpl) GetDeckList(ctx mcpcore.ResourceContext, req *slydsv1.DeckListRequest) (*slydsv1.ListDecksResponse, error) {
	ws, err := s.workspace(ctx)
	if err != nil {
		return nil, err
	}
	refs, err := ws.ListDecks()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list decks: %v", err)
	}
	var decks []*slydsv1.DeckSummary
	for _, ref := range refs {
		d, err := ws.OpenDeck(ref.Name)
		if err != nil {
			continue
		}
		count, _ := d.SlideCount()
		decks = append(decks, &slydsv1.DeckSummary{
			Name:   ref.Name,
			Title:  d.Title(),
			Theme:  d.Theme(),
			Slides: int32(count),
		})
	}
	return &slydsv1.ListDecksResponse{Decks: decks}, nil
}

func (s *SlydsServiceImpl) GetDeck(ctx mcpcore.ResourceContext, req *slydsv1.GetDeckResourceRequest) (*slydsv1.DeckDescription, error) {
	d, err := s.deck(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	return s.describeDeck(d)
}

func (s *SlydsServiceImpl) GetSlideList(ctx mcpcore.ResourceContext, req *slydsv1.GetSlideListResourceRequest) (*slydsv1.ListSlidesResponse, error) {
	d, err := s.deck(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	desc, err := s.describeDeck(d)
	if err != nil {
		return nil, err
	}
	return &slydsv1.ListSlidesResponse{Slides: desc.Slides}, nil
}

func (s *SlydsServiceImpl) GetSlideContent(ctx mcpcore.ResourceContext, req *slydsv1.GetSlideContentResourceRequest) (*slydsv1.SlideContentResource, error) {
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
	return &slydsv1.SlideContentResource{Content: content}, nil
}

func (s *SlydsServiceImpl) GetDeckConfig(ctx mcpcore.ResourceContext, req *slydsv1.GetDeckResourceRequest) (*slydsv1.DeckConfigResource, error) {
	d, err := s.deck(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	data, err := d.FS.ReadFile(".slyds.yaml")
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "no .slyds.yaml in deck %q", req.Name)
	}
	return &slydsv1.DeckConfigResource{Content: string(data)}, nil
}

func (s *SlydsServiceImpl) GetAgentGuide(ctx mcpcore.ResourceContext, req *slydsv1.GetDeckResourceRequest) (*slydsv1.AgentGuideResource, error) {
	d, err := s.deck(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	data, err := d.FS.ReadFile("AGENT.md")
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "no AGENT.md in deck %q", req.Name)
	}
	return &slydsv1.AgentGuideResource{Content: string(data)}, nil
}

// --- Prompt implementations ---

func (s *SlydsServiceImpl) CreatePresentation(ctx mcpcore.PromptContext, req *slydsv1.CreatePresentationPromptRequest) (*slydsv1.CreatePresentationPromptResponse, error) {
	topic := req.Topic
	if topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	slideCount := "5"
	if req.SlideCount != nil && *req.SlideCount != "" {
		slideCount = *req.SlideCount
	}
	theme := "default"
	if req.Theme != nil && *req.Theme != "" {
		theme = *req.Theme
	}
	var themes []string
	if ws := workspaceFromContext(ctx); ws != nil {
		themes = ws.AvailableThemes()
	} else {
		themes = core.AvailableThemeNames()
	}
	layouts, _ := core.ListLayouts()
	text := fmt.Sprintf(
		"Create a slyds presentation about %q with %s slides using the %q theme.\n\n"+
			"Available themes: %s\n"+
			"Available layouts: %s\n\n"+
			"Steps:\n"+
			"1. Use create_deck to scaffold the deck\n"+
			"2. Use edit_slide on each slide to add content\n"+
			"3. Use check_deck to validate\n"+
			"4. Use build_deck to produce the final HTML",
		topic, slideCount, theme,
		strings.Join(themes, ", "),
		strings.Join(layouts, ", "),
	)
	return &slydsv1.CreatePresentationPromptResponse{
		Description: fmt.Sprintf("Create a presentation about %q", topic),
		Text:        text,
	}, nil
}

func (s *SlydsServiceImpl) ReviewSlides(ctx mcpcore.PromptContext, req *slydsv1.ReviewSlidesPromptRequest) (*slydsv1.ReviewSlidesPromptResponse, error) {
	name := req.Name
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	ws := workspaceFromContext(ctx)
	if ws == nil {
		return nil, fmt.Errorf("no workspace available")
	}
	d, err := ws.OpenDeck(name)
	if err != nil {
		return nil, fmt.Errorf("deck %q: %w", name, err)
	}
	desc, err := d.Describe()
	if err != nil {
		return nil, fmt.Errorf("describe deck: %w", err)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Review the presentation %q (%d slides, theme: %s) for clarity, flow, and consistency.\n\n",
		desc.Title, desc.SlideCount, desc.Theme)
	for i := 1; i <= desc.SlideCount; i++ {
		content, err := d.GetSlideContent(i)
		if err != nil {
			continue
		}
		fmt.Fprintf(&sb, "--- Slide %d ---\n%s\n\n", i, content)
	}
	sb.WriteString("Provide specific feedback on each slide and overall flow suggestions.")
	return &slydsv1.ReviewSlidesPromptResponse{
		Description: fmt.Sprintf("Review %q (%d slides)", desc.Title, desc.SlideCount),
		Text:        sb.String(),
	}, nil
}

func (s *SlydsServiceImpl) SuggestSpeakerNotes(ctx mcpcore.PromptContext, req *slydsv1.SuggestSpeakerNotesPromptRequest) (*slydsv1.SuggestSpeakerNotesPromptResponse, error) {
	name := req.Name
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	slide := req.Slide
	if slide == "" {
		return nil, fmt.Errorf("slide is required")
	}
	ws := workspaceFromContext(ctx)
	if ws == nil {
		return nil, fmt.Errorf("no workspace available")
	}
	d, err := ws.OpenDeck(name)
	if err != nil {
		return nil, fmt.Errorf("deck %q: %w", name, err)
	}
	pos, err := resolveSlidePosition(d, slide, 0)
	if err != nil {
		return nil, fmt.Errorf("resolve slide: %w", err)
	}
	content, err := d.GetSlideContent(pos)
	if err != nil {
		return nil, fmt.Errorf("read slide %d: %w", pos, err)
	}
	desc, _ := d.Describe()
	title := name
	if desc != nil {
		title = desc.Title
	}
	text := fmt.Sprintf(
		"Draft speaker notes for slide %d of the presentation %q.\n\n"+
			"Slide content:\n%s\n\n"+
			"The notes should:\n"+
			"- Complement the visual content, not repeat it\n"+
			"- Provide talking points and transitions\n"+
			"- Include timing guidance (approximate minutes)",
		pos, title, content,
	)
	return &slydsv1.SuggestSpeakerNotesPromptResponse{
		Description: fmt.Sprintf("Speaker notes for slide %d of %q", pos, title),
		Text:        text,
	}, nil
}

// --- Helpers ---

// describeDeck converts a core.Deck into a proto DeckDescription.
func (s *SlydsServiceImpl) describeDeck(d *core.Deck) (*slydsv1.DeckDescription, error) {
	desc, err := d.Describe()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "describe: %v", err)
	}
	var slides []*slydsv1.SlideDescription
	for _, sd := range desc.Slides {
		slides = append(slides, &slydsv1.SlideDescription{
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
	// TODO: pass workspace context to include external themes
	themes := core.AvailableThemeNames()
	layouts, _ := core.ListLayouts()
	return &slydsv1.DeckDescription{
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

func (s *SlydsServiceImpl) CompleteN(ctx mcpcore.PromptContext, _ mcpcore.CompletionRef, arg mcpcore.CompletionArgument) (mcpcore.CompletionResult, error) {
	ws := workspaceFromContext(ctx)
	if ws == nil {
		return mcpcore.CompletionResult{}, nil
	}
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
