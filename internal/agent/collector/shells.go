package collector

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type ShellInfo struct{ ID, Name, Path string }

func ScanShells() []ShellInfo {
	if runtime.GOOS != "windows" {
		return nil
	}
	candidates := []ShellInfo{
		{ID: "pwsh", Name: "PowerShell 7", Path: "pwsh.exe"},
		{ID: "powershell", Name: "Windows PowerShell", Path: "powershell.exe"},
		{ID: "cmd", Name: "Command Prompt", Path: "cmd.exe"},
	}
	result := make([]ShellInfo, 0, 4)
	for _, item := range candidates {
		if path, err := exec.LookPath(item.Path); err == nil {
			item.Path = path
			result = append(result, item)
		}
	}
	gitCandidates := []string{filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe"), filepath.Join(os.Getenv("ProgramFiles(x86)"), "Git", "bin", "bash.exe")}
	if path, err := exec.LookPath("bash.exe"); err == nil {
		gitCandidates = append([]string{path}, gitCandidates...)
	}
	for _, path := range gitCandidates {
		if path != "" {
			if _, err := os.Stat(path); err == nil {
				result = append(result, ShellInfo{ID: "git-bash", Name: "Git Bash", Path: path})
				break
			}
		}
	}
	return result
}
