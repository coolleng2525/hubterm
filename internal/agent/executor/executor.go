// Package executor provides shell command execution with timeout and PTY support.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"
)

// Result 命令执行结果
type Result struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Duration int64  `json:"duration_ms"` // 执行耗时(ms)
}

// Execute 执行 shell 命令，返回结果
// 支持超时控制。command 可以是任意 shell 命令，通过系统 shell 执行。
func Execute(command string, timeout time.Duration) (*Result, error) {
	start := time.Now()

	// 创建带超时的 context
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 通过系统 shell 执行
	shell, shellFlag := detectShell()
	cmd := exec.CommandContext(ctx, shell, shellFlag, command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	duration := time.Since(start).Milliseconds()
	exitCode := 0

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			// 超时
			return &Result{
				Stdout:   stdout.String(),
				Stderr:   fmt.Sprintf("command timed out after %v", timeout),
				ExitCode: -1,
				Duration: duration,
			}, nil
		}
		// 尝试获取退出码
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}, nil
}

// ExecuteWithPty 在 PTY 中执行命令（支持交互式命令）
// rows, cols 指定终端尺寸，为 0 时使用默认值 80x24。
func ExecuteWithPty(command string, rows, cols int) (*Result, error) {
	start := time.Now()

	if rows <= 0 {
		rows = 24
	}
	if cols <= 0 {
		cols = 80
	}

	// 使用 script 命令模拟 PTY（Linux/macOS 通用）
	scriptArgs := []string{"-q", "-c", command}

	// 尝试使用 script 命令创建 PTY
	cmd := exec.Command("script", scriptArgs...)

	// 设置终端尺寸环境变量
	cmd.Env = append(cmd.Environ(),
		fmt.Sprintf("LINES=%d", rows),
		fmt.Sprintf("COLUMNS=%d", cols),
		"TERM=xterm-256color",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	duration := time.Since(start).Milliseconds()
	exitCode := 0

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// script 命令输出可能包含控制字符，清理一下
	output := cleanScriptOutput(stdout.String())

	return &Result{
		Stdout:   output,
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}, nil
}

// detectShell 检测系统可用的 shell
func detectShell() (string, string) {
	// 优先使用 bash，否则 fallback 到 sh
	for _, shell := range []string{"/bin/bash", "/bin/sh"} {
		if _, err := exec.LookPath(shell); err == nil {
			return shell, "-c"
		}
	}
	// Windows fallback
	if _, err := exec.LookPath("cmd"); err == nil {
		return "cmd", "/c"
	}
	return "/bin/sh", "-c"
}

// cleanScriptOutput 清理 script 命令输出中的控制字符和多余内容
func cleanScriptOutput(output string) string {
	// 移除 script 命令可能添加的 "Script started" 等头部
	lines := strings.Split(output, "\n")
	var cleaned []string
	for _, line := range lines {
		// 跳过 script 命令的元信息行
		if strings.HasPrefix(line, "Script started") || strings.HasPrefix(line, "Script done") {
			continue
		}
		// 移除控制字符（保留可打印字符和常见空白）
		clean := strings.Map(func(r rune) rune {
			if unicode.IsPrint(r) || r == '\t' || r == '\r' {
				return r
			}
			return -1
		}, line)
		cleaned = append(cleaned, clean)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
