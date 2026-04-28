package model

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"claude-go/internal/command"
	"claude-go/internal/config"
	"claude-go/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

var MODEL_ALIASES = []string{
	"opus",
	"sonnet",
	"haiku",
	"claude-opus-4-6",
	"claude-sonnet-4-6",
	"claude-haiku-4-5",
	"gpt-4o",
	"gpt-4o-mini",
	"gpt-4-turbo",
	"o1",
	"o1-mini",
	"o3-mini",
}

var COMMON_INFO_ARGS = []string{
	"--info",
	"--show",
	"--current",
	"--status",
}

var COMMON_HELP_ARGS = []string{
	"--help",
	"-h",
	"help",
}

var POPULAR_MODELS = []string{
	"claude-opus-4-6 (alias: opus)",
	"claude-sonnet-4-6 (alias: sonnet)",
	"claude-haiku-4-5 (alias: haiku)",
	"gpt-4o",
	"gpt-4o-mini",
	"gpt-4-turbo",
	"o1",
	"o1-mini",
	"o3-mini",
}

func Register(r *command.Registry) {
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocalJSX,
		Name:         "model",
		Description:  "Manage models and API profiles",
		ArgumentHint: "[model|list|add|use|remove|current]",
		Aliases:      []string{"models"},
		Handler:      handleModelCommand,
		Load:         loadModelCommandModel,
	})
}

func handleModelCommand(ctx context.Context, rt command.Runtime, args []string) (string, error) {
	_ = ctx
	arg := ""
	if len(args) > 0 {
		arg = strings.TrimSpace(args[0])
	}
	fullArg := strings.TrimSpace(strings.Join(args, " "))

	for _, helpArg := range COMMON_HELP_ARGS {
		if arg == helpArg {
			return buildModelHelp(), nil
		}
	}

	switch strings.ToLower(arg) {
	case "list", "ls":
		return listAPIProfiles(rt)
	case "current", "status":
		return showCurrentProfile(rt)
	case "add", "set":
		return addAPIProfile(rt, args[1:])
	case "use", "switch":
		return useAPIProfile(rt, args[1:])
	case "remove", "rm", "delete":
		return removeAPIProfile(args[1:])
	}

	for _, infoArg := range COMMON_INFO_ARGS {
		if arg == infoArg {
			return showCurrentModel(rt), nil
		}
	}

	if fullArg != "" {
		return setModel(fullArg, rt)
	}

	return buildModelPickerInfo(rt), nil
}

func buildModelHelp() string {
	lines := []string{
		"Usage: /model [model-name or alias]",
		"",
		"Commands:",
		"  /model          Show current model and available models",
		"  /model <name>   Switch to specified model for this session",
		"  /model default  Reset to configured model",
		"  /model list     List saved API profiles",
		"  /model current  Show current API profile/config",
		"  /model add <name> key=<api-key> url=<base-url> model=<model> [summary=<model>]",
		"  /model use <name>     Switch to a saved API profile",
		"  /model remove <name>  Delete a saved API profile",
		"  /model --info   Show current model info",
		"  /model --help   Show this help",
		"",
		"Aliases:",
		"  opus    -> claude-opus-4-6",
		"  sonnet  -> claude-sonnet-4-6",
		"  haiku   -> claude-haiku-4-5",
		"",
		"Examples:",
		"  /model opus",
		"  /model add work key=sk-... url=https://api.example.com/v1/chat/completions model=gpt-4.1 summary=gpt-4.1-mini",
		"  /model use work",
	}
	return strings.Join(lines, "\n")
}

func showCurrentModel(rt command.Runtime) string {
	currentModel := getCurrentModel(rt)
	displayModel := renderModelLabel(currentModel)

	sessionOverride := ""
	if rt.State != nil {
		state := rt.State.Snapshot()
		if state.MainLoopModel != "" && state.MainLoopModel != currentModel {
			sessionOverride = fmt.Sprintf("\nSession override: %s", state.MainLoopModel)
		}
	}

	return fmt.Sprintf("Current model: %s%s\nBase URL: %s", displayModel, sessionOverride, emptyDash(rt.Config.BaseURL))
}

func setModel(arg string, rt command.Runtime) (string, error) {
	if arg == "default" {
		if rt.State != nil {
			rt.State.SetMainLoopModel("")
		}
		if rt.Provider != nil && rt.Config.Model != "" {
			rt.Provider.SetModel(rt.Config.Model)
		}
		return "Reset to configured model", nil
	}

	resolvedModel := arg
	if isKnownAlias(arg) {
		resolvedModel = resolveAlias(arg)
	}

	if rt.State != nil {
		rt.State.SetMainLoopModel(resolvedModel)
	}
	if rt.Provider != nil {
		rt.Provider.SetModel(resolvedModel)
	}
	nextCfg := rt.Config
	nextCfg.Model = resolvedModel
	if err := applyRuntimeConfig(rt, nextCfg); err != nil {
		return "", err
	}

	return fmt.Sprintf("Set model to %s", renderModelLabel(resolvedModel)), nil
}

func buildModelPickerInfo(rt command.Runtime) string {
	currentModel := getCurrentModel(rt)
	displayModel := renderModelLabel(currentModel)

	lines := []string{
		fmt.Sprintf("Current model: %s", displayModel),
		fmt.Sprintf("Base URL: %s", emptyDash(rt.Config.BaseURL)),
		"",
		"Available models:",
	}

	for _, model := range POPULAR_MODELS {
		lines = append(lines, fmt.Sprintf("  %s", model))
	}

	lines = append(lines, "")
	lines = append(lines, "Aliases: opus, sonnet, haiku")
	lines = append(lines, "")
	lines = append(lines, "Usage: /model <model-name>")
	lines = append(lines, "Profiles: /model list, /model add <name> key=<key> url=<url> model=<model>, /model use <name>")

	return strings.Join(lines, "\n")
}

func listAPIProfiles(rt command.Runtime) (string, error) {
	store, err := config.LoadAPIProfiles()
	if err != nil {
		return "", err
	}
	lines := []string{
		"API profiles",
		fmt.Sprintf("Store: %s", config.APIProfilesPath()),
		"",
		fmt.Sprintf("Current: model=%s  base_url=%s", emptyDash(rt.Config.Model), emptyDash(rt.Config.BaseURL)),
	}
	profiles := store.SortedProfiles()
	if len(profiles) == 0 {
		lines = append(lines, "", "No saved profiles.", "Add one with:", "  /model add <name> key=<api-key> url=<base-url> model=<model> [summary=<model>]")
		return strings.Join(lines, "\n"), nil
	}
	lines = append(lines, "", "Saved profiles:")
	for _, profile := range profiles {
		marker := " "
		if profile.Name == store.Active {
			marker = "*"
		}
		lines = append(lines, fmt.Sprintf("%s %s  model=%s  url=%s  key=%s", marker, profile.Name, profile.Model, profile.BaseURL, maskAPIKey(profile.APIKey)))
		if profile.SummaryModel != "" {
			lines = append(lines, fmt.Sprintf("    summary=%s", profile.SummaryModel))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func showCurrentProfile(rt command.Runtime) (string, error) {
	store, err := config.LoadAPIProfiles()
	if err != nil {
		return "", err
	}
	active := store.Active
	if active == "" {
		active = "(none)"
	}
	return strings.Join([]string{
		"Current API configuration",
		fmt.Sprintf("Active profile: %s", active),
		fmt.Sprintf("Model: %s", emptyDash(rt.Config.Model)),
		fmt.Sprintf("Summary model: %s", emptyDash(rt.Config.SummaryModel)),
		fmt.Sprintf("Base URL: %s", emptyDash(rt.Config.BaseURL)),
		fmt.Sprintf("API key: %s", maskAPIKey(rt.Config.APIKey)),
		fmt.Sprintf("Profiles file: %s", config.APIProfilesPath()),
	}, "\n"), nil
}

func addAPIProfile(rt command.Runtime, args []string) (string, error) {
	profile, err := parseProfileArgs(args)
	if err != nil {
		return "", err
	}
	store, err := config.LoadAPIProfiles()
	if err != nil {
		return "", err
	}
	if err := store.Upsert(profile); err != nil {
		return "", err
	}
	store.Active = profile.Name
	if err := config.SaveAPIProfiles(store); err != nil {
		return "", err
	}
	applied := config.ApplyAPIProfile(rt.Config, store.Profiles[profile.Name])
	if err := applyRuntimeConfig(rt, applied); err != nil {
		return "", err
	}
	return fmt.Sprintf("Saved and switched to API profile %q · model=%s · url=%s", profile.Name, profile.Model, profile.BaseURL), nil
}

func useAPIProfile(rt command.Runtime, args []string) (string, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return "", fmt.Errorf("usage: /model use <name>")
	}
	name := strings.TrimSpace(args[0])
	store, err := config.LoadAPIProfiles()
	if err != nil {
		return "", err
	}
	profile, ok := store.Profiles[name]
	if !ok {
		return "", fmt.Errorf("API profile %q not found", name)
	}
	store.Active = name
	if err := config.SaveAPIProfiles(store); err != nil {
		return "", err
	}
	applied := config.ApplyAPIProfile(rt.Config, profile)
	if err := applyRuntimeConfig(rt, applied); err != nil {
		return "", err
	}
	return fmt.Sprintf("Switched to API profile %q · model=%s · url=%s", name, profile.Model, profile.BaseURL), nil
}

func removeAPIProfile(args []string) (string, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return "", fmt.Errorf("usage: /model remove <name>")
	}
	name := strings.TrimSpace(args[0])
	store, err := config.LoadAPIProfiles()
	if err != nil {
		return "", err
	}
	if !store.Remove(name) {
		return "", fmt.Errorf("API profile %q not found", name)
	}
	if err := config.SaveAPIProfiles(store); err != nil {
		return "", err
	}
	return fmt.Sprintf("Removed API profile %q", name), nil
}

func parseProfileArgs(args []string) (config.APIProfile, error) {
	if len(args) == 0 || config.CleanConfigValue(args[0]) == "" {
		return config.APIProfile{}, fmt.Errorf("usage: /model add <name> key=<api-key> url=<base-url> model=<model> [summary=<model>]")
	}
	profile := config.APIProfile{Name: config.CleanConfigValue(args[0])}
	positional := []string{}
	for _, raw := range args[1:] {
		part := config.CleanConfigValue(raw)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			positional = append(positional, part)
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "key", "api_key", "apikey", "token":
			profile.APIKey = config.CleanConfigValue(value)
		case "url", "base_url", "baseurl":
			profile.BaseURL = config.CleanConfigValue(value)
		case "model":
			profile.Model = config.CleanConfigValue(value)
		case "summary", "summary_model", "summarymodel":
			profile.SummaryModel = config.CleanConfigValue(value)
		}
	}
	if len(positional) > 0 && profile.APIKey == "" {
		profile.APIKey = config.CleanConfigValue(positional[0])
	}
	if len(positional) > 1 && profile.BaseURL == "" {
		profile.BaseURL = config.CleanConfigValue(positional[1])
	}
	if len(positional) > 2 && profile.Model == "" {
		profile.Model = config.CleanConfigValue(positional[2])
	}
	if len(positional) > 3 && profile.SummaryModel == "" {
		profile.SummaryModel = config.CleanConfigValue(positional[3])
	}
	return profile, nil
}

func applyRuntimeConfig(rt command.Runtime, cfg config.Config) error {
	if rt.OnConfigChange != nil {
		return rt.OnConfigChange(cfg)
	}
	if rt.Provider != nil {
		rt.Provider.Configure(cfg)
	}
	if rt.State != nil {
		rt.State.SetCurrentModel(cfg.Model)
	}
	return nil
}

type modelWizardStep struct {
	label       string
	key         string
	optional    bool
	secret      bool
	placeholder string
}

var modelWizardSteps = []modelWizardStep{
	{label: "Profile name", key: "name", placeholder: "work"},
	{label: "API key", key: "key", secret: true, placeholder: "sk-..."},
	{label: "Base URL", key: "url", placeholder: "https://api.example.com/v1/chat/completions"},
	{label: "Model", key: "model", placeholder: "gpt-4.1"},
	{label: "Summary model", key: "summary", optional: true, placeholder: "optional"},
}

type modelCommandModel struct {
	rt     command.Runtime
	step   int
	input  string
	values map[string]string
	err    string
	done   string
	width  int
}

func loadModelCommandModel(ctx context.Context, rt command.Runtime, args []string) (tea.Model, error) {
	if len(args) > 0 {
		result, err := handleModelCommand(ctx, rt, args)
		if err != nil {
			return nil, err
		}
		return modelResultModel{body: result, rt: rt}, nil
	}
	values := map[string]string{
		"key":     config.CleanConfigValue(rt.Config.APIKey),
		"url":     config.CleanConfigValue(rt.Config.BaseURL),
		"model":   config.CleanConfigValue(rt.Config.Model),
		"summary": config.CleanConfigValue(rt.Config.SummaryModel),
	}
	return modelCommandModel{
		rt:     rt,
		values: values,
		input:  "",
	}, nil
}

func (m modelCommandModel) Init() tea.Cmd { return nil }

func (m modelCommandModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			m.finish("Model setup cancelled", "system")
			return m, tea.Quit
		case tea.KeyBackspace:
			m.input = dropLastRune(m.input)
			m.err = ""
			return m, nil
		case tea.KeyEnter:
			return m.submit()
		case tea.KeyRunes:
			m.input += string(msg.Runes)
			m.err = ""
			return m, nil
		case tea.KeySpace:
			m.input += " "
			m.err = ""
			return m, nil
		}
	}
	return m, nil
}

func (m modelCommandModel) submit() (tea.Model, tea.Cmd) {
	current := modelWizardSteps[m.step]
	value := config.CleanConfigValue(m.input)
	if value == "" {
		value = config.CleanConfigValue(m.values[current.key])
	}
	if value == "" && !current.optional {
		m.err = current.label + " is required"
		return m, nil
	}
	m.values[current.key] = value
	m.input = ""
	m.err = ""
	if m.step < len(modelWizardSteps)-1 {
		m.step++
		return m, nil
	}

	profile := config.APIProfile{
		Name:         m.values["name"],
		APIKey:       m.values["key"],
		BaseURL:      m.values["url"],
		Model:        m.values["model"],
		SummaryModel: m.values["summary"],
	}
	store, err := config.LoadAPIProfiles()
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	if err := store.Upsert(profile); err != nil {
		m.err = err.Error()
		return m, nil
	}
	store.Active = profile.Name
	if err := config.SaveAPIProfiles(store); err != nil {
		m.err = err.Error()
		return m, nil
	}
	applied := config.ApplyAPIProfile(m.rt.Config, store.Profiles[profile.Name])
	if err := applyRuntimeConfig(m.rt, applied); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.done = fmt.Sprintf("Saved and switched to API profile %q · model=%s", profile.Name, profile.Model)
	m.finish(m.done, "command")
	return m, tea.Quit
}

func (m modelCommandModel) View() string {
	if m.done != "" {
		return m.done
	}
	step := modelWizardSteps[m.step]
	var b strings.Builder
	b.WriteString(ui.Style(&ui.Dark.Claude, nil, "Model API setup", true))
	b.WriteString("\n")
	b.WriteString(ui.Style(&ui.Dark.Muted, nil, fmt.Sprintf("Step %d of %d  %s", m.step+1, len(modelWizardSteps), renderWizardProgress(m.step, len(modelWizardSteps))), false))
	b.WriteString("\n\n")
	b.WriteString(ui.Style(&ui.Dark.Claude, nil, step.label, true))
	if step.optional {
		b.WriteString(ui.Style(&ui.Dark.Muted, nil, " optional", false))
	}
	b.WriteString("\n")
	if existing := strings.TrimSpace(m.values[step.key]); existing != "" {
		if step.secret {
			b.WriteString(ui.Style(&ui.Dark.Muted, nil, fmt.Sprintf("Current: %s", maskAPIKey(existing)), false))
		} else {
			b.WriteString(ui.Style(&ui.Dark.Muted, nil, fmt.Sprintf("Current: %s", existing), false))
		}
		b.WriteString("\n")
		b.WriteString(ui.Style(&ui.Dark.Muted, nil, "Press Enter to keep it, or type a new value.", false))
		b.WriteString("\n")
	} else if step.placeholder != "" {
		b.WriteString(ui.Style(&ui.Dark.Muted, nil, fmt.Sprintf("Example: %s", step.placeholder), false))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(ui.Style(&ui.Dark.Muted, nil, "Input", true))
	b.WriteString("\n")
	b.WriteString(renderWizardInputBox(m.input, step.secret, m.width))
	if m.err != "" {
		b.WriteString("\n\n")
		b.WriteString(ui.Style(&ui.Dark.Error, nil, m.err, true))
	}
	b.WriteString("\n\n")
	b.WriteString(ui.Style(&ui.Dark.Muted, nil, "Enter next · Esc cancel", true))
	return strings.TrimRight(b.String(), "\n")
}

func renderWizardProgress(step, total int) string {
	if total <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < total; i++ {
		if i <= step {
			b.WriteString("#")
		} else {
			b.WriteString("-")
		}
	}
	b.WriteString("]")
	return b.String()
}

func renderWizardInputBox(input string, secret bool, width int) string {
	display := input
	if secret {
		display = strings.Repeat("*", utf8.RuneCountInString(input))
	}

	boxWidth := 56
	if width > 0 {
		boxWidth = width - 10
		if boxWidth < 24 {
			boxWidth = 24
		}
		if boxWidth > 72 {
			boxWidth = 72
		}
	}
	contentWidth := boxWidth - 4
	if contentWidth < 1 {
		contentWidth = 1
	}
	display = truncateRunes(display, contentWidth-1)
	padding := contentWidth - utf8.RuneCountInString(display) - 1
	if padding < 0 {
		padding = 0
	}

	borderTop := ui.Style(&ui.Dark.Muted, nil, "╭"+strings.Repeat("─", boxWidth-2)+"╮", false)
	borderBottom := ui.Style(&ui.Dark.Muted, nil, "╰"+strings.Repeat("─", boxWidth-2)+"╯", false)
	left := ui.Style(&ui.Dark.Muted, nil, "│ ", false)
	right := ui.Style(&ui.Dark.Muted, nil, " │", false)
	cursor := ui.Style(&ui.Dark.Claude, nil, "|", true)
	line := left + display + cursor + strings.Repeat(" ", padding) + right
	return borderTop + "\n" + line + "\n" + borderBottom
}

func (m modelCommandModel) finish(result, display string) {
	if m.rt.OnLocalJSXDone != nil {
		m.rt.OnLocalJSXDone(result, command.LocalJSXDoneOptions{Display: display})
	} else if m.rt.OnExit != nil {
		m.rt.OnExit()
	}
}

type modelResultModel struct {
	body string
	rt   command.Runtime
}

func (m modelResultModel) Init() tea.Cmd { return nil }

func (m modelResultModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyEnter, tea.KeyEsc, tea.KeyCtrlC:
			if m.rt.OnLocalJSXDone != nil {
				m.rt.OnLocalJSXDone(m.body, command.LocalJSXDoneOptions{Display: "command"})
			} else if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m modelResultModel) View() string {
	return strings.TrimRight(m.body+"\n\nEnter close · Esc close", "\n")
}

func dropLastRune(s string) string {
	if s == "" {
		return ""
	}
	_, size := utf8.DecodeLastRuneInString(s)
	if size <= 0 {
		return ""
	}
	return s[:len(s)-size]
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "~"
}

func getCurrentModel(rt command.Runtime) string {
	if rt.State != nil {
		state := rt.State.Snapshot()
		if state.MainLoopModel != "" {
			return state.MainLoopModel
		}
	}
	if rt.Config.Model != "" {
		return rt.Config.Model
	}
	return "default"
}

func renderModelLabel(model string) string {
	if model == "" || model == "default" {
		return "default (configured model)"
	}
	return model
}

func isKnownAlias(model string) bool {
	modelLower := strings.ToLower(strings.TrimSpace(model))
	for _, alias := range MODEL_ALIASES {
		if alias == modelLower {
			return true
		}
	}
	return false
}

func resolveAlias(alias string) string {
	aliasLower := strings.ToLower(strings.TrimSpace(alias))
	aliasMap := map[string]string{
		"opus":              "claude-opus-4-6",
		"sonnet":            "claude-sonnet-4-6",
		"haiku":             "claude-haiku-4-5",
		"claude-opus-4-6":   "claude-opus-4-6",
		"claude-sonnet-4-6": "claude-sonnet-4-6",
		"claude-haiku-4-5":  "claude-haiku-4-5",
	}
	if resolved, ok := aliasMap[aliasLower]; ok {
		return resolved
	}
	return alias
}

func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "-"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func emptyDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}
