package ai

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// improveInstruction is wrapped around the input for KindImprove. The input
// is delimited with XML-style tags so it can never be read as an
// instruction.
const improveInstruction = `Fix grammar, spelling and punctuation in the text below, and improve clarity and flow.
Preserve the author's voice, tone, register and formatting (markdown, line breaks, code).
Do not add, remove or reinterpret content. Do not translate.
Output only the corrected text, with no preamble, explanation or quoting.`

// disallowedTools mirrors ~/.dotfiles/extra/scripts/haiku exactly; it is
// blf's contract, not user-configurable.
const disallowedTools = "Bash Read Write Edit Glob Grep Task WebFetch WebSearch TodoWrite NotebookEdit Agent Skill"

// BuildPrompt turns input into the prompt sent to the model. KindAI passes
// input through unchanged, matching the reference haiku script. KindImprove
// wraps it in the fixed instruction with the input delimited in <text> tags.
func BuildPrompt(kind Kind, input string) string {
	if kind == KindImprove {
		return fmt.Sprintf("<instructions>\n%s\n</instructions>\n\n<text>\n%s\n</text>", improveInstruction, input)
	}
	return input
}

// InvokeResult is the outcome of an Invoke call.
type InvokeResult struct {
	Response string
	Status   Status
	Err      error
}

// ExecFunc runs name with args, feeding it stdin, and returns its stdout and
// stderr. It is injected so Run can be tested without shelling out.
type ExecFunc func(ctx context.Context, name string, args []string, stdin io.Reader) (stdout, stderr []byte, err error)

// Exec is the production ExecFunc, running the real subprocess.
func Exec(ctx context.Context, name string, args []string, stdin io.Reader) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.Bytes(), errBuf.Bytes(), err
}

// Invoke builds the prompt for kind/input and invokes claude via execFn with
// the full hardening flag set. The prompt is delivered on stdin, never in
// argv. ctx's deadline, if any, is what bounds the run: an exec error
// coinciding with an expired ctx is reported as StatusTimeout, distinct from
// any other failure.
func Invoke(ctx context.Context, execFn ExecFunc, model string, kind Kind, input string) InvokeResult {
	prompt := BuildPrompt(kind, input)

	args := []string{
		"-p",
		"--model", model,
		"--strict-mcp-config",
		"--mcp-config", `{"mcpServers":{}}`,
		"--disallowed-tools", disallowedTools,
		"--settings", "{}",
	}

	stdout, stderr, err := execFn(ctx, "claude", args, strings.NewReader(prompt))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return InvokeResult{Status: StatusTimeout, Err: ctx.Err()}
		}
		if len(bytes.TrimSpace(stderr)) > 0 {
			return InvokeResult{Status: StatusFailure, Err: fmt.Errorf("%w: %s", err, strings.TrimSpace(string(stderr)))}
		}
		return InvokeResult{Status: StatusFailure, Err: err}
	}

	return InvokeResult{Status: StatusSuccess, Response: strings.TrimSpace(string(stdout))}
}
