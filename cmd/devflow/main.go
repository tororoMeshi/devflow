package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tororoMeshi/devflow/internal/command"
)

const usage = `Usage:
  devflow init
  devflow list
  devflow start <flow-id> --task-file <path>
  devflow status
  devflow prompt
  devflow context
	devflow completion-context --step <step-id> --attempt <attempt-id>
  devflow work-package --step <step-id> --attempt <attempt-id>
  devflow approve --step <step-id> --attempt <attempt-id> --note <note>
  devflow artifact record --step <step-id> --attempt <attempt-id> --path <project-relative-path>
  devflow done
  devflow back [--to <step>] --reason <reason>
  devflow skip --reason <reason>
  devflow finish --reason <reason>
  devflow check request --step <step-id> --attempt <attempt-id> --check <check-id>
  devflow check record --file <result.json>
  devflow execution-report record --file <report.json>
`

func main() {
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(run(os.Args[1:], projectRoot, os.Stdout, os.Stderr))
}

func run(args []string, projectRoot string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		writeUsage(stderr)
		return 1
	}

	ctx := command.Context{
		ProjectRoot: projectRoot,
		Stdout:      stdout,
		Stderr:      stderr,
	}

	var result command.CommandResult
	switch args[0] {
	case "init":
		if len(args) != 1 {
			writeUsage(stderr)
			return 1
		}
		result = command.Init(ctx)
	case "list":
		if len(args) != 1 {
			writeUsage(stderr)
			return 1
		}
		result = command.List(ctx)
	case "start":
		flowID, taskPath, ok := parseStartArgs(args[1:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.Start(ctx, flowID, taskPath)
	case "status":
		if len(args) != 1 {
			writeUsage(stderr)
			return 1
		}
		result = command.Status(ctx)
	case "prompt":
		if len(args) != 1 {
			writeUsage(stderr)
			return 1
		}
		result = command.Prompt(ctx)
	case "context":
		if len(args) != 1 {
			writeUsage(stderr)
			return 1
		}
		result = command.CurrentContext(ctx)
	case "completion-context":
		stepID, attemptID, ok := parseCompletionContextArgs(args[1:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.CompletionContext(ctx, stepID, attemptID)
	case "work-package":
		stepID, attemptID, ok := parseWorkPackageArgs(args[1:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.WorkPackage(ctx, stepID, attemptID)
	case "approve":
		stepID, attemptID, note, ok := parseApproveArgs(args[1:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.Approve(ctx, stepID, attemptID, note)
	case "artifact":
		if len(args) < 2 || args[1] != "record" {
			writeUsage(stderr)
			return 1
		}
		stepID, attemptID, path, ok := parseArtifactRecordArgs(args[2:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.RecordArtifact(ctx, stepID, attemptID, path)
	case "done":
		if len(args) != 1 {
			writeUsage(stderr)
			return 1
		}
		result = command.Done(ctx)
	case "back":
		toStepID, reason, ok := parseBackArgs(args[1:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.Back(ctx, toStepID, reason)
	case "skip":
		reason, ok := parseReasonArgs(args[1:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.Skip(ctx, reason)
	case "finish":
		reason, ok := parseReasonArgs(args[1:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.Finish(ctx, reason)
	case "check":
		if len(args) >= 2 && args[1] == "request" {
			stepID, attemptID, checkID, ok := parseCheckRequestArgs(args[2:])
			if !ok {
				writeUsage(stderr)
				return 1
			}
			result = command.CheckRequest(ctx, stepID, attemptID, checkID)
		} else if len(args) >= 2 && args[1] == "record" {
			path, ok := parseCheckRecordArgs(args[2:])
			if !ok {
				writeUsage(stderr)
				return 1
			}
			result = command.CheckRecord(ctx, path)
		} else {
			writeUsage(stderr)
			return 1
		}
	case "execution-report":
		if len(args) < 2 || args[1] != "record" {
			writeUsage(stderr)
			return 1
		}
		path, ok := parseExecutionReportRecordArgs(args[2:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.ExecutionReportRecord(ctx, path)
	default:
		writeUsage(stderr)
		return 1
	}

	if err := writeResult(ctx, result); err != nil {
		return 1
	}
	return result.ExitCode
}

func parseStartArgs(args []string) (string, string, bool) {
	if len(args) != 3 || args[1] != "--task-file" || args[0] == "" || strings.TrimSpace(args[2]) == "" {
		return "", "", false
	}
	if strings.HasPrefix(args[0], "-") {
		return "", "", false
	}
	return args[0], args[2], true
}

func parseApproveArgs(args []string) (string, string, string, bool) {
	if len(args) != 6 || args[0] != "--step" || args[2] != "--attempt" || args[4] != "--note" {
		return "", "", "", false
	}
	if strings.TrimSpace(args[1]) == "" || strings.TrimSpace(args[3]) == "" || strings.TrimSpace(args[5]) == "" {
		return "", "", "", false
	}
	return args[1], args[3], args[5], true
}

func parseArtifactRecordArgs(args []string) (string, string, string, bool) {
	if len(args) != 6 {
		return "", "", "", false
	}
	values := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		option := args[i]
		if option != "--step" && option != "--attempt" && option != "--path" {
			return "", "", "", false
		}
		if _, duplicate := values[option]; duplicate || strings.TrimSpace(args[i+1]) == "" {
			return "", "", "", false
		}
		values[option] = args[i+1]
	}
	return values["--step"], values["--attempt"], values["--path"],
		values["--step"] != "" && values["--attempt"] != "" && values["--path"] != ""
}

func parseReasonArgs(args []string) (string, bool) {
	if len(args) != 2 || args[0] != "--reason" {
		return "", false
	}
	return args[1], true
}

func parseCheckRequestArgs(args []string) (string, string, string, bool) {
	values, ok := parseExactOptions(args, "--step", "--attempt", "--check")
	if !ok {
		return "", "", "", false
	}
	return values["--step"], values["--attempt"], values["--check"], true
}

func parseWorkPackageArgs(args []string) (string, string, bool) {
	values, ok := parseExactOptions(args, "--step", "--attempt")
	if !ok {
		return "", "", false
	}
	return values["--step"], values["--attempt"], true
}

func parseCompletionContextArgs(args []string) (string, string, bool) {
	return parseWorkPackageArgs(args)
}

func parseCheckRecordArgs(args []string) (string, bool) {
	values, ok := parseExactOptions(args, "--file")
	if !ok {
		return "", false
	}
	return values["--file"], true
}

func parseExecutionReportRecordArgs(args []string) (string, bool) {
	values, ok := parseExactOptions(args, "--file")
	if !ok {
		return "", false
	}
	return values["--file"], true
}

func parseExactOptions(args []string, allowed ...string) (map[string]string, bool) {
	if len(args) != len(allowed)*2 {
		return nil, false
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, option := range allowed {
		allowedSet[option] = struct{}{}
	}
	values := make(map[string]string, len(allowed))
	for i := 0; i < len(args); i += 2 {
		option, value := args[i], args[i+1]
		if _, exists := allowedSet[option]; !exists || strings.Contains(option, "=") {
			return nil, false
		}
		if _, duplicate := values[option]; duplicate || strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return nil, false
		}
		values[option] = value
	}
	for _, option := range allowed {
		if _, exists := values[option]; !exists {
			return nil, false
		}
	}
	return values, true
}

func parseBackArgs(args []string) (string, string, bool) {
	var toStepID string
	var reason string
	hasTo := false
	hasReason := false
	for i := 0; i < len(args); i++ {
		if i+1 >= len(args) {
			return "", "", false
		}
		switch args[i] {
		case "--to":
			if hasTo || args[i+1] == "" {
				return "", "", false
			}
			toStepID = args[i+1]
			hasTo = true
		case "--reason":
			if hasReason {
				return "", "", false
			}
			reason = args[i+1]
			hasReason = true
		default:
			return "", "", false
		}
		i++
	}
	return toStepID, reason, hasReason
}

func writeUsage(stderr io.Writer) {
	_, _ = io.WriteString(stderr, usage)
}

func writeResult(ctx command.Context, result command.CommandResult) error {
	writeActions(ctx.Stdout, result.Actions)
	writeFlows(ctx.Stdout, result.Flows)
	if result.Success != nil {
		writeSuccess(ctx.Stdout, *result.Success)
	}
	if result.Status != nil {
		writeStatus(ctx.Stdout, *result.Status)
	}
	if result.Prompt != nil {
		writePrompt(ctx.Stdout, *result.Prompt)
	}
	if result.ExecutionContext != nil {
		_ = json.NewEncoder(ctx.Stdout).Encode(result.ExecutionContext)
	}
	if result.CompletionContext != nil {
		if err := json.NewEncoder(ctx.Stdout).Encode(result.CompletionContext); err != nil {
			return err
		}
	}
	if result.CheckRequest != nil {
		_ = json.NewEncoder(ctx.Stdout).Encode(result.CheckRequest)
	}
	if result.WorkPackage != nil {
		if err := json.NewEncoder(ctx.Stdout).Encode(result.WorkPackage); err != nil {
			return err
		}
	}
	if result.ExecutionReport != nil {
		writeExecutionReport(ctx.Stdout, *result.ExecutionReport)
	}
	command.WriteDiagnostics(ctx, result.Diagnostics)
	return nil
}

func writeExecutionReport(stdout io.Writer, result command.ExecutionReportRecordResult) {
	_, _ = fmt.Fprintln(stdout, "Recorded execution report")
	_, _ = fmt.Fprintf(stdout, "Run: %s\n", result.FlowRunID)
	_, _ = fmt.Fprintf(stdout, "Step: %s\n", result.StepID)
	_, _ = fmt.Fprintf(stdout, "Attempt: %s\n", result.AttemptID)
	_, _ = fmt.Fprintf(stdout, "Work package: %s\n", result.WorkPackageDigest)
	_, _ = fmt.Fprintf(stdout, "Execution report: %s\n", result.ExecutionReportDigest)
	_, _ = fmt.Fprintf(stdout, "Outcome: %s\n", result.Outcome)
	_, _ = fmt.Fprintf(stdout, "Idempotent: %t\n", result.Idempotent)
}

func writeSuccess(stdout io.Writer, success command.SuccessResult) {
	if success.StartedFlowID != "" {
		_, _ = fmt.Fprintf(stdout, "Started flow: %s\n", success.StartedFlowID)
	}
	if success.CurrentStepID != "" {
		_, _ = fmt.Fprintf(stdout, "Current step: %s\n", success.CurrentStepID)
	}
	if success.CompletedStepID != "" {
		_, _ = fmt.Fprintf(stdout, "Completed step: %s\n", success.CompletedStepID)
	}
	if success.ApprovedStepID != "" {
		_, _ = fmt.Fprintf(stdout, "Approved step: %s\n", success.ApprovedStepID)
	}
	if success.ApprovedAttemptID != "" {
		_, _ = fmt.Fprintf(stdout, "Approved attempt: %s\n", success.ApprovedAttemptID)
	}
	if success.ApprovedEvidenceSetDigest != "" {
		_, _ = fmt.Fprintf(stdout, "Evidence set: %s\n", success.ApprovedEvidenceSetDigest)
	}
	if success.RecordedArtifactPath != "" {
		_, _ = fmt.Fprintf(stdout, "Recorded artifact: %s\n", success.RecordedArtifactPath)
		_, _ = fmt.Fprintf(stdout, "Attempt: %s\n", success.RecordedAttemptID)
		_, _ = fmt.Fprintf(stdout, "Digest: %s\n", success.RecordedArtifactDigest)
		_, _ = fmt.Fprintf(stdout, "Size: %d\n", success.RecordedArtifactSize)
	}
	if success.RecordedCheckID != "" {
		_, _ = fmt.Fprintf(stdout, "Recorded check: %s\n", success.RecordedCheckID)
		_, _ = fmt.Fprintf(stdout, "Run: %s\n", success.RecordedCheckRunID)
		_, _ = fmt.Fprintf(stdout, "Step: %s\n", success.RecordedCheckStepID)
		_, _ = fmt.Fprintf(stdout, "Attempt: %s\n", success.RecordedCheckAttemptID)
		_, _ = fmt.Fprintf(stdout, "Exit code: %d\n", *success.RecordedCheckExitCode)
	}
	if success.MovedBackToID != "" {
		_, _ = fmt.Fprintf(stdout, "Moved back to: %s\n", success.MovedBackToID)
	}
	if success.SkippedStepID != "" {
		_, _ = fmt.Fprintf(stdout, "Skipped step: %s\n", success.SkippedStepID)
	}
	if success.NextStepID != "" {
		_, _ = fmt.Fprintf(stdout, "Next step: %s\n", success.NextStepID)
	}
	if success.CompletedFlowID != "" {
		_, _ = fmt.Fprintf(stdout, "Flow completed: %s\n", success.CompletedFlowID)
	}
	if success.FinishedFlowID != "" {
		_, _ = fmt.Fprintf(stdout, "Finished flow: %s\n", success.FinishedFlowID)
	}
}

func writeActions(stdout io.Writer, actions []command.CommandAction) {
	for _, action := range actions {
		_, _ = fmt.Fprintf(stdout, "%s %s\n", action.Status, action.Path)
	}
}

func writeFlows(stdout io.Writer, flows []command.FlowListItem) {
	for _, flow := range flows {
		if flow.Status == command.FlowStatusInvalid {
			_, _ = fmt.Fprintf(stdout, "file: %s\nstatus: %s\n", flow.FilePath, flow.Status)
			if flow.Err != nil {
				_, _ = fmt.Fprintf(stdout, "error: %v\n", flow.Err)
			}
			_, _ = fmt.Fprintln(stdout)
			continue
		}
		_, _ = fmt.Fprintf(stdout, "id: %s\n", flow.ID)
		_, _ = fmt.Fprintf(stdout, "title: %s\n", flow.Title)
		_, _ = fmt.Fprintf(stdout, "description: %s\n", flow.Description)
		_, _ = fmt.Fprintf(stdout, "steps: %d\n", flow.StepCount)
		_, _ = fmt.Fprintf(stdout, "status: %s\n\n", flow.Status)
	}
}

func writeStatus(stdout io.Writer, status command.StatusResult) {
	_, _ = fmt.Fprintf(stdout, "Flow: %s\n", status.FlowTitle)
	if status.FlowStatus == "completed" || status.FlowStatus == "finished" {
		_, _ = fmt.Fprintf(stdout, "Flow status: %s\n", status.FlowStatus)
		_, _ = fmt.Fprintln(stdout, "This Flow is no longer active. No devflow transition is needed.")
		return
	}

	_, _ = fmt.Fprintf(stdout, "Current step: %s\n", status.CurrentStepTitle)
	_, _ = fmt.Fprintf(stdout, "Objective: %s\n\n", status.CurrentStepObjective)
	_, _ = fmt.Fprintln(stdout, "Completion requirements checked by devflow:")
	if !hasCompletionRequirements(status) {
		_, _ = fmt.Fprintln(stdout, "- None are declared for this Step.")
		_, _ = fmt.Fprintln(stdout, "- Complete the Objective outside devflow. When you judge it is complete, request the transition:")
		_, _ = fmt.Fprintln(stdout, "  devflow done")
		_, _ = fmt.Fprintln(stdout, "- `done` does not perform the Objective; it only checks declared requirements and advances the Flow.")
	} else {
		writeHumanArtifactStatus(stdout, status)
		writeHumanCheckStatus(stdout, status.Checks)
		writeHumanApprovalStatus(stdout, status)
		if status.CompletionReady {
			_, _ = fmt.Fprintln(stdout, "All declared completion requirements are satisfied. Request the transition:")
			_, _ = fmt.Fprintln(stdout, "  devflow done")
		} else {
			_, _ = fmt.Fprintln(stdout, "Resolve the unmet requirements above before requesting the transition with `devflow done`.")
		}
	}
}

func hasCompletionRequirements(status command.StatusResult) bool {
	return len(status.Artifacts) > 0 || len(status.Checks) > 0 || status.Approval != nil
}

func writeHumanArtifactStatus(stdout io.Writer, status command.StatusResult) {
	if len(status.Artifacts) == 0 {
		return
	}
	_, _ = fmt.Fprintln(stdout, "Required artifacts:")
	for _, artifact := range status.Artifacts {
		switch artifact.State {
		case command.ArtifactStatusCurrent:
			_, _ = fmt.Fprintf(stdout, "- %s exists and devflow's saved file record matches the current file.\n", artifact.Path)
		case command.ArtifactStatusMissingEvidence:
			if artifact.Exists {
				_, _ = fmt.Fprintf(stdout, "- %s exists, but devflow has no saved file record for it. Record it:\n", artifact.Path)
			} else {
				_, _ = fmt.Fprintf(stdout, "- %s does not exist, and devflow has no saved file record for it. Create it, then record it:\n", artifact.Path)
			}
			writeArtifactRecordCommand(stdout, status, artifact.Path)
		case command.ArtifactStatusMissingFile:
			_, _ = fmt.Fprintf(stdout, "- %s was recorded before but is no longer present. Restore the recorded version to satisfy this Attempt.\n", artifact.Path)
			_, _ = fmt.Fprintln(stdout, "  If it must be recreated with different contents, return to the previous Step to create a new Attempt:")
			writeBackCommand(stdout)
		case command.ArtifactStatusChanged:
			_, _ = fmt.Fprintf(stdout, "- %s changed after devflow saved its file record. That record cannot be replaced while this Step is in progress.\n", artifact.Path)
			_, _ = fmt.Fprintln(stdout, "  Return to the previous Step, then record the current file when this Step is entered again:")
			writeBackCommand(stdout)
		default:
			_, _ = fmt.Fprintf(stdout, "- %s cannot be inspected safely. Resolve the file access problem before recording evidence.\n", artifact.Path)
		}
	}
}

func writeArtifactRecordCommand(stdout io.Writer, status command.StatusResult, path string) {
	_, _ = fmt.Fprintf(stdout, "  devflow artifact record --step %s --attempt %s --path %s\n", status.CurrentStepID, status.CurrentAttemptID, path)
}

func writeBackCommand(stdout io.Writer) {
	_, _ = fmt.Fprintln(stdout, "  devflow back --reason \"Artifact changed after evidence was recorded\"")
}

func writeHumanCheckStatus(stdout io.Writer, checks []command.CheckStatusResult) {
	if len(checks) == 0 {
		return
	}
	_, _ = fmt.Fprintln(stdout, "Required checks:")
	for _, check := range checks {
		if check.ExitCode == nil {
			_, _ = fmt.Fprintf(stdout, "- %s has not been recorded.\n", check.CheckID)
			continue
		}
		if check.Status == "passed" {
			_, _ = fmt.Fprintf(stdout, "- %s passed.\n", check.CheckID)
			continue
		}
		_, _ = fmt.Fprintf(stdout, "- %s failed (exit code %d). Record a passing result before continuing.\n", check.CheckID, *check.ExitCode)
	}
}

func writeHumanApprovalStatus(stdout io.Writer, status command.StatusResult) {
	if status.Approval == nil {
		return
	}
	_, _ = fmt.Fprintln(stdout, "Human approval:")
	if status.Approval.Approved {
		_, _ = fmt.Fprintf(stdout, "- Approved for this Step. Note: %s\n", status.Approval.Note)
		return
	}
	_, _ = fmt.Fprintf(stdout, "- Waiting for approval of this Step's Objective: %s\n", status.CurrentStepObjective)
	_, _ = fmt.Fprintln(stdout, "  Record the approval with:")
	_, _ = fmt.Fprintf(stdout, "  devflow approve --step %s --attempt %s --note \"<approval note>\"\n", status.Approval.StepID, status.CurrentAttemptID)
}

func writePrompt(stdout io.Writer, prompt command.PromptResult) {
	_, _ = fmt.Fprintf(stdout, "Task:\n%s", prompt.TaskContent)
	if strings.HasSuffix(prompt.TaskContent, "\n") {
		_, _ = io.WriteString(stdout, "\n")
	} else {
		_, _ = io.WriteString(stdout, "\n\n")
	}
	_, _ = fmt.Fprintf(stdout, "Current step: %s (%s)\n", prompt.CurrentStepTitle, prompt.CurrentStepID)
	_, _ = fmt.Fprintf(stdout, "Objective:\n%s\n", prompt.CurrentStepObjective)
	writeArtifactList(stdout, "Required inputs", prompt.RequiredInputs)
	writeArtifactList(stdout, "Required artifacts", prompt.RequiredArtifacts)
	_, _ = fmt.Fprintln(stdout, "Approval:")
	if prompt.RequiredApproval == nil {
		_, _ = fmt.Fprintln(stdout, "- not required")
	} else {
		_, _ = fmt.Fprintln(stdout, "- required from a human or external operation")
	}
	writeStringList(stdout, "Required checks", prompt.RequiredChecks)
	if len(prompt.ArtifactBlockers) > 0 {
		writeStringList(stdout, "Artifact blockers", prompt.ArtifactBlockers)
	}
	if len(prompt.CheckBlockers) > 0 {
		writeStringList(stdout, "Check blockers", prompt.CheckBlockers)
	}
	if len(prompt.CompletionBlockers) > 0 {
		writeStringList(stdout, "Current input blockers", prompt.CompletionBlockers)
	}
	_, _ = fmt.Fprintln(stdout, "Boundary:")
	_, _ = fmt.Fprintln(stdout, "- Work only on the current Step.")
	_, _ = fmt.Fprintln(stdout, "- Do not run lifecycle commands or advance the Flow.")
	_, _ = fmt.Fprintln(stdout, "- When the Objective and declared requirements are satisfied, stop and report completion to the caller.")
}

func writeArtifactList(stdout io.Writer, label string, artifacts []command.ArtifactResult) {
	_, _ = fmt.Fprintf(stdout, "%s:\n", label)
	if len(artifacts) == 0 {
		_, _ = fmt.Fprintln(stdout, "- none")
		return
	}
	for _, artifact := range artifacts {
		_, _ = fmt.Fprintf(stdout, "- %s\n", artifact.Path)
	}
}

func writeStringList(stdout io.Writer, label string, values []string) {
	_, _ = fmt.Fprintf(stdout, "%s:\n", label)
	if len(values) == 0 {
		_, _ = fmt.Fprintln(stdout, "- none")
		return
	}
	for _, value := range values {
		_, _ = fmt.Fprintf(stdout, "- %s\n", value)
	}
}
