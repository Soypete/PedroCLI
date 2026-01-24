package repl

import (
	"context"
	"fmt"
	"strings"

	"github.com/soypete/pedrocli/pkg/cli"
)

// InteractiveSession wraps a REPL session with interactive approval
type InteractiveSession struct {
	session *Session
	repl    *REPL
}

// NewInteractiveSession creates a new interactive session
func NewInteractiveSession(session *Session, repl *REPL) *InteractiveSession {
	return &InteractiveSession{
		session: session,
		repl:    repl,
	}
}

// ExecuteWithApproval executes an agent with approval checkpoints
func (is *InteractiveSession) ExecuteWithApproval(ctx context.Context, agentName string, prompt string) error {
	// For now, this is a simplified version
	// TODO: Add proper proposal → approve → apply workflow

	is.repl.output.PrintMessage("\n🔍 Analyzing your request...\n")
	is.repl.output.PrintMessage("   Agent: %s\n", agentName)
	is.repl.output.PrintMessage("   Task: %s\n\n", prompt)

	// Ask for confirmation before starting
	approved := is.askApproval("Start this task?", "[y/n]")
	if !approved {
		is.repl.output.PrintMessage("❌ Task cancelled\n")
		return nil
	}

	// Execute the agent
	result, err := is.session.Bridge.ExecuteAgent(ctx, agentName, prompt)
	if err != nil {
		return err
	}

	// Show result
	if !result.Success {
		is.repl.output.PrintError("❌ Agent failed: %s\n", result.Error)
	} else {
		is.repl.output.PrintSuccess("✅ %s\n", result.Output)
	}

	return nil
}

// askApproval asks the user for approval
func (is *InteractiveSession) askApproval(message, options string) bool {
	is.repl.output.PrintMessage("\n%s %s: ", message, options)

	// Read input
	line, err := is.repl.input.ReadLine()
	if err != nil {
		return false
	}

	response := strings.TrimSpace(strings.ToLower(line))
	return response == "y" || response == "yes"
}

// InteractiveAgentExecutor handles interactive agent execution with approval
type InteractiveAgentExecutor struct {
	bridge *cli.CLIBridge
	output *ProgressOutput
}

// NewInteractiveAgentExecutor creates a new interactive executor
func NewInteractiveAgentExecutor(bridge *cli.CLIBridge, output *ProgressOutput) *InteractiveAgentExecutor {
	return &InteractiveAgentExecutor{
		bridge: bridge,
		output: output,
	}
}

// Execute executes an agent with interactive approval
// This is a placeholder for the full interactive workflow
func (e *InteractiveAgentExecutor) Execute(ctx context.Context, agentName string, description string) error {
	// Phase 1: Analyze (show progress)
	e.output.PrintMessage("\n🔍 Phase 1: Analyze\n")
	e.output.PrintMessage("   Analyzing the request...\n")

	// TODO: Actually run analysis phase

	// Phase 2: Propose (show what will be done, wait for approval)
	e.output.PrintMessage("\n📝 Phase 2: Propose\n")
	e.output.PrintMessage("   Generating implementation plan...\n")

	// TODO: Generate proposal and show it

	proposal := "Example proposal:\n  - Edit main.go to add a print statement\n  - Run tests"
	e.output.PrintMessage("\n%s\n", proposal)

	// TODO: Add actual approval prompt here

	// Phase 3: Apply (only if approved)
	e.output.PrintMessage("\n✍️  Phase 3: Apply\n")
	e.output.PrintMessage("   Executing changes...\n")

	// Execute the actual agent
	result, err := e.bridge.ExecuteAgent(ctx, agentName, description)
	if err != nil {
		return err
	}

	if !result.Success {
		e.output.PrintError("❌ Failed: %s\n", result.Error)
		return fmt.Errorf("agent failed: %s", result.Error)
	}

	e.output.PrintSuccess("✅ %s\n", result.Output)

	// Phase 4: Validate (run tests)
	e.output.PrintMessage("\n🧪 Phase 4: Validate\n")
	e.output.PrintMessage("   Running tests...\n")

	// TODO: Run tests and show results

	return nil
}
