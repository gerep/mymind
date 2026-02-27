package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Views & Focus ──────────────────────────────────────────

type tuiView int

const (
	viewList tuiView = iota
	viewDetail
	viewAdd
	viewRecap
	viewConfirmDelete
)

type tuiFocus int

const (
	focusSearch tuiFocus = iota
	focusTags
)

// ─── Model ──────────────────────────────────────────────────

type tuiModel struct {
	vault   string
	noteDir string
	apiKey  string
	aiModel string
	prompt  string

	allNotes []Note
	filtered []Note

	search   textinput.Model
	addInput textinput.Model
	viewport viewport.Model

	cursor int
	offset int
	view   tuiView
	focus  tuiFocus
	width  int
	height int
	ready  bool

	// Detail view
	selected     *Note
	relatedNotes []relatedNote

	// Preview pane
	previewContent string

	// Tag bar
	allTags    []tagInfo
	activeTags map[string]bool
	tagCursor  int

	// Status
	status string
	saving bool

	// Help overlay
	showHelp bool

	// Recap
	recapVP      viewport.Model
	recapLoading bool

	// Delete confirmation
	deleteTarget *Note
}

type tagInfo struct {
	Name  string
	Count int
}

// ─── Messages ───────────────────────────────────────────────

type noteSavedMsg struct {
	err   error
	notes []Note
}

type recapDoneMsg struct {
	err     error
	content string
}

type noteDeletedMsg struct {
	err   error
	notes []Note
}

// ─── Styles ─────────────────────────────────────────────────

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7dc4e4"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e738d"))

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#363a4f")).
			Foreground(lipgloss.Color("#cad3f5")).
			Bold(true)

	tagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6da95"))

	activeTagStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1e2030")).
			Background(lipgloss.Color("#a6da95"))

	tagCursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6da95")).
			Background(lipgloss.Color("#363a4f")).
			Bold(true)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#cad3f5")).
			Background(lipgloss.Color("#24273a")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e738d")).
			Padding(0, 1)

	previewBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#363a4f")).
				Padding(0, 1)

	helpBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7dc4e4")).
			Padding(1, 3)

	relatedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f5a97f"))

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ed8796")).
			Bold(true)
)

// ─── Constructor & Init ─────────────────────────────────────

func newTUIModel(vault, noteDir, apiKey, aiModel, prompt string, notes []Note) tuiModel {
	ti := textinput.New()
	ti.Placeholder = "Search notes..."
	ti.Focus()
	ti.CharLimit = 256

	ai := textinput.New()
	ai.Placeholder = "Type a note or paste a URL..."
	ai.CharLimit = 2048

	m := tuiModel{
		vault:      vault,
		noteDir:    noteDir,
		apiKey:     apiKey,
		aiModel:    aiModel,
		prompt:     prompt,
		allNotes:   notes,
		filtered:   notes,
		search:     ti,
		addInput:   ai,
		activeTags: make(map[string]bool),
	}
	m.collectTags()
	return m
}

func (m tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

// ─── Update Routing ─────────────────────────────────────────

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		if m.view == viewDetail {
			m.viewport.Width = m.width
			m.viewport.Height = m.height - 4
		}
		if m.view == viewRecap {
			m.recapVP.Width = m.width
			m.recapVP.Height = m.height - 3
		}
		return m, nil

	case noteSavedMsg:
		m.saving = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.allNotes = msg.notes
			m.collectTags()
			m.filterNotes()
			m.status = "✓ Note saved!"
		}
		return m, nil

	case recapDoneMsg:
		m.recapLoading = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Error: %v", msg.err)
			m.view = viewList
		} else {
			vp := viewport.New(m.width, m.height-3)
			vp.SetContent(msg.content)
			m.recapVP = vp
		}
		return m, nil

	case noteDeletedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.allNotes = msg.notes
			m.collectTags()
			m.filterNotes()
			m.status = "✓ Note deleted"
		}
		return m, nil

	case tea.KeyMsg:
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		switch m.view {
		case viewDetail:
			return m.updateDetail(msg)
		case viewAdd:
			return m.updateAdd(msg)
		case viewRecap:
			return m.updateRecap(msg)
		case viewConfirmDelete:
			return m.updateConfirmDelete(msg)
		default:
			if m.focus == focusTags {
				return m.updateTagBar(msg)
			}
			return m.updateList(msg)
		}
	}

	switch m.view {
	case viewList:
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd
	case viewAdd:
		var cmd tea.Cmd
		m.addInput, cmd = m.addInput.Update(msg)
		return m, cmd
	case viewDetail:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case viewRecap:
		var cmd tea.Cmd
		m.recapVP, cmd = m.recapVP.Update(msg)
		return m, cmd
	}
	return m, nil
}

// ─── List View Update ───────────────────────────────────────

func (m tuiModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		if m.search.Value() != "" {
			m.search.SetValue("")
			m.filterNotes()
			return m, nil
		}
		if len(m.activeTags) > 0 {
			m.activeTags = make(map[string]bool)
			m.filterNotes()
			return m, nil
		}
		return m, tea.Quit

	case "?":
		m.showHelp = true
		return m, nil

	case "ctrl+a":
		if m.saving {
			return m, nil
		}
		m.view = viewAdd
		m.addInput.SetValue("")
		m.addInput.Focus()
		m.status = ""
		return m, textinput.Blink

	case "ctrl+d":
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			note := m.filtered[m.cursor]
			m.deleteTarget = &note
			m.view = viewConfirmDelete
		}
		return m, nil

	case "ctrl+r":
		if m.apiKey == "" {
			m.status = "Error: GEMINI_API_KEY not set"
			return m, nil
		}
		m.view = viewRecap
		m.recapLoading = true
		return m, m.generateRecapCmd()

	case "tab":
		if len(m.allTags) > 0 {
			m.focus = focusTags
			m.search.Blur()
		}
		return m, nil

	case "up", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
			m.updatePreview()
		}
		return m, nil

	case "down", "ctrl+n":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			listH := m.listHeight()
			if m.cursor >= m.offset+listH {
				m.offset = m.cursor - listH + 1
			}
			m.updatePreview()
		}
		return m, nil

	case "enter":
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			m.openNoteDetail(m.filtered[m.cursor])
		}
		return m, nil

	default:
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.filterNotes()
		return m, cmd
	}
}

// ─── Tag Bar Update ─────────────────────────────────────────

func (m tuiModel) updateTagBar(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "tab", "esc":
		m.focus = focusSearch
		m.search.Focus()
		return m, textinput.Blink

	case "left", "h":
		if m.tagCursor > 0 {
			m.tagCursor--
		}
		return m, nil

	case "right", "l":
		if m.tagCursor < len(m.allTags)-1 {
			m.tagCursor++
		}
		return m, nil

	case " ", "enter":
		if m.tagCursor < len(m.allTags) {
			tag := m.allTags[m.tagCursor].Name
			if m.activeTags[tag] {
				delete(m.activeTags, tag)
			} else {
				m.activeTags[tag] = true
			}
			m.filterNotes()
		}
		return m, nil

	case "up", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
			m.updatePreview()
		}
		return m, nil

	case "down", "ctrl+n":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			listH := m.listHeight()
			if m.cursor >= m.offset+listH {
				m.offset = m.cursor - listH + 1
			}
			m.updatePreview()
		}
		return m, nil

	case "?":
		m.showHelp = true
		return m, nil
	}
	return m, nil
}

// ─── Detail View Update ─────────────────────────────────────

func (m tuiModel) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.view = viewList
		m.selected = nil
		m.relatedNotes = nil
		return m, nil
	case "?":
		m.showHelp = true
		return m, nil
	case "ctrl+d":
		if m.selected != nil {
			m.deleteTarget = m.selected
			m.view = viewConfirmDelete
		}
		return m, nil
	}

	// 1-9: jump to related note
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		idx := int(key[0] - '1')
		if idx < len(m.relatedNotes) {
			name := m.relatedNotes[idx].Name
			if note := m.findNoteByName(name); note != nil {
				m.openNoteDetail(*note)
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// ─── Add View Update ────────────────────────────────────────

func (m tuiModel) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.view = viewList
		m.search.Focus()
		return m, textinput.Blink
	case "enter":
		text := strings.TrimSpace(m.addInput.Value())
		if text == "" {
			return m, nil
		}
		if m.apiKey == "" {
			m.status = "Error: GEMINI_API_KEY not set"
			m.view = viewList
			m.search.Focus()
			return m, textinput.Blink
		}
		m.saving = true
		if isURL(text) {
			m.status = "Fetching & analyzing..."
		} else {
			m.status = "Analyzing with AI..."
		}
		m.view = viewList
		m.search.Focus()
		return m, m.saveInputCmd(text)
	default:
		var cmd tea.Cmd
		m.addInput, cmd = m.addInput.Update(msg)
		return m, cmd
	}
}

// ─── Recap View Update ──────────────────────────────────────

func (m tuiModel) updateRecap(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.view = viewList
		return m, nil
	case "?":
		m.showHelp = true
		return m, nil
	default:
		var cmd tea.Cmd
		m.recapVP, cmd = m.recapVP.Update(msg)
		return m, cmd
	}
}

// ─── Delete Confirmation Update ─────────────────────────────

func (m tuiModel) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.deleteTarget != nil {
			target := *m.deleteTarget
			m.deleteTarget = nil
			m.view = viewList
			m.selected = nil
			return m, m.deleteNoteCmd(target)
		}
		return m, nil
	case "n", "N", "esc":
		prev := viewList
		if m.selected != nil {
			prev = viewDetail
		}
		m.deleteTarget = nil
		m.view = prev
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// ─── Filtering & Search ─────────────────────────────────────

func (m *tuiModel) filterNotes() {
	query := strings.TrimSpace(m.search.Value())

	var results []Note
	for i := range m.allNotes {
		n := &m.allNotes[i]

		// Active tag filters (from tag bar)
		if len(m.activeTags) > 0 {
			allMatch := true
			for tag := range m.activeTags {
				if !hasTag(n.Tags, tag) {
					allMatch = false
					break
				}
			}
			if !allMatch {
				continue
			}
		}

		// Search query
		if query != "" {
			if !m.matchQuery(n, query) {
				continue
			}
		}

		results = append(results, m.allNotes[i])
	}

	m.filtered = results
	if m.cursor >= len(m.filtered) {
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		} else {
			m.cursor = 0
		}
	}
	m.offset = 0
	m.updatePreview()
}

func (m *tuiModel) matchQuery(n *Note, query string) bool {
	tokens := strings.Fields(query)
	var tagTokens, textTokens []string
	for _, t := range tokens {
		if strings.HasPrefix(t, "#") {
			tagTokens = append(tagTokens, strings.ToLower(t[1:]))
		} else {
			textTokens = append(textTokens, t)
		}
	}

	// Tag tokens: exact match
	for _, tag := range tagTokens {
		if !hasTag(n.Tags, tag) {
			return false
		}
	}

	// Text tokens: fuzzy match on title, fallback to tags and body
	for _, tok := range textTokens {
		if matched, _ := fuzzyMatch(tok, n.Title); matched {
			continue
		}
		// Check tags
		foundInTags := false
		for _, t := range n.Tags {
			if strings.Contains(strings.ToLower(t), strings.ToLower(tok)) {
				foundInTags = true
				break
			}
		}
		if foundInTags {
			continue
		}
		// Fall back to body
		if n.Summary == "" {
			_ = loadNoteBody(n)
		}
		if !strings.Contains(strings.ToLower(n.Summary), strings.ToLower(tok)) {
			return false
		}
	}
	return true
}

func fuzzyMatch(pattern, str string) (bool, int) {
	p := strings.ToLower(pattern)
	s := strings.ToLower(str)

	pi := 0
	score := 0
	consecutive := 0

	for si := 0; si < len(s) && pi < len(p); si++ {
		if s[si] == p[pi] {
			pi++
			consecutive++
			score += consecutive
			if si == 0 || !unicode.IsLetter(rune(s[si-1])) {
				score += 5
			}
		} else {
			consecutive = 0
		}
	}

	return pi == len(p), score
}

// ─── Tag Management ─────────────────────────────────────────

func (m *tuiModel) collectTags() {
	counts := make(map[string]int)
	for _, n := range m.allNotes {
		for _, t := range n.Tags {
			counts[t]++
		}
	}
	var tags []tagInfo
	for name, count := range counts {
		tags = append(tags, tagInfo{Name: name, Count: count})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count != tags[j].Count {
			return tags[i].Count > tags[j].Count
		}
		return tags[i].Name < tags[j].Name
	})
	maxTags := 15
	if len(tags) > maxTags {
		tags = tags[:maxTags]
	}
	m.allTags = tags
	if m.tagCursor >= len(m.allTags) {
		m.tagCursor = 0
	}
}

// ─── Content Loading ────────────────────────────────────────

var embedRe = regexp.MustCompile(`!\[\[([^\]]+)\]\]`)

func (m tuiModel) loadNoteContent(n Note) string {
	data, err := os.ReadFile(n.Path)
	if err != nil {
		return fmt.Sprintf("Error reading note: %v", err)
	}

	raw := string(data)

	// Strip frontmatter
	if strings.HasPrefix(raw, "---") {
		if idx := strings.Index(raw[3:], "---"); idx >= 0 {
			raw = strings.TrimSpace(raw[idx+6:])
		}
	}

	noteDir := filepath.Dir(n.Path)
	raw = embedRe.ReplaceAllStringFunc(raw, func(match string) string {
		sub := embedRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		name := sub[1]
		if !isImageFile(name) {
			return match
		}
		imgPath := filepath.Join(noteDir, name)
		rendered, err := renderImageToText(imgPath, m.width-4)
		if err != nil {
			return dimStyle.Render(fmt.Sprintf("[image: %s]", name))
		}
		return rendered
	})

	// Append related notes section
	related := findRelated(n, m.allNotes)
	if len(related) > 0 {
		raw += "\n\n" + strings.Repeat("─", min(m.width-4, 40)) + "\n"
		raw += relatedStyle.Render("Related Notes") + "\n\n"
		for i, r := range related {
			if i >= 9 {
				break
			}
			tags := dimStyle.Render("(" + strings.Join(r.SharedTags, ", ") + ")")
			raw += fmt.Sprintf("  %s  %s  %s\n",
				relatedStyle.Render(fmt.Sprintf("[%d]", i+1)),
				titleStyle.Render(r.Title),
				tags,
			)
		}
	}

	return raw
}

func (m tuiModel) loadPreview(n Note) string {
	data, err := os.ReadFile(n.Path)
	if err != nil {
		return ""
	}

	raw := string(data)
	if strings.HasPrefix(raw, "---") {
		if idx := strings.Index(raw[3:], "---"); idx >= 0 {
			raw = strings.TrimSpace(raw[idx+6:])
		}
	}

	// Replace image embeds with placeholder text
	raw = embedRe.ReplaceAllString(raw, dimStyle.Render("[image]"))

	// Strip the heading (first # line)
	lines := strings.Split(raw, "\n")
	var out []string
	skippedHeading := false
	for _, line := range lines {
		if !skippedHeading && strings.HasPrefix(strings.TrimSpace(line), "# ") {
			skippedHeading = true
			continue
		}
		out = append(out, line)
	}

	maxLines := 30
	if len(out) > maxLines {
		out = out[:maxLines]
		out = append(out, dimStyle.Render("..."))
	}

	return strings.Join(out, "\n")
}

func (m *tuiModel) updatePreview() {
	if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
		m.previewContent = m.loadPreview(m.filtered[m.cursor])
	} else {
		m.previewContent = ""
	}
}

func (m tuiModel) truncatePreview(maxLines, maxWidth int) string {
	if m.previewContent == "" {
		return ""
	}
	lines := strings.Split(m.previewContent, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	if maxWidth > 0 {
		for i, line := range lines {
			lines[i] = lipgloss.NewStyle().MaxWidth(maxWidth).Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *tuiModel) openNoteDetail(note Note) {
	m.selected = &note
	m.relatedNotes = findRelated(note, m.allNotes)
	m.view = viewDetail

	content := m.loadNoteContent(note)
	wrapped := lipgloss.NewStyle().Width(m.width).Render(content)
	vp := viewport.New(m.width, m.height-4)
	vp.SetContent(wrapped)
	m.viewport = vp
}

func (m tuiModel) findNoteByName(name string) *Note {
	for i := range m.allNotes {
		base := strings.TrimSuffix(filepath.Base(m.allNotes[i].Path), ".md")
		if base == name {
			return &m.allNotes[i]
		}
	}
	return nil
}

func isImageFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

func (m tuiModel) listHeight() int {
	// header(1) + search(1) + tag bar(1) + status(1) = 4
	h := m.height - 4
	if h < 0 {
		h = 0
	}
	return h
}

// ─── View Routing ───────────────────────────────────────────

func (m tuiModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	var content string
	switch m.view {
	case viewDetail:
		content = m.viewDetail()
	case viewAdd:
		content = m.viewAdd()
	case viewRecap:
		content = m.viewRecap()
	case viewConfirmDelete:
		content = m.viewConfirmDelete()
	default:
		content = m.viewList()
	}

	if m.showHelp {
		help := m.renderHelp()
		content = lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			help,
			lipgloss.WithWhitespaceChars("·"),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("#24273a")),
		)
	}

	return content
}

// ─── List View ──────────────────────────────────────────────

func (m tuiModel) viewList() string {
	var sb strings.Builder

	// Header
	header := headerStyle.Width(m.width).Render("mymind")
	sb.WriteString(header)
	sb.WriteRune('\n')

	// Search bar
	sb.WriteString(m.search.View())
	sb.WriteRune('\n')

	// Tag bar
	sb.WriteString(m.renderTagBar())
	sb.WriteRune('\n')

	listH := m.listHeight()

	showPreview := m.width >= 80
	listWidth := m.width
	previewWidth := 0
	if showPreview {
		listWidth = m.width * 45 / 100
		previewWidth = m.width - listWidth - 3 // 3 for separator
	}

	// Build list lines
	var listLines []string
	end := m.offset + listH
	if end > len(m.filtered) {
		end = len(m.filtered)
	}
	visible := m.filtered[m.offset:end]

	for i, note := range visible {
		idx := m.offset + i
		line := m.renderListItem(note, idx, listWidth)
		listLines = append(listLines, line)
	}
	for i := len(listLines); i < listH; i++ {
		listLines = append(listLines, "")
	}

	if showPreview && listH > 2 {
		// Truncate preview content to fit the box
		previewContentH := listH - 2 // border top + bottom
		preview := m.truncatePreview(previewContentH, previewWidth-4) // -4 for border + padding
		previewBox := previewBorderStyle.
			Width(previewWidth).
			Height(previewContentH).
			Render(preview)

		listContent := strings.Join(listLines, "\n")
		joined := lipgloss.JoinHorizontal(lipgloss.Top, listContent, " ", previewBox)
		sb.WriteString(joined)
	} else {
		sb.WriteString(strings.Join(listLines, "\n"))
	}
	sb.WriteRune('\n')

	// Status bar
	statusText := fmt.Sprintf("%d/%d notes  ? help", len(m.filtered), len(m.allNotes))
	if m.status != "" {
		statusText = m.status + "  │  " + statusText
	}
	sb.WriteString(statusStyle.Render(statusText))

	return sb.String()
}

func (m tuiModel) renderListItem(note Note, idx, maxWidth int) string {
	date := note.Created.Format("2006-01-02")
	kind := fmt.Sprintf("[%s]", note.Kind)
	tags := ""
	if len(note.Tags) > 0 {
		tags = " " + tagStyle.Render("#"+strings.Join(note.Tags, " #"))
	}

	if idx == m.cursor {
		line := fmt.Sprintf("▸ %s  %-6s  %s", date, kind, note.Title)
		if maxWidth > 0 && len(line) > maxWidth {
			line = line[:maxWidth-1] + "…"
		}
		return selectedStyle.Render(line) + tags
	}

	title := titleStyle.Render(note.Title)
	line := fmt.Sprintf("  %s  %s  %s",
		dimStyle.Render(date),
		dimStyle.Render(kind),
		title,
	)
	return line + tags
}

func (m tuiModel) renderTagBar() string {
	if len(m.allTags) == 0 {
		return dimStyle.Render("tags: (none)")
	}

	prefix := dimStyle.Render("tags:")
	if m.focus == focusTags {
		prefix = tagStyle.Render("tags:")
	}

	used := lipgloss.Width(prefix) + 1 // +1 for space after prefix
	var parts []string
	for i, t := range m.allTags {
		label := t.Name
		active := m.activeTags[label]
		isCursor := m.focus == focusTags && i == m.tagCursor

		var styled string
		switch {
		case active && isCursor:
			styled = activeTagStyle.Underline(true).Render(" " + label + " ")
		case active:
			styled = activeTagStyle.Render(" " + label + " ")
		case isCursor:
			styled = tagCursorStyle.Render(" " + label + " ")
		default:
			styled = dimStyle.Render(" " + label + " ")
		}

		w := lipgloss.Width(styled) + 1 // +1 for space separator
		if m.width > 0 && used+w > m.width {
			break
		}
		used += w
		parts = append(parts, styled)
	}

	return prefix + " " + strings.Join(parts, " ")
}

// ─── Detail View ────────────────────────────────────────────

func (m tuiModel) viewDetail() string {
	var sb strings.Builder

	title := ""
	if m.selected != nil {
		title = m.selected.Title
	}
	header := headerStyle.Width(m.width).Render(title)
	sb.WriteString(header)
	sb.WriteRune('\n')

	if m.selected != nil {
		tags := ""
		if len(m.selected.Tags) > 0 {
			tags = tagStyle.Render("#" + strings.Join(m.selected.Tags, " #"))
		}
		meta := dimStyle.Render(fmt.Sprintf("%s  [%s]", m.selected.Created.Format("2006-01-02"), m.selected.Kind))
		if tags != "" {
			meta += "  " + tags
		}
		sb.WriteString(meta)
	}
	sb.WriteRune('\n')
	sb.WriteRune('\n')

	sb.WriteString(m.viewport.View())
	sb.WriteRune('\n')

	sb.WriteString(statusStyle.Render("esc back  ↑↓ scroll  1-9 related  ctrl+d delete  ? help"))

	return sb.String()
}

// ─── Add View ───────────────────────────────────────────────

func (m tuiModel) viewAdd() string {
	var sb strings.Builder

	header := headerStyle.Width(m.width).Render("Add Note")
	sb.WriteString(header)
	sb.WriteRune('\n')
	sb.WriteRune('\n')

	sb.WriteString("  ")
	sb.WriteString(m.addInput.View())
	sb.WriteRune('\n')
	sb.WriteRune('\n')

	sb.WriteString(dimStyle.Render("  Type a text note or paste a URL (web page, tweet, image URL)"))
	sb.WriteRune('\n')
	sb.WriteString(dimStyle.Render("  Enter to save  •  Esc to cancel"))

	return sb.String()
}

// ─── Recap View ─────────────────────────────────────────────

func (m tuiModel) viewRecap() string {
	var sb strings.Builder

	header := headerStyle.Width(m.width).Render("Recap — Last 7 Days")
	sb.WriteString(header)
	sb.WriteRune('\n')
	sb.WriteRune('\n')

	if m.recapLoading {
		sb.WriteString(dimStyle.Render("  Generating recap with AI..."))
	} else {
		sb.WriteString(m.recapVP.View())
	}
	sb.WriteRune('\n')

	sb.WriteString(statusStyle.Render("esc back  ↑↓ scroll"))

	return sb.String()
}

// ─── Delete Confirmation ────────────────────────────────────

func (m tuiModel) viewConfirmDelete() string {
	name := ""
	if m.deleteTarget != nil {
		name = m.deleteTarget.Title
	}

	box := helpBoxStyle.Render(
		warnStyle.Render("Delete note?") + "\n\n" +
			titleStyle.Render(name) + "\n\n" +
			dimStyle.Render("This cannot be undone.") + "\n\n" +
			"Press " + warnStyle.Render("y") + " to confirm, " +
			dimStyle.Render("n") + " to cancel",
	)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
		lipgloss.WithWhitespaceChars("·"),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#24273a")),
	)
}

// ─── Help Overlay ───────────────────────────────────────────

func (m tuiModel) renderHelp() string {
	sections := []string{
		headerStyle.Render("  Keyboard Shortcuts  "),
		"",
		titleStyle.Render("List View"),
		"  ↑/↓           Navigate notes",
		"  Enter          View note",
		"  Ctrl+A         Add note (text or URL)",
		"  Ctrl+D         Delete note",
		"  Ctrl+R         Generate recap (7 days)",
		"  Tab            Focus tag bar",
		"  Esc            Clear search → Clear tags → Quit",
		"",
		titleStyle.Render("Tag Bar") + dimStyle.Render("  (Tab to focus)"),
		"  ←/→            Navigate tags",
		"  Space/Enter    Toggle tag filter",
		"  Tab/Esc        Back to search",
		"",
		titleStyle.Render("Detail View"),
		"  ↑/↓            Scroll content",
		"  1-9            Jump to related note",
		"  Ctrl+D         Delete this note",
		"  Esc/q          Back to list",
		"",
		titleStyle.Render("Add / Recap"),
		"  Esc/q          Back to list",
		"",
		dimStyle.Render("        Press any key to close"),
	}

	return helpBoxStyle.Render(strings.Join(sections, "\n"))
}

// ─── Async Commands ─────────────────────────────────────────

func (m tuiModel) saveInputCmd(input string) tea.Cmd {
	vault := m.vault
	noteDir := m.noteDir
	apiKey := m.apiKey
	aiModel := m.aiModel
	prompt := m.prompt

	return func() tea.Msg {
		input = strings.TrimSpace(input)

		var content ContentInput
		var err error

		if isURL(input) {
			content, err = resolveInput(input)
			if err != nil {
				return noteSavedMsg{err: err}
			}
		} else {
			content = ContentInput{Kind: "note", Text: input}
		}

		hash := contentHash(content)
		notes, _ := loadNotes(vault)
		if dup := findDuplicate(notes, hash); dup != nil {
			return noteSavedMsg{err: fmt.Errorf("duplicate: already saved as %s", dup.Path)}
		}

		result, err := analyze(apiKey, aiModel, prompt, content)
		if err != nil {
			return noteSavedMsg{err: fmt.Errorf("AI analysis failed: %w", err)}
		}

		path, err := writeMarkdown(noteDir, content, result, hash)
		if err != nil {
			return noteSavedMsg{err: fmt.Errorf("could not write: %w", err)}
		}

		appendRelatedToNewNote(path, vault)
		refreshed, _ := loadNotes(vault)
		return noteSavedMsg{notes: refreshed}
	}
}

func (m tuiModel) deleteNoteCmd(note Note) tea.Cmd {
	vault := m.vault
	notePath := note.Path
	return func() tea.Msg {
		if err := os.Remove(notePath); err != nil {
			return noteDeletedMsg{err: err}
		}
		notes, _ := loadNotes(vault)
		return noteDeletedMsg{notes: notes}
	}
}

func (m tuiModel) generateRecapCmd() tea.Cmd {
	apiKey := m.apiKey
	aiModel := m.aiModel
	allNotes := m.allNotes
	return func() tea.Msg {
		dur, _ := parsePeriod("7d")
		cutoff := time.Now().Add(-dur)

		var recent []Note
		for _, n := range allNotes {
			if n.Created.After(cutoff) {
				recent = append(recent, n)
			}
		}

		if len(recent) == 0 {
			return recapDoneMsg{err: fmt.Errorf("no notes found in the last 7 days")}
		}

		for i := range recent {
			_ = loadNoteBody(&recent[i])
		}

		var sb strings.Builder
		for _, n := range recent {
			fmt.Fprintf(&sb, "## %s\nDate: %s\n", n.Title, n.Created.Format("2006-01-02 15:04"))
			if len(n.Tags) > 0 {
				fmt.Fprintf(&sb, "Tags: %s\n", strings.Join(n.Tags, ", "))
			}
			if n.Summary != "" {
				fmt.Fprintf(&sb, "\n%s\n", n.Summary)
			}
			sb.WriteString("\n---\n\n")
			if sb.Len() > 30000 {
				break
			}
		}

		text := sb.String()
		if len(text) > 30000 {
			text = text[:30000]
		}

		recap, err := generateRecap(apiKey, aiModel, text, "7d")
		if err != nil {
			return recapDoneMsg{err: err}
		}
		return recapDoneMsg{content: recap}
	}
}

// ─── Entry Point ────────────────────────────────────────────

func runTUI(vault, noteDir, apiKey, model, prompt string) {
	notes, err := loadNotes(vault)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading notes: %v\n", err)
		os.Exit(1)
	}

	m := newTUIModel(vault, noteDir, apiKey, model, prompt, notes)
	m.updatePreview()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
