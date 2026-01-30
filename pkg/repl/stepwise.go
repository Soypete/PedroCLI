package repl

import (
	"fmt"
	"strings"

	"github.com/soypete/pedrocli/pkg/agents"
)

// handleInteractivePhased handles phase-by-phase interactive execution
// Shows results after each phase and asks for user approval
// NOTE: This runs SYNCHRONOUSLY (not async) so prompts work in REPL
func (r *REPL) handleInteractivePhased(agentName string, prompt string) error {
	r.output.PrintMessage("\n🔍 Starting interactive execution\n")
	r.output.PrintMessage("   You'll review and approve each phase\n\n")

	// Create phase callback
	phaseCallback := func(phase agents.Phase, result *agents.PhaseResult) (bool, error) {
		// Skip approval for analyze phase (just information gathering)
		if phase.Name == "analyze" {
			r.output.PrintMessage("\n✅ Analysis complete - continuing to planning phase...\n\n")
			return true, nil
		}

		// Show phase summary
		r.showPhaseSummary(phase, result)

		// Ask user what to do next
		action, feedback, err := r.askPhaseAction()
		if err != nil {
			return false, err
		}

		switch action {
		case "c", "continue", "":
			// Continue to next phase
			return true, nil

		case "f", "feedback":
			// Retry with user feedback
			// TODO: Need to implement retry mechanism in executor
			r.output.PrintWarning("\n⚠️  Retry with feedback not fully implemented yet\n")
			r.output.PrintMessage("   Your feedback: %s\n", feedback)
			r.output.PrintMessage("   Continuing for now...\n\n")
			return true, nil

		case "x", "cancel":
			r.output.PrintWarning("\n❌ Task cancelled by user\n")
			return false, fmt.Errorf("task cancelled by user")

		default:
			r.output.PrintWarning("⚠️  Invalid action: %s (using continue)\n", action)
			return true, nil
		}
	}

	// Show Pedro
	ShowPedro(r.output.writer)
	r.output.PrintMessage("\n🤖 Running %s agent with phase-by-phase approval...\n\n", agentName)

	// Execute synchronously with callback
	if err := r.executePhasedAgentSync(r.ctx, agentName, prompt, phaseCallback); err != nil {
		r.output.PrintError("\n❌ Execution failed: %v\n", err)
		return err
	}

	r.output.PrintSuccess("\n✅ All phases completed!\n")
	ShowCompletePedro(r.output.writer)

	return nil
}

// showPhaseSummary shows what happened in a phase
func (r *REPL) showPhaseSummary(phase agents.Phase, result *agents.PhaseResult) {
	r.output.PrintMessage("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	r.output.PrintMessage("📊 Phase: %s\n", phase.Name)
	r.output.PrintMessage("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Show status
	if result.Success {
		r.output.PrintSuccess("✅ Phase completed in %d rounds\n\n", result.RoundsUsed)
	} else {
		r.output.PrintError("❌ Phase failed: %s\n\n", result.Error)
	}

	// Tool call tracking removed - PhaseResult no longer tracks individual tool calls
	// The phase output and structured data contain all relevant information

	// Show FULL LLM reasoning (if plan/analyze phase)
	if phase.Name == "plan" || phase.Name == "analyze" {
		r.output.PrintMessage("💭 LLM Plan:\n\n")
		r.output.PrintMessage("%s\n\n", result.Output)
	}

	// Show JSON data if available (e.g., structured plan)
	if len(result.Data) > 0 {
		r.output.PrintMessage("📦 Structured data: %v\n\n", getKeys(result.Data))
	}
}

// askPhaseAction asks the user what to do after a phase
// Returns: (action, feedback, error)
// action: "c" (continue), "r" (retry), "f" (retry with feedback), "x" (cancel)
// feedback: user's feedback/instructions for retry
func (r *REPL) askPhaseAction() (string, string, error) {
	r.output.PrintMessage("┌─────────────────────────────────────────┐\n")
	r.output.PrintMessage("│ What would you like to do?              │\n")
	r.output.PrintMessage("│  [c] Continue to next phase (default)   │\n")
	r.output.PrintMessage("│  [f] Provide feedback and retry          │\n")
	r.output.PrintMessage("│  [x] Cancel task                        │\n")
	r.output.PrintMessage("└─────────────────────────────────────────┘\n")

	// Temporarily change prompt
	r.input.SetPrompt("[c/f/x]> ")
	line, err := r.input.ReadLine()
	r.input.UpdatePrompt() // Restore normal prompt

	if err != nil {
		return "", "", err
	}

	action := strings.TrimSpace(strings.ToLower(line))

	// If user wants to provide feedback, ask for it
	if action == "f" || action == "feedback" {
		r.output.PrintMessage("\n💬 Enter your feedback/instructions:\n")
		r.input.SetPrompt("> ")
		feedbackLine, err := r.input.ReadLine()
		r.input.UpdatePrompt()

		if err != nil {
			return "", "", err
		}

		feedback := strings.TrimSpace(feedbackLine)
		if feedback == "" {
			r.output.PrintWarning("⚠️  No feedback provided, continuing instead\n")
			return "c", "", nil
		}

		return "f", feedback, nil
	}

	return action, "", nil
}

// getKeys returns the keys from a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
