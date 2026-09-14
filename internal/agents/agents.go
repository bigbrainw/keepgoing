// Package agents knows how to point each coding agent at the proxy and how to
// resume it after a crash.
package agents

import (
	"path/filepath"
	"strings"
)

// Kind identifies a supported agent.
type Kind string

const (
	Claude  Kind = "claude"
	Codex   Kind = "codex"
	Unknown Kind = ""
)

const resumePrompt = "You were interrupted (network or process restart). Continue the previous task exactly where you left off. Do not restart from scratch."

// Detect guesses the agent from argv[0].
func Detect(argv []string) Kind {
	if len(argv) == 0 {
		return Unknown
	}
	switch filepath.Base(argv[0]) {
	case "claude":
		return Claude
	case "codex":
		return Codex
	}
	return Unknown
}

// Env returns env vars that route the agent through the proxy.
//
// Claude Code honours ANTHROPIC_BASE_URL, so it gets the HTTP holding proxy
// (request-level hold, partial-stream accounting).
//
// Codex under ChatGPT login hard-codes wss://chatgpt.com/backend-api/codex/
// responses and ignores chatgpt_base_url for it, so it gets the CONNECT
// tunnel via HTTPS_PROXY (reqwest honours it). Hold happens at CONNECT time.
func Env(k Kind, base, tun string) []string {
	switch k {
	case Claude:
		return []string{
			"ANTHROPIC_BASE_URL=" + base + "/anthropic",
			// SDK default is 10 min; a held request must not trip the client timeout.
			"API_TIMEOUT_MS=3600000",
		}
	case Codex:
		return []string{
			"HTTPS_PROXY=" + tun,
			"HTTP_PROXY=" + tun,
			"NO_PROXY=localhost,127.0.0.1,::1",
		}
	}
	return nil
}

// Argv rewrites the launch command if an agent needs flags rather than env.
// Currently a no-op for both agents; kept as the extension point.
func Argv(k Kind, argv []string, base, tun string) []string {
	return argv
}

// ResumeArgv builds the command that continues the most recent session.
// It keeps mode/permission/model flags from the original invocation and
// drops the positional prompt.
func ResumeArgv(k Kind, original []string, base, tun string) []string {
	if len(original) == 0 {
		return nil
	}
	keep := carryFlags(k, original[1:])
	switch k {
	case Claude:
		headless := hasAny(original, "-p", "--print")
		out := []string{original[0]}
		if headless {
			out = append(out, "-p", "--continue", resumePrompt)
		} else {
			out = append(out, "--continue")
		}
		return append(out, keep...)
	case Codex:
		headless := len(original) > 1 && (original[1] == "exec" || original[1] == "e")
		out := Argv(Codex, []string{original[0]}, base, tun)
		if headless {
			out = append(out, "exec", "resume", "--last")
			out = append(out, keep...)
			return append(out, resumePrompt)
		}
		out = append(out, "resume", "--last")
		return append(out, keep...)
	}
	return nil
}

// flags whose value we carry over on resume. value=true means it takes an arg.
var carry = map[Kind]map[string]bool{
	Claude: {
		"--dangerously-skip-permissions": false,
		"--permission-mode":              true,
		"--model":                        true,
		"--allowedTools":                 true,
		"--disallowedTools":              true,
		"--max-turns":                    true,
		"--output-format":                true,
		"--verbose":                      false,
	},
	Codex: {
		"--full-auto": false,
		"--yolo":      false,
		"--dangerously-bypass-approvals-and-sandbox": false,
		"-a": true, "--ask-for-approval": true,
		"-s": true, "--sandbox": true,
		"-m": true, "--model": true,
		"-c": true, "--config": true,
		"--skip-git-repo-check": false,
	},
}

func carryFlags(k Kind, args []string) []string {
	spec := carry[k]
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, inline, hasEq := strings.Cut(a, "=")
		takes, ok := spec[name]
		if !ok {
			continue
		}
		if !takes {
			out = append(out, a)
			continue
		}
		if hasEq {
			out = append(out, name, inline)
			continue
		}
		if i+1 < len(args) {
			out = append(out, name, args[i+1])
			i++
		}
	}
	return out
}

func hasAny(argv []string, names ...string) bool {
	for _, a := range argv {
		for _, n := range names {
			if a == n {
				return true
			}
		}
	}
	return false
}

func hasFlag(argv []string, needle string) bool {
	for _, a := range argv {
		if strings.Contains(a, needle) {
			return true
		}
	}
	return false
}
