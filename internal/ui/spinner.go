package ui

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

// getDefaultCharacters returns spinner characters based on platform/terminal
// Matches src/components/Spinner/utils.ts getDefaultCharacters()
func getDefaultCharacters() []string {
	// Check for Ghostty terminal
	// Note: In Go, we'd need to check TERM env var
	// For now, use the darwin vs other platform distinction
	if runtime.GOOS == "darwin" {
		return []string{"·", "✢", "✳", "✶", "✻", "✽"}
	}
	return []string{"·", "✢", "*", "✶", "✻", "✽"}
}

// SpinnerFrames contains the animation frames (forward + reverse)
// Matches: SPINNER_FRAMES = [...DEFAULT_CHARACTERS, ...[...DEFAULT_CHARACTERS].reverse()]
var SpinnerFrames []string

func init() {
	chars := getDefaultCharacters()
	// Create forward frames
	SpinnerFrames = append(SpinnerFrames, chars...)
	// Create reversed frames
	for i := len(chars) - 1; i >= 0; i-- {
		SpinnerFrames = append(SpinnerFrames, chars[i])
	}
}

// SpinnerVerbs contains the animated verbs for the loading spinner
// Matches the full list from src/constants/spinnerVerbs.ts
var SpinnerVerbs = []string{
	"Accomplishing",
	"Actioning",
	"Actualizing",
	"Architecting",
	"Baking",
	"Beaming",
	"Beboppin'",
	"Befuddling",
	"Billowing",
	"Blanching",
	"Bloviating",
	"Boogieing",
	"Boondoggling",
	"Booping",
	"Bootstrapping",
	"Brewing",
	"Bunning",
	"Burrowing",
	"Calculating",
	"Canoodling",
	"Caramelizing",
	"Cascading",
	"Catapulting",
	"Cerebrating",
	"Channeling",
	"Channelling",
	"Choreographing",
	"Churning",
	"Clauding",
	"Coalescing",
	"Cogitating",
	"Combobulating",
	"Composing",
	"Computing",
	"Concocting",
	"Considering",
	"Contemplating",
	"Cooking",
	"Crafting",
	"Creating",
	"Crunching",
	"Crystallizing",
	"Cultivating",
	"Deciphering",
	"Deliberating",
	"Determining",
	"Dilly-dallying",
	"Discombobulating",
	"Doing",
	"Doodling",
	"Drizzling",
	"Ebbing",
	"Effecting",
	"Elucidating",
	"Embellishing",
	"Enchanting",
	"Envisioning",
	"Evaporating",
	"Fermenting",
	"Fiddle-faddling",
	"Finagling",
	"Flambéing",
	"Flibbertigibbeting",
	"Flowing",
	"Flummoxing",
	"Fluttering",
	"Forging",
	"Forming",
	"Frolicking",
	"Frosting",
	"Gallivanting",
	"Galloping",
	"Garnishing",
	"Generating",
	"Gesticulating",
	"Germinating",
	"Gitifying",
	"Grooving",
	"Gusting",
	"Harmonizing",
	"Hashing",
	"Hatching",
	"Herding",
	"Honking",
	"Hullaballooing",
	"Hyperspacing",
	"Ideating",
	"Imagining",
	"Improvising",
	"Incubating",
	"Inferring",
	"Infusing",
	"Ionizing",
	"Jitterbugging",
	"Julienning",
	"Kneading",
	"Leavening",
	"Levitating",
	"Lollygagging",
	"Manifesting",
	"Marinating",
	"Meandering",
	"Metamorphosing",
	"Misting",
	"Moonwalking",
	"Moseying",
	"Mulling",
	"Mustering",
	"Musing",
	"Nebulizing",
	"Nesting",
	"Newspapering",
	"Noodling",
	"Nucleating",
	"Orbiting",
	"Orchestrating",
	"Osmosing",
	"Perambulating",
	"Percolating",
	"Perusing",
	"Philosophising",
	"Photosynthesizing",
	"Pollinating",
	"Pondering",
	"Pontificating",
	"Pouncing",
	"Precipitating",
	"Prestidigitating",
	"Processing",
	"Proofing",
	"Propagating",
	"Puttering",
	"Puzzling",
	"Quantumizing",
	"Razzle-dazzling",
	"Razzmatazzing",
	"Recombobulating",
	"Reticulating",
	"Roosting",
	"Ruminating",
	"Sautéing",
	"Scampering",
	"Schlepping",
	"Scurrying",
	"Seasoning",
	"Shenaniganing",
	"Shimmying",
	"Simmering",
	"Skedaddling",
	"Sketching",
	"Slithering",
	"Smooshing",
	"Sock-hopping",
	"Spelunking",
	"Spinning",
	"Sprouting",
	"Stewing",
	"Sublimating",
	"Swirling",
	"Swooping",
	"Symbioting",
	"Synthesizing",
	"Tempering",
	"Thinking",
	"Thundering",
	"Tinkering",
	"Tomfoolering",
	"Topsy-turvying",
	"Transfiguring",
	"Transmuting",
	"Twisting",
	"Undulating",
	"Unfurling",
	"Unravelling",
	"Vibing",
	"Waddling",
	"Wandering",
	"Warping",
	"Whatchamacalliting",
	"Whirlpooling",
	"Whirring",
	"Whisking",
	"Wibbling",
	"Working",
	"Wrangling",
	"Zesting",
	"Zigzagging",
}

// ShimmerIntervalMs is the interval for shimmer animation tick (ms)
// Matches SHIMMER_INTERVAL_MS in bridgeStatusUtil.ts
const ShimmerIntervalMs = 150

// SpinnerState holds the state for the spinner animation
type SpinnerState struct {
	mu            sync.RWMutex
	startTime     time.Time
	randomVerb    string
	displayedLen  int // For smooth token counter animation
	reducedMotion bool
}

// SpinnerState creates a new spinner state with a randomly selected verb
func NewSpinnerState() *SpinnerState {
	return SpinnerStateFor()
}

func SpinnerStateFor() *SpinnerState {
	return &SpinnerState{
		startTime:  time.Now(),
		randomVerb: GetRandomVerb(), // Selected ONCE at creation
	}
}

// Reset resets the spinner state with a new random verb
func (s *SpinnerState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startTime = time.Now()
	s.randomVerb = GetRandomVerb()
	s.displayedLen = 0
}

// GetVerb returns the randomly selected verb (constant during a request)
func (s *SpinnerState) GetVerb() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.randomVerb
}

// SetVerb sets a specific verb (for override cases)
func (s *SpinnerState) SetVerb(verb string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.randomVerb = verb
}

// Elapsed returns the elapsed time since start
func (s *SpinnerState) Elapsed() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.startTime)
}

// UpdateDisplayedLength smoothly animates the displayed response length
// Matches the token counter animation in SpinnerAnimationRow.tsx
func (s *SpinnerState) UpdateDisplayedLength(currentLen int) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.reducedMotion {
		s.displayedLen = currentLen
		return currentLen
	}

	gap := currentLen - s.displayedLen
	if gap <= 0 {
		return s.displayedLen
	}

	var increment int
	if gap < 70 {
		increment = 3
	} else if gap < 200 {
		increment = int(math.Max(8, math.Ceil(float64(gap)*0.15)))
	} else {
		increment = 50
	}

	s.displayedLen = int(math.Min(float64(s.displayedLen+increment), float64(currentLen)))
	return s.displayedLen
}

// SetReducedMotion sets reduced motion preference
func (s *SpinnerState) SetReducedMotion(reduced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reducedMotion = reduced
}

// GetRandomVerb returns a randomly selected verb (like lodash sample())
func GetRandomVerb() string {
	if len(SpinnerVerbs) == 0 {
		return "Working"
	}
	return SpinnerVerbs[rand.Intn(len(SpinnerVerbs))]
}

// GetSpinnerFrame returns the spinner frame based on time (120ms per frame)
// Matches: frame = Math.floor(time / 120) in SpinnerAnimationRow.tsx
func GetSpinnerFrame(timeMs int) string {
	if len(SpinnerFrames) == 0 {
		return "·"
	}
	frame := timeMs / 120
	return SpinnerFrames[frame%len(SpinnerFrames)]
}

// ComputeGlimmerIndex computes the glimmer index for shimmer animation
// Matches computeGlimmerIndex in bridgeStatusUtil.ts
func ComputeGlimmerIndex(tick, messageWidth int, mode string, isStalled, reducedMotion bool) int {
	if reducedMotion || isStalled {
		return -100
	}

	cycleLength := messageWidth + 20
	cyclePosition := tick

	// glimmerSpeed: 50 for 'requesting', 200 otherwise
	glimmerSpeed := 200
	if mode == "requesting" {
		glimmerSpeed = 50
	}
	cyclePosition = tick / glimmerSpeed

	if mode == "requesting" {
		return (cyclePosition % cycleLength) - 10
	}
	return messageWidth + 10 - (cyclePosition % cycleLength)
}

// ShimmerSegments represents the three parts of text for shimmer rendering
type ShimmerSegments struct {
	Before  string
	Shimmer string
	After   string
}

// ComputeShimmerSegments splits text into three segments by visual column position
func ComputeShimmerSegments(text string, glimmerIndex int) ShimmerSegments {
	messageWidth := visibleWidth(text)
	shimmerStart := glimmerIndex - 1
	shimmerEnd := glimmerIndex + 1

	// When shimmer is offscreen, return all text as "before"
	if shimmerStart >= messageWidth || shimmerEnd < 0 {
		return ShimmerSegments{Before: text, Shimmer: "", After: ""}
	}

	clampedStart := max(0, shimmerStart)
	colPos := 0
	before := ""
	shimmer := ""
	after := ""

	for _, r := range text {
		segWidth := runeCellWidth(r)
		if colPos+segWidth <= clampedStart {
			before += string(r)
		} else if colPos > shimmerEnd {
			after += string(r)
		} else {
			shimmer += string(r)
		}
		colPos += segWidth
	}

	return ShimmerSegments{Before: before, Shimmer: shimmer, After: after}
}

// SpinnerMode represents the different spinner modes
type SpinnerMode string

const (
	SpinnerModeThinking   SpinnerMode = "thinking"
	SpinnerModeToolUse    SpinnerMode = "tool-use"
	SpinnerModeRequesting SpinnerMode = "requesting"
	SpinnerModeResponding SpinnerMode = "responding"
	SpinnerModeToolInput  SpinnerMode = "tool-input"
)

// FormatDuration formats a duration in a human-readable way
// Matches formatDuration in src/utils/format.ts
// - ms == 0: "0s"
// - ms >= 1 and < 60000 (60s): integer seconds like "5s", "10s", "59s"
// - ms >= 60000: minutes + seconds format like "1m 30s" (with space)
func FormatDuration(ms int) string {
	// Special case for 0
	if ms == 0 {
		return "0s"
	}
	// For durations < 60 seconds, show integer seconds
	if ms < 60000 {
		seconds := ms / 1000
		if seconds == 0 {
			return "0s"
		}
		return fmt.Sprintf("%ds", seconds)
	}

	// Calculate days, hours, minutes, seconds
	days := ms / 86400000
	hours := (ms % 86400000) / 3600000
	minutes := (ms % 3600000) / 60000
	seconds := (ms % 60000) / 1000

	// Handle rounding carry-over
	if seconds == 60 {
		seconds = 0
		minutes++
	}
	if minutes == 60 {
		minutes = 0
		hours++
	}
	if hours == 24 {
		hours = 0
		days++
	}

	// Format with spaces between units (matching TS: `${minutes}m ${seconds}s`)
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// FormatNumber formats a number with commas for thousands
// Matches formatNumber in src/utils/format.ts
func FormatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	s := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

// RGBColor represents an RGB color
type RGBColor struct {
	R, G, B int
}

// InterpolateColor interpolates between two RGB colors
// Matches interpolateColor in Spinner/utils.ts
func InterpolateColor(color1, color2 RGBColor, t float64) RGBColor {
	return RGBColor{
		R: int(math.Round(float64(color1.R) + float64(color2.R-color1.R)*t)),
		G: int(math.Round(float64(color1.G) + float64(color2.G-color1.G)*t)),
		B: int(math.Round(float64(color1.B) + float64(color2.B-color1.B)*t)),
	}
}

// RenderSpinner renders the spinner with frame animation
func RenderSpinner(timeMs int, verb string, messageColor *rgb, reducedMotion bool) string {
	if reducedMotion {
		// Reduced motion: static dot with slow pulse
		return style(messageColor, nil, "●", true)
	}

	frame := GetSpinnerFrame(timeMs)
	return style(messageColor, nil, frame, false)
}

// RenderSpinnerRow renders a complete spinner row with verb and status
// Matches the SpinnerAnimationRow component
func RenderSpinnerRow(timeMs int, verb string, toolName string, elapsedMs int, tokenCount int, width int, mode SpinnerMode, reducedMotion bool) string {
	// Frame character (120ms per frame, matching TS: Math.floor(time / 120))
	frame := GetSpinnerFrame(timeMs)
	if reducedMotion {
		frame = "●"
	}

	// Build the message with shimmer
	message := verb + "…"

	// Compute glimmer for shimmer effect
	// TS: glimmerSpeed = mode === 'requesting' ? 50 : 200
	glimmerSpeed := 200
	if mode == SpinnerModeRequesting {
		glimmerSpeed = 50
	}
	messageWidth := visibleWidth(message)
	cycleLength := messageWidth + 20
	cyclePosition := timeMs / glimmerSpeed

	// Compute glimmer index (TS: cyclePosition % cycleLength - 10 for requesting, messageWidth + 10 - cyclePosition % cycleLength for responding)
	var glimmerIndex int
	if reducedMotion {
		glimmerIndex = -100 // Off-screen
	} else if mode == SpinnerModeRequesting {
		glimmerIndex = (cyclePosition % cycleLength) - 10
	} else {
		glimmerIndex = messageWidth + 10 - (cyclePosition % cycleLength)
	}

	// Compute shimmer segments (TS: shimmerStart = glimmerIndex - 1, shimmerEnd = glimmerIndex + 1)
	shimmerStart := glimmerIndex - 1
	shimmerEnd := glimmerIndex + 1

	var messageText string
	if shimmerStart < messageWidth && shimmerEnd >= 0 && !reducedMotion {
		// Split message into before/shimmer/after parts
		clampedStart := shimmerStart
		if clampedStart < 0 {
			clampedStart = 0
		}

		colPos := 0
		var before, shimmer, after string
		for _, r := range message {
			segWidth := runeCellWidth(r)
			if colPos+segWidth <= clampedStart {
				before += string(r)
			} else if colPos > shimmerEnd {
				after += string(r)
			} else {
				shimmer += string(r)
			}
			colPos += segWidth
		}

		// Render with colors:
		// - before/after: claude orange (messageColor)
		// - shimmer: lighter claude orange (shimmerColor)
		messageText = style(&dark.claude, nil, before, false) +
			style(&dark.claudeShimmer, nil, shimmer, true) +
			style(&dark.claude, nil, after, false)
	} else {
		// No shimmer or reduced motion: solid claude color
		messageText = style(&dark.claude, nil, message, false)
	}

	// Build status parts
	var statusParts []string

	// Elapsed time
	if elapsedMs > 0 {
		statusParts = append(statusParts, FormatDuration(elapsedMs))
	}

	// Token count
	if tokenCount > 0 {
		tokenText := FormatNumber(tokenCount) + " tokens"
		statusParts = append(statusParts, tokenText)
	}

	// Build the row
	var result string
	result = style(&dark.claude, nil, frame+" ", false) + messageText

	if toolName != "" {
		result += " " + style(&dark.muted, nil, "["+toolName+"]", false)
	}

	if len(statusParts) > 0 {
		statusStr := ""
		for i, part := range statusParts {
			if i > 0 {
				statusStr += " · "
			}
			statusStr += part
		}
		result += " " + style(&dark.muted, nil, "("+statusStr+")", false)
	}

	return result
}
