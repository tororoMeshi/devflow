package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/8noki8/devflow/internal/command"
)

const usage = `Usage:
  devflow init
  devflow list
  devflow start <flow-id> --task-file <path>
  devflow status
  devflow prompt
  devflow context
  devflow approve --step <step-id> --attempt <attempt-id> --note <note>
  devflow artifact record --step <step-id> --attempt <attempt-id> --path <project-relative-path>
  devflow done
  devflow back [--to <step>] --reason <reason>
  devflow skip --reason <reason>
  devflow finish --reason <reason>
  devflow check request --step <step-id> --attempt <attempt-id> --check <check-id>
  devflow check record --file <result.json>
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
	default:
		writeUsage(stderr)
		return 1
	}

	writeResult(ctx, result)
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

func parseCheckRecordArgs(args []string) (string, bool) {
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

func writeResult(ctx command.Context, result command.CommandResult) {
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
	if result.CheckRequest != nil {
		_ = json.NewEncoder(ctx.Stdout).Encode(result.CheckRequest)
	}
	command.WriteDiagnostics(ctx, result.Diagnostics)
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
	_, _ = fmt.Fprintf(stdout, "Flow: %s - %s\n", status.FlowID, status.FlowTitle)
	_, _ = fmt.Fprintf(stdout, "Current step: %s - %s\n", status.CurrentStepID, status.CurrentStepTitle)
	if status.EntrySequence > 0 {
		_, _ = fmt.Fprintf(stdout, "Entry sequence: %d\n", status.EntrySequence)
	}
	writeStringList(stdout, "Completed steps", status.CompletedSteps)

	_, _ = fmt.Fprintln(stdout, "Skipped steps:")
	for _, stepID := range sortedSkippedStepKeys(status.SkippedSteps) {
		_, _ = fmt.Fprintf(stdout, "- %s: %s\n", stepID, status.SkippedSteps[stepID].Reason)
	}
	if len(status.SkippedSteps) == 0 {
		_, _ = fmt.Fprintln(stdout, "- none")
	}

	_, _ = fmt.Fprintln(stdout, "Approvals:")
	if status.Approval == nil {
		_, _ = fmt.Fprintln(stdout, "- none")
	} else {
		_, _ = fmt.Fprintf(stdout, "- %s: approved=%t note=%s\n", status.Approval.StepID, status.Approval.Approved, status.Approval.Note)
	}

	_, _ = fmt.Fprintln(stdout, "Checks:")
	if len(status.Checks) == 0 {
		_, _ = fmt.Fprintln(stdout, "- none")
	}
	for _, check := range status.Checks {
		if check.ExitCode == nil {
			_, _ = fmt.Fprintf(stdout, "- %s: pending\n", check.CheckID)
			continue
		}
		_, _ = fmt.Fprintf(stdout, "- %s: %s exit=%d", check.CheckID, check.Status, *check.ExitCode)
		if check.LogPath != "" {
			_, _ = fmt.Fprintf(stdout, " log=%s", check.LogPath)
		}
		_, _ = fmt.Fprintln(stdout)
	}
	if len(status.Artifacts) > 0 {
		_, _ = fmt.Fprintln(stdout, "Artifacts:")
		for _, artifact := range status.Artifacts {
			_, _ = fmt.Fprintf(stdout, "- %s: %s\n", artifact.Path, artifact.State)
		}
	}
}

func writePrompt(stdout io.Writer, prompt command.PromptResult) {
	_, _ = fmt.Fprintf(stdout, "Flow: %s\n", prompt.FlowID)
	_, _ = fmt.Fprintf(stdout, "Task:\n%s", prompt.TaskContent)
	if strings.HasSuffix(prompt.TaskContent, "\n") {
		_, _ = io.WriteString(stdout, "\n")
	} else {
		_, _ = io.WriteString(stdout, "\n\n")
	}
	_, _ = fmt.Fprintf(stdout, "Current step: %s - %s\n", prompt.CurrentStepID, prompt.CurrentStepTitle)
	_, _ = fmt.Fprintf(stdout, "Instruction:\n%s\n", prompt.CurrentStepInstruction)
	writeArtifactList(stdout, "Required artifacts", prompt.RequiredArtifacts)
	if len(prompt.OptionalArtifacts) > 0 {
		writeArtifactList(stdout, "Optional artifacts", prompt.OptionalArtifacts)
	}
	_, _ = fmt.Fprintln(stdout, "Required approval:")
	if prompt.RequiredApproval == nil {
		_, _ = fmt.Fprintln(stdout, "- none")
	} else {
		_, _ = fmt.Fprintf(stdout, "- %s\n", prompt.RequiredApproval.StepID)
	}
	writeStringList(stdout, "Required checks", prompt.RequiredChecks)
	if len(prompt.ArtifactBlockers) > 0 {
		writeStringList(stdout, "Artifact blockers", prompt.ArtifactBlockers)
	}
	if len(prompt.CheckBlockers) > 0 {
		writeStringList(stdout, "Check blockers", prompt.CheckBlockers)
	}
	writeStringList(stdout, "After completing", prompt.AfterCompleting.Commands)
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

func sortedSkippedStepKeys(values map[string]command.SkippedStepResult) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
