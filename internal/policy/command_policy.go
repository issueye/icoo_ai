package policy

import "strings"

func assessCommand(command string) assessment {
	normalized := normalizeCommand(command)
	details := map[string]any{"command": command}
	if normalized == "" {
		return assessment{Risk: RiskLevelLow, Reason: "empty shell command", Details: details}
	}

	switch {
	case hasAny(normalized, []string{
		"rm -rf /",
		"rm -fr /",
		"del /f /s /q",
		"format ",
		"mkfs",
		"dd if=",
		":(){:|:&};:",
	}):
		return assessment{Risk: RiskLevelCritical, Reason: "destructive system command is blocked", Details: details}
	case hasAny(normalized, []string{
		"rm -rf ",
		"rm -fr ",
		"remove-item -recurse",
		"remove-item -r",
		"rmdir /s",
		"rd /s",
		"git clean",
		"git reset --hard",
		"git checkout --",
		"git restore --source",
	}):
		return assessment{Risk: RiskLevelHigh, Reason: "command may delete or overwrite files", Details: details}
	case hasAny(normalized, []string{
		"sudo ",
		"su -",
		"chmod -r 777",
		"chown -r",
		"set-executionpolicy",
		"reg add",
		"reg delete",
		"sc config",
		"systemctl ",
		"launchctl ",
		"defaults write",
	}):
		return assessment{Risk: RiskLevelHigh, Reason: "command may modify system configuration", Details: details}
	case hasAny(normalized, []string{
		"npm install -g",
		"npm i -g",
		"pnpm add -g",
		"yarn global add",
		"pip install ",
		"pip3 install ",
		"go install ",
		"cargo install ",
		"brew install ",
		"choco install ",
		"scoop install ",
	}):
		if hasAny(normalized, []string{" --user", " -r ", " requirements.txt"}) {
			return assessment{Risk: RiskLevelMedium, Reason: "command installs dependencies", Details: details}
		}
		return assessment{Risk: RiskLevelHigh, Reason: "command may install global software", Details: details}
	case hasAny(normalized, []string{
		"curl ",
		"wget ",
		"Invoke-WebRequest",
		"invoke-webrequest",
		"iwr ",
		"scp ",
		"rsync ",
		"ftp ",
		"sftp ",
	}):
		if hasAny(normalized, []string{" -t ", " --upload-file", " -f ", " --form", " -d ", " --data", " --data-binary", " put "}) {
			return assessment{Risk: RiskLevelHigh, Reason: "command may upload data to the network", Details: details}
		}
		return assessment{Risk: RiskLevelMedium, Reason: "command performs network access", Details: details}
	case hasAny(normalized, []string{
		">",
		" tee ",
		"out-file",
		"set-content",
		"add-content",
	}):
		return assessment{Risk: RiskLevelMedium, Reason: "command may write files", Details: details}
	default:
		return assessment{Risk: RiskLevelLow, Reason: "command appears read-only or low risk", Details: details}
	}
}

func normalizeCommand(command string) string {
	command = strings.TrimSpace(command)
	command = strings.ReplaceAll(command, "\r\n", "\n")
	command = strings.Join(strings.Fields(command), " ")
	return command
}

func hasAny(value string, needles []string) bool {
	lowerValue := strings.ToLower(value)
	for _, needle := range needles {
		if strings.Contains(lowerValue, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
