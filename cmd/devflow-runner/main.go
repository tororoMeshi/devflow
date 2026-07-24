package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/8noki8/devflow/internal/automationruntime"
)

const usage = `Usage:
  devflow-runner execute [--project-root <path>] [--devflow <executable>] --step <step-id> --attempt <attempt-id> [--timeout <duration>] [--terminate-grace <duration>] -- <executor> [executor-args...]
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cfg, ok := parseArgs(args)
	if !ok {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	outcome := automationruntime.Run(ctx, cfg)
	if err := automationruntime.WriteResult(stdout, outcome.Result); err != nil {
		_, _ = fmt.Fprintln(stderr, "runtime_io:result_write_failed")
		return 6
	}
	if outcome.ExitCode != 0 && outcome.Result.Error != nil {
		_, _ = fmt.Fprintf(stderr, "%s:%s\n", outcome.Result.Error.Category, outcome.Result.Error.Code)
	} else if outcome.CleanupCode != "" {
		_, _ = fmt.Fprintf(stderr, "runtime_io:%s\n", outcome.CleanupCode)
	}
	return outcome.ExitCode
}

func parseArgs(args []string) (automationruntime.Config, bool) {
	cfg := automationruntime.Config{
		ProjectRoot: ".", Devflow: "devflow", TerminateGrace: 5 * time.Second,
	}
	if len(args) == 0 || args[0] != "execute" {
		return cfg, false
	}
	values := map[string]bool{}
	separator := -1
	for i := 1; i < len(args); {
		if args[i] == "--" {
			separator = i
			break
		}
		option := args[i]
		if !strings.HasPrefix(option, "--") || strings.Contains(option, "=") || values[option] || i+1 >= len(args) {
			return cfg, false
		}
		switch option {
		case "--project-root", "--devflow", "--step", "--attempt", "--timeout", "--terminate-grace":
		default:
			return cfg, false
		}
		raw := args[i+1]
		value := strings.TrimSpace(raw)
		if value == "" || raw != value {
			return cfg, false
		}
		values[option] = true
		switch option {
		case "--project-root":
			cfg.ProjectRoot = value
		case "--devflow":
			cfg.Devflow = value
		case "--step":
			cfg.StepID = value
		case "--attempt":
			cfg.AttemptID = value
		case "--timeout":
			duration, err := time.ParseDuration(value)
			if err != nil || duration < 0 {
				return cfg, false
			}
			cfg.Timeout = duration
		case "--terminate-grace":
			duration, err := time.ParseDuration(value)
			if err != nil || duration < 0 {
				return cfg, false
			}
			cfg.TerminateGrace = duration
		}
		i += 2
	}
	if separator < 0 || separator+1 >= len(args) || cfg.StepID == "" || cfg.AttemptID == "" ||
		args[separator+1] == "" {
		return cfg, false
	}
	cfg.Executor = args[separator+1]
	cfg.ExecutorArgs = append([]string(nil), args[separator+2:]...)
	return cfg, true
}
