package evals

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Reporter outputs evaluation results in various formats.
type Reporter interface {
	// Report outputs the evaluation run results.
	Report(run *EvalRun) error
}

// JSONReporter outputs results as JSON.
type JSONReporter struct {
	outputPath string
	pretty     bool
}

// NewJSONReporter creates a JSON reporter.
func NewJSONReporter(outputPath string, pretty bool) *JSONReporter {
	return &JSONReporter{
		outputPath: outputPath,
		pretty:     pretty,
	}
}

func (r *JSONReporter) Report(run *EvalRun) error {
	var data []byte
	var err error

	if r.pretty {
		data, err = json.MarshalIndent(run, "", "  ")
	} else {
		data, err = json.Marshal(run)
	}
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	if r.outputPath == "" || r.outputPath == "-" {
		fmt.Println(string(data))
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(r.outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	return os.WriteFile(r.outputPath, data, 0644)
}

// ConsoleReporter outputs results to the console in a human-readable format.
type ConsoleReporter struct {
	verbose bool
	color   bool
}

// NewConsoleReporter creates a console reporter.
func NewConsoleReporter(verbose, color bool) *ConsoleReporter {
	return &ConsoleReporter{
		verbose: verbose,
		color:   color,
	}
}

func (r *ConsoleReporter) Report(run *EvalRun) error {
	// Header
	r.printHeader(run)

	// Summary
	r.printSummary(run.Summary)

	// Per-task results (if verbose)
	if r.verbose {
		r.printTaskResults(run)
	}

	// Grader breakdown
	r.printGraderStats(run.Summary)

	// Pass@k metrics
	r.printPassMetrics(run.Summary)

	return nil
}

func (r *ConsoleReporter) printHeader(run *EvalRun) {
	fmt.Println()
	fmt.Println(r.bold("═══════════════════════════════════════════════════════════════"))
	fmt.Printf(r.bold("  EVALUATION REPORT: %s\n"), run.Suite.Name)
	fmt.Println(r.bold("═══════════════════════════════════════════════════════════════"))
	fmt.Println()
	fmt.Printf("  Run ID:     %s\n", run.ID)
	fmt.Printf("  Model:      %s\n", run.Config.Model)
	fmt.Printf("  Provider:   %s\n", run.Config.Provider)
	fmt.Printf("  Started:    %s\n", run.StartedAt.Format(time.RFC3339))
	fmt.Printf("  Duration:   %s\n", run.CompletedAt.Sub(run.StartedAt).Round(time.Second))
	fmt.Println()
}

func (r *ConsoleReporter) printSummary(summary *RunSummary) {
	fmt.Println(r.bold("  SUMMARY"))
	fmt.Println("  ───────────────────────────────────────────────────────────────")

	passRate := summary.OverallPassRate * 100
	passRateStr := fmt.Sprintf("%.1f%%", passRate)
	if r.color {
		if passRate >= 80 {
			passRateStr = r.green(passRateStr)
		} else if passRate >= 50 {
			passRateStr = r.yellow(passRateStr)
		} else {
			passRateStr = r.red(passRateStr)
		}
	}

	fmt.Printf("  Tasks:       %d\n", summary.TotalTasks)
	fmt.Printf("  Trials:      %d (passed: %s%d%s, failed: %s%d%s, errors: %s%d%s)\n",
		summary.TotalTrials,
		r.green(""), summary.PassedTrials, r.reset(),
		r.red(""), summary.FailedTrials, r.reset(),
		r.yellow(""), summary.ErrorTrials, r.reset())
	fmt.Printf("  Pass Rate:   %s\n", passRateStr)
	fmt.Printf("  Avg Score:   %.2f\n", summary.AvgScore)
	fmt.Printf("  Avg Latency: %s\n", summary.AvgLatency.Round(time.Millisecond))
	fmt.Printf("  Avg Tokens:  %.0f\n", summary.AvgTokensUsed)
	fmt.Println()
}

func (r *ConsoleReporter) printTaskResults(run *EvalRun) {
	fmt.Println(r.bold("  TASK RESULTS"))
	fmt.Println("  ───────────────────────────────────────────────────────────────")

	// Group trials by task
	trialsByTask := make(map[string][]*Trial)
	for _, trial := range run.Trials {
		trialsByTask[trial.TaskID] = append(trialsByTask[trial.TaskID], trial)
	}

	// Sort task IDs
	taskIDs := make([]string, 0, len(trialsByTask))
	for id := range trialsByTask {
		taskIDs = append(taskIDs, id)
	}
	sort.Strings(taskIDs)

	for _, taskID := range taskIDs {
		trials := trialsByTask[taskID]
		passed := 0
		var totalScore float64
		for _, trial := range trials {
			if trial.Passed {
				passed++
			}
			totalScore += trial.Score
		}
		avgScore := totalScore / float64(len(trials))

		status := r.green("✓")
		if passed == 0 {
			status = r.red("✗")
		} else if passed < len(trials) {
			status = r.yellow("~")
		}

		fmt.Printf("  %s %-30s  %d/%d passed  avg: %.2f\n",
			status, truncate(taskID, 30), passed, len(trials), avgScore)

		if r.verbose {
			for _, trial := range trials {
				trialStatus := r.green("✓")
				if !trial.Passed {
					trialStatus = r.red("✗")
				}
				fmt.Printf("      Trial %d: %s  score: %.2f  latency: %s\n",
					trial.TrialNumber, trialStatus, trial.Score,
					trial.Metrics.TotalLatency.Round(time.Millisecond))

				// Show grader results
				for _, gr := range trial.GradeResults {
					grStatus := r.green("✓")
					if !gr.Passed {
						grStatus = r.red("✗")
					}
					fmt.Printf("        %s %s: %s\n",
						grStatus, gr.GraderType, truncate(gr.Feedback, 50))
				}
			}
		}
	}
	fmt.Println()
}

func (r *ConsoleReporter) printGraderStats(summary *RunSummary) {
	if len(summary.ByGraderType) == 0 {
		return
	}

	fmt.Println(r.bold("  GRADER BREAKDOWN"))
	fmt.Println("  ───────────────────────────────────────────────────────────────")
	fmt.Println("  Grader Type        Runs   Passed   Pass Rate   Avg Score")
	fmt.Println("  ─────────────────────────────────────────────────────────────")

	// Sort grader types
	types := make([]GraderType, 0, len(summary.ByGraderType))
	for t := range summary.ByGraderType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool {
		return string(types[i]) < string(types[j])
	})

	for _, t := range types {
		stats := summary.ByGraderType[t]
		passRateStr := fmt.Sprintf("%.1f%%", stats.PassRate*100)
		if r.color {
			if stats.PassRate >= 0.8 {
				passRateStr = r.green(passRateStr)
			} else if stats.PassRate >= 0.5 {
				passRateStr = r.yellow(passRateStr)
			} else {
				passRateStr = r.red(passRateStr)
			}
		}
		fmt.Printf("  %-18s %5d %8d %10s %11.2f\n",
			t, stats.TotalRuns, stats.Passed, passRateStr, stats.AvgScore)
	}
	fmt.Println()
}

func (r *ConsoleReporter) printPassMetrics(summary *RunSummary) {
	if len(summary.PassAtK) == 0 {
		return
	}

	fmt.Println(r.bold("  PASS METRICS"))
	fmt.Println("  ───────────────────────────────────────────────────────────────")

	// Sort k values
	kValues := make([]int, 0, len(summary.PassAtK))
	for k := range summary.PassAtK {
		kValues = append(kValues, k)
	}
	sort.Ints(kValues)

	for _, k := range kValues {
		passAtK := summary.PassAtK[k]
		passPowerK := summary.PassPowerK[k]
		fmt.Printf("  pass@%d:  %.1f%%    pass^%d:  %.1f%%\n",
			k, passAtK*100, k, passPowerK*100)
	}
	fmt.Println()
}

// Color helpers
func (r *ConsoleReporter) bold(s string) string {
	if r.color {
		return "\033[1m" + s + "\033[0m"
	}
	return s
}

func (r *ConsoleReporter) green(s string) string {
	if r.color {
		return "\033[32m" + s + "\033[0m"
	}
	return s
}

func (r *ConsoleReporter) red(s string) string {
	if r.color {
		return "\033[31m" + s + "\033[0m"
	}
	return s
}

func (r *ConsoleReporter) yellow(s string) string {
	if r.color {
		return "\033[33m" + s + "\033[0m"
	}
	return s
}

func (r *ConsoleReporter) reset() string {
	if r.color {
		return "\033[0m"
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// HTMLReporter outputs results as an HTML report.
type HTMLReporter struct {
	outputPath string
}

// NewHTMLReporter creates an HTML reporter.
func NewHTMLReporter(outputPath string) *HTMLReporter {
	return &HTMLReporter{outputPath: outputPath}
}

func (r *HTMLReporter) Report(run *EvalRun) error {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"percent":  func(f float64) string { return fmt.Sprintf("%.1f%%", f*100) },
		"score":    func(f float64) string { return fmt.Sprintf("%.2f", f) },
		"duration": func(d time.Duration) string { return d.Round(time.Millisecond).String() },
		"passClass": func(passed bool) string {
			if passed {
				return "pass"
			}
			return "fail"
		},
		"rateClass": func(rate float64) string {
			if rate >= 0.8 {
				return "good"
			} else if rate >= 0.5 {
				return "warn"
			}
			return "bad"
		},
	}).Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	// Prepare data
	data := struct {
		Run          *EvalRun
		TrialsByTask map[string][]*Trial
		TaskIDs      []string
		GraderTypes  []GraderType
	}{
		Run:          run,
		TrialsByTask: make(map[string][]*Trial),
	}

	for _, trial := range run.Trials {
		data.TrialsByTask[trial.TaskID] = append(data.TrialsByTask[trial.TaskID], trial)
	}

	for id := range data.TrialsByTask {
		data.TaskIDs = append(data.TaskIDs, id)
	}
	sort.Strings(data.TaskIDs)

	for t := range run.Summary.ByGraderType {
		data.GraderTypes = append(data.GraderTypes, t)
	}
	sort.Slice(data.GraderTypes, func(i, j int) bool {
		return string(data.GraderTypes[i]) < string(data.GraderTypes[j])
	})

	// Render
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	if r.outputPath == "" || r.outputPath == "-" {
		fmt.Println(buf.String())
		return nil
	}

	dir := filepath.Dir(r.outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	return os.WriteFile(r.outputPath, []byte(buf.String()), 0644)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Evaluation Report - {{.Run.Suite.Name}}</title>
    <style>
        :root {
            --bg: #1a1a2e;
            --surface: #16213e;
            --border: #0f3460;
            --text: #eaeaea;
            --text-dim: #94a3b8;
            --good: #10b981;
            --warn: #f59e0b;
            --bad: #ef4444;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
            padding: 2rem;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        h1, h2, h3 { margin-bottom: 1rem; }
        h1 { font-size: 2rem; border-bottom: 2px solid var(--border); padding-bottom: 0.5rem; }
        h2 { font-size: 1.5rem; margin-top: 2rem; color: var(--text-dim); }
        .meta { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin: 1rem 0; }
        .meta-item { background: var(--surface); padding: 1rem; border-radius: 8px; }
        .meta-item label { color: var(--text-dim); font-size: 0.875rem; display: block; }
        .meta-item value { font-size: 1.25rem; font-weight: 600; }
        .summary-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 1rem; margin: 1rem 0; }
        .stat-card { background: var(--surface); padding: 1.5rem; border-radius: 8px; text-align: center; }
        .stat-card .value { font-size: 2rem; font-weight: 700; }
        .stat-card .label { color: var(--text-dim); font-size: 0.875rem; }
        .stat-card.good .value { color: var(--good); }
        .stat-card.warn .value { color: var(--warn); }
        .stat-card.bad .value { color: var(--bad); }
        table { width: 100%; border-collapse: collapse; margin: 1rem 0; }
        th, td { padding: 0.75rem 1rem; text-align: left; border-bottom: 1px solid var(--border); }
        th { background: var(--surface); color: var(--text-dim); font-weight: 500; }
        tr:hover { background: rgba(255,255,255,0.02); }
        .pass { color: var(--good); }
        .fail { color: var(--bad); }
        .badge { display: inline-block; padding: 0.25rem 0.5rem; border-radius: 4px; font-size: 0.75rem; font-weight: 500; }
        .badge.good { background: rgba(16, 185, 129, 0.2); color: var(--good); }
        .badge.warn { background: rgba(245, 158, 11, 0.2); color: var(--warn); }
        .badge.bad { background: rgba(239, 68, 68, 0.2); color: var(--bad); }
        .task-row { cursor: pointer; }
        .task-row:hover { background: rgba(255,255,255,0.05); }
        .trial-details { display: none; background: var(--surface); }
        .trial-details.open { display: table-row; }
        .trial-details td { padding: 1rem; }
        .grader-result { margin: 0.5rem 0; padding: 0.5rem; background: rgba(0,0,0,0.2); border-radius: 4px; }
        .progress-bar { height: 8px; background: var(--border); border-radius: 4px; overflow: hidden; }
        .progress-bar .fill { height: 100%; transition: width 0.3s; }
        .progress-bar .fill.good { background: var(--good); }
        .progress-bar .fill.warn { background: var(--warn); }
        .progress-bar .fill.bad { background: var(--bad); }
    </style>
</head>
<body>
    <div class="container">
        <h1>📊 Evaluation Report: {{.Run.Suite.Name}}</h1>

        <div class="meta">
            <div class="meta-item">
                <label>Run ID</label>
                <value>{{.Run.ID}}</value>
            </div>
            <div class="meta-item">
                <label>Model</label>
                <value>{{.Run.Config.Model}}</value>
            </div>
            <div class="meta-item">
                <label>Provider</label>
                <value>{{.Run.Config.Provider}}</value>
            </div>
            <div class="meta-item">
                <label>Duration</label>
                <value>{{duration .Run.CompletedAt.Sub .Run.StartedAt}}</value>
            </div>
        </div>

        <h2>Summary</h2>
        <div class="summary-grid">
            <div class="stat-card {{rateClass .Run.Summary.OverallPassRate}}">
                <div class="value">{{percent .Run.Summary.OverallPassRate}}</div>
                <div class="label">Pass Rate</div>
            </div>
            <div class="stat-card">
                <div class="value">{{.Run.Summary.TotalTasks}}</div>
                <div class="label">Tasks</div>
            </div>
            <div class="stat-card">
                <div class="value">{{.Run.Summary.TotalTrials}}</div>
                <div class="label">Trials</div>
            </div>
            <div class="stat-card good">
                <div class="value">{{.Run.Summary.PassedTrials}}</div>
                <div class="label">Passed</div>
            </div>
            <div class="stat-card bad">
                <div class="value">{{.Run.Summary.FailedTrials}}</div>
                <div class="label">Failed</div>
            </div>
            <div class="stat-card">
                <div class="value">{{score .Run.Summary.AvgScore}}</div>
                <div class="label">Avg Score</div>
            </div>
        </div>

        {{if .Run.Summary.PassAtK}}
        <h2>Pass Metrics</h2>
        <table>
            <thead>
                <tr>
                    <th>Metric</th>
                    <th>Value</th>
                    <th>Description</th>
                </tr>
            </thead>
            <tbody>
                {{range $k, $v := .Run.Summary.PassAtK}}
                <tr>
                    <td>pass@{{$k}}</td>
                    <td><span class="badge {{if ge $v 0.8}}good{{else if ge $v 0.5}}warn{{else}}bad{{end}}">{{percent $v}}</span></td>
                    <td>Probability of at least 1 success in {{$k}} trials</td>
                </tr>
                {{end}}
                {{range $k, $v := .Run.Summary.PassPowerK}}
                <tr>
                    <td>pass^{{$k}}</td>
                    <td><span class="badge {{if ge $v 0.8}}good{{else if ge $v 0.5}}warn{{else}}bad{{end}}">{{percent $v}}</span></td>
                    <td>Probability of all {{$k}} trials succeeding</td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{end}}

        <h2>Grader Performance</h2>
        <table>
            <thead>
                <tr>
                    <th>Grader Type</th>
                    <th>Runs</th>
                    <th>Passed</th>
                    <th>Pass Rate</th>
                    <th>Avg Score</th>
                </tr>
            </thead>
            <tbody>
                {{range .GraderTypes}}
                {{$stats := index $.Run.Summary.ByGraderType .}}
                <tr>
                    <td>{{.}}</td>
                    <td>{{$stats.TotalRuns}}</td>
                    <td>{{$stats.Passed}}</td>
                    <td>
                        <div class="progress-bar" style="width: 100px; display: inline-block; vertical-align: middle;">
                            <div class="fill {{rateClass $stats.PassRate}}" style="width: {{percent $stats.PassRate}};"></div>
                        </div>
                        <span style="margin-left: 0.5rem;">{{percent $stats.PassRate}}</span>
                    </td>
                    <td>{{score $stats.AvgScore}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>

        <h2>Task Results</h2>
        <table>
            <thead>
                <tr>
                    <th>Task ID</th>
                    <th>Trials</th>
                    <th>Passed</th>
                    <th>Pass Rate</th>
                    <th>Avg Score</th>
                    <th>Avg Latency</th>
                </tr>
            </thead>
            <tbody>
                {{range .TaskIDs}}
                {{$trials := index $.TrialsByTask .}}
                {{$passed := 0}}
                {{$totalScore := 0.0}}
                {{$totalLatency := 0}}
                {{range $trials}}
                    {{if .Passed}}{{$passed = (add $passed 1)}}{{end}}
                    {{$totalScore = (addf $totalScore .Score)}}
                {{end}}
                <tr class="task-row" onclick="toggleDetails('{{.}}')">
                    <td>{{.}}</td>
                    <td>{{len $trials}}</td>
                    <td><span class="{{if eq $passed (len $trials)}}pass{{else if gt $passed 0}}warn{{else}}fail{{end}}">{{$passed}}</span></td>
                    <td>
                        {{$rate := (divf (float $passed) (float (len $trials)))}}
                        <span class="badge {{rateClass $rate}}">{{percent $rate}}</span>
                    </td>
                    <td>{{score (divf $totalScore (float (len $trials)))}}</td>
                    <td>-</td>
                </tr>
                <tr class="trial-details" id="details-{{.}}">
                    <td colspan="6">
                        {{range $trials}}
                        <div class="grader-result">
                            <strong>Trial {{.TrialNumber}}</strong>
                            <span class="badge {{passClass .Passed}}">{{if .Passed}}PASS{{else}}FAIL{{end}}</span>
                            Score: {{score .Score}}
                            {{range .GradeResults}}
                            <div style="margin-left: 1rem; font-size: 0.875rem; color: var(--text-dim);">
                                <span class="{{passClass .Passed}}">{{if .Passed}}✓{{else}}✗{{end}}</span>
                                {{.GraderType}}: {{.Feedback}}
                            </div>
                            {{end}}
                        </div>
                        {{end}}
                    </td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>

    <script>
        function toggleDetails(taskId) {
            const row = document.getElementById('details-' + taskId);
            row.classList.toggle('open');
        }
        // Helper functions for template
        function add(a, b) { return a + b; }
        function addf(a, b) { return a + b; }
        function divf(a, b) { return b === 0 ? 0 : a / b; }
    </script>
</body>
</html>`

// MultiReporter runs multiple reporters.
type MultiReporter struct {
	reporters []Reporter
}

// NewMultiReporter creates a reporter that outputs to multiple formats.
func NewMultiReporter(reporters ...Reporter) *MultiReporter {
	return &MultiReporter{reporters: reporters}
}

func (r *MultiReporter) Report(run *EvalRun) error {
	var errs []string
	for _, reporter := range r.reporters {
		if err := reporter.Report(run); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("reporter errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ReportComparison outputs a model comparison report.
func ReportComparison(result *ComparisonResult, format string, outputPath string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		if outputPath == "" {
			fmt.Println(string(data))
			return nil
		}
		return os.WriteFile(outputPath, data, 0644)

	case "console":
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("  MODEL COMPARISON REPORT")
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println()
		fmt.Printf("  Model 1: %s\n", result.Model1)
		fmt.Printf("  Model 2: %s\n", result.Model2)
		fmt.Println()
		fmt.Printf("  Model 1 Wins: %d\n", result.Model1Wins)
		fmt.Printf("  Model 2 Wins: %d\n", result.Model2Wins)
		fmt.Printf("  Ties:         %d\n", result.Ties)
		fmt.Println()
		if result.SignificanceP < 0.05 {
			winner := result.Model1
			if result.Model2Wins > result.Model1Wins {
				winner = result.Model2
			}
			fmt.Printf("  Result: %s is significantly better (p=%.4f)\n", winner, result.SignificanceP)
		} else {
			fmt.Printf("  Result: No significant difference (p=%.4f)\n", result.SignificanceP)
		}
		fmt.Println()
		return nil

	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}
