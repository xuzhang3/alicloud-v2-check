// Command alicloud-v2-check scans Terraform HCL for aliyun/alicloud provider
// v2.0.0 breaking changes (all rooted in TypeMap -> TypeList attribute changes).
//
// It is read-only: it locates and reports problems with file:line, and never
// modifies any file.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aliyun/alicloud-v2-check/internal/report"
	"github.com/aliyun/alicloud-v2-check/internal/scanner"
)

// injected via -ldflags at build time
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const usage = `alicloud-v2-check — 扫描 Terraform HCL 的 aliyun/alicloud provider v2 breaking change

用法:
  alicloud-v2-check [选项] [路径...]

路径:
  一个或多个目录或 .tf 文件；缺省为当前目录。递归扫描，自动跳过
  .terraform / .git / .idea / .vscode / node_modules。

选项:
  --format text|json   输出格式（默认 text）
  --json               等价于 --format json
  --exclude <glob>     排除路径（可重复）；支持 **/dir/** 段匹配。
                       默认已内置排除 **/.claude/**
  --fail-on <policy>   退出码策略: none|module|ref|arg|any（默认 any）
                         any    有任意 ARG/REF/MODULE 即返回 1
                         arg    仅 ARG 触发
                         ref    ARG 或 REF 触发
                         module 仅 MODULE 触发
                         none   始终返回 0
  --no-color           关闭彩色输出（非 TTY 自动关闭）
  --quiet              省略顶部类别说明图例
  --version, -v        打印版本并退出
  --help, -h           打印帮助并退出

退出码:
  0  未发现需处理项（依据 --fail-on）
  1  发现需处理项
  2  运行错误（路径不存在等）

本工具只检查、只报告，绝不修改任何文件。`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type config struct {
	format   string
	excludes []string
	failOn   report.FailOn
	noColor  bool
	quiet    bool
	paths    []string
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg := config{
		format:   "text",
		failOn:   report.FailAny,
		excludes: []string{"**/.claude/**"},
	}
	var extraExcludes []string
	failOnSet := "any"

	i := 0
	next := func(flag string) (string, bool) {
		if i+1 >= len(args) {
			fmt.Fprintf(stderr, "错误: %s 需要一个参数\n", flag)
			return "", false
		}
		i++
		return args[i], true
	}

	for ; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, usage)
			return 0
		case a == "--version" || a == "-v":
			fmt.Fprintf(stdout, "alicloud-v2-check %s (commit %s, built %s)\n", version, commit, date)
			return 0
		case a == "--json":
			cfg.format = "json"
		case a == "--format":
			v, ok := next(a)
			if !ok {
				return 2
			}
			cfg.format = v
		case strings.HasPrefix(a, "--format="):
			cfg.format = strings.TrimPrefix(a, "--format=")
		case a == "--exclude":
			v, ok := next(a)
			if !ok {
				return 2
			}
			extraExcludes = append(extraExcludes, v)
		case strings.HasPrefix(a, "--exclude="):
			extraExcludes = append(extraExcludes, strings.TrimPrefix(a, "--exclude="))
		case a == "--fail-on":
			v, ok := next(a)
			if !ok {
				return 2
			}
			failOnSet = v
		case strings.HasPrefix(a, "--fail-on="):
			failOnSet = strings.TrimPrefix(a, "--fail-on=")
		case a == "--no-color":
			cfg.noColor = true
		case a == "--quiet":
			cfg.quiet = true
		case a == "--":
			cfg.paths = append(cfg.paths, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(stderr, "错误: 未知选项 %q\n\n%s\n", a, usage)
			return 2
		default:
			cfg.paths = append(cfg.paths, a)
		}
	}

	if cfg.format != "text" && cfg.format != "json" {
		fmt.Fprintf(stderr, "错误: --format 只能是 text 或 json（收到 %q）\n", cfg.format)
		return 2
	}
	switch report.FailOn(failOnSet) {
	case report.FailNone, report.FailModule, report.FailRef, report.FailArg, report.FailAny:
		cfg.failOn = report.FailOn(failOnSet)
	default:
		fmt.Fprintf(stderr, "错误: --fail-on 只能是 none|module|ref|arg|any（收到 %q）\n", failOnSet)
		return 2
	}

	cfg.excludes = append(cfg.excludes, extraExcludes...)
	if len(cfg.paths) == 0 {
		cfg.paths = []string{"."}
	}

	findings, scanned, err := scanner.ScanPaths(cfg.paths, scanner.Options{Excludes: cfg.excludes})
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 2
	}

	if cfg.format == "json" {
		if err := report.JSON(stdout, findings, cfg.paths, scanned); err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 2
		}
	} else {
		color := !cfg.noColor && isTerminalWriter(stdout)
		report.Text(stdout, findings, report.Options{
			Roots: cfg.paths,
			Color: color,
			Quiet: cfg.quiet,
		})
	}

	return report.ExitCode(findings, cfg.failOn)
}
