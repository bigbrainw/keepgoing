// Package lid keeps the Mac running with the lid closed by toggling
// `pmset disablesleep` — the only userspace switch that overrides clamshell
// sleep. It needs root, so a one-time sudoers rule is installed that allows
// exactly four pmset commands without a password.
package lid

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SudoersPath is the drop-in file the installer writes.
const SudoersPath = "/etc/sudoers.d/keepgoing"

// Sudoers is the rule: admins may run the four exact pmset commands.
const Sudoers = `# keepgoing: let the daemon keep the Mac awake with the lid closed.
# Only these four exact commands, nothing else.
%admin ALL=(root) NOPASSWD: /usr/bin/pmset -a disablesleep 1, /usr/bin/pmset -a disablesleep 0, /usr/bin/pmset -a lowpowermode 1, /usr/bin/pmset -a lowpowermode 0
`

// InstallScript is the shell run as root (via sudo or an admin prompt).
// The rule travels as base64 so it survives sh and AppleScript quoting
// unchanged; visudo -c validates and grep proves the rule line is present
// (a quoting slip would otherwise leave a file that is all comment and
// "parses OK").
var InstallScript = fmt.Sprintf(`set -e
umask 077
tmp=$(mktemp)
echo %s | /usr/bin/base64 -d > "$tmp"
grep -q '^%%admin ALL=(root) NOPASSWD: /usr/bin/pmset -a disablesleep 1, /usr/bin/pmset -a disablesleep 0, /usr/bin/pmset -a lowpowermode 1, /usr/bin/pmset -a lowpowermode 0$' "$tmp"
/usr/sbin/visudo -cf "$tmp" >/dev/null
install -m 0440 -o root -g wheel "$tmp" %s
rm -f "$tmp"
echo installed %s`, base64.StdEncoding.EncodeToString([]byte(Sudoers)), SudoersPath, SudoersPath)

// UninstallScript removes the rule and re-enables sleep.
var UninstallScript = fmt.Sprintf(`rm -f %s; /usr/bin/pmset -a disablesleep 0; /usr/bin/pmset -a lowpowermode 0; echo removed`, SudoersPath)

func canSudo(subcmd, val string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "sudo", "-n", "-l", "/usr/bin/pmset", "-a", subcmd, val).Run() == nil
}

// Available reports whether all four passwordless pmset commands work right now.
func Available() bool {
	return canSudo("disablesleep", "1") && canSudo("disablesleep", "0") &&
		canSudo("lowpowermode", "1") && canSudo("lowpowermode", "0")
}

// LegacyAvailable reports whether the old two-command rule is installed.
func LegacyAvailable() bool {
	return canSudo("disablesleep", "1") && canSudo("disablesleep", "0") &&
		!canSudo("lowpowermode", "1")
}

// SleepDisabled reads the live pmset state.
func SleepDisabled() bool {
	out, err := exec.Command("pmset", "-g").Output()
	if err != nil {
		return false
	}
	for _, ln := range strings.Split(string(out), "\n") {
		f := strings.Fields(ln)
		if len(f) == 2 && f[0] == "SleepDisabled" {
			return f[1] == "1"
		}
	}
	return false
}

// LowPowerMode reads whether Low Power Mode is on.
func LowPowerMode() bool {
	out, err := exec.Command("pmset", "-g").Output()
	if err != nil {
		return false
	}
	for _, ln := range strings.Split(string(out), "\n") {
		f := strings.Fields(ln)
		if len(f) >= 2 && f[0] == "lowpowermode" {
			return f[1] == "1"
		}
	}
	return false
}

// Set flips disablesleep. No-op if already in the requested state.
func Set(disable bool) error {
	if SleepDisabled() == disable {
		return nil
	}
	v := "0"
	if disable {
		v = "1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sudo", "-n", "/usr/bin/pmset", "-a", "disablesleep", v).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pmset disablesleep %s: %v %s", v, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetLowPower flips lowpowermode. No-op if already in the requested state.
func SetLowPower(enable bool) error {
	if LowPowerMode() == enable {
		return nil
	}
	v := "0"
	if enable {
		v = "1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sudo", "-n", "/usr/bin/pmset", "-a", "lowpowermode", v).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pmset lowpowermode %s: %v %s", v, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Closed reports whether the MacBook lid is shut (ioreg AppleClamshellState).
func Closed() bool {
	out, err := exec.Command("ioreg", "-r", "-k", "AppleClamshellState", "-d", "4").Output()
	if err != nil {
		return false
	}
	for _, ln := range strings.Split(string(out), "\n") {
		if strings.Contains(ln, "AppleClamshellState") && strings.Contains(ln, "Yes") {
			return true
		}
	}
	return false
}
