# Checkpoint 10: UI Components

## Scope
Implement UI components using a terminal UI framework.

## Files to Inspect

### Source Files
| File | Purpose |
|-------|---------|
| `src/components/App.tsx` | Root app component |
| `src/components/design-system/*.tsx` | Design system |
| `src/components/PromptInput/*.tsx` | Prompt input |
| `src/components/*/*.tsx` | Other components |

### Target Structure
```
internal/ui/components/
├── app.go              # Root app
├── dialog.go           # Dialog component
├── text.go            # Text component
├── box.go             # Box layout
├── input.go           # Text input
├── list.go            # List component
├── spinner.go         # Spinner
└── [other components]

// Components that need different approach in Go
├── fuzzy_picker.go     # FuzzyPicker - TBD approach
├── tabs.go            # Tabs - achievable
├── progress_bar.go    # ProgressBar - achievable
└── [dialogs]
    ├── model_picker.go
    ├── config_dialog.go
    └── [other dialogs]
```

## Note on UI Framework

The TypeScript codebase uses React + Ink for terminal UI. In Go, options include:
1. **bubbletea** (Charm) - Most similar to React patterns
2. **tview** - Rich terminal UI
3. **lipgloss** - Styling library
4. **gocui** - Low-level UI library

Recommendation: Use **bubbletea** for React-like patterns or **tview** for rich UI.

## Implementation Details

### 10.1 Using Bubbletea
```go
// components/app.go
type AppModel struct {
    state app_state.AppState
    // other state
}

func (m AppModel) Init() tea.Cmd
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m AppModel) View() string
```

### 10.2 Example Component
```go
// components/spinner.go
type SpinnerModel struct {
    text string
    done bool
}

func NewSpinner(text string) SpinnerModel
func (m SpinnerModel) View() string
func (m SpinnerModel) Tick() SpinnerModel
```

## Parity Checklist
- [ ] App root component
- [ ] Design system components
- [ ] Prompt input
- [ ] Dialogs
- [ ] Lists
- [ ] Status displays
- [ ] Tool result rendering

## Note
This checkpoint may require significant rework due to the fundamental difference between React/JSX and Go terminal UI approaches.

## Next Checkpoint
- [Checkpoint 11: Advanced Features](./checkpoint-11.md)