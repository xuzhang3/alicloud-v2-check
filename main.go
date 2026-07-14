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

	"github.com/spf13/cobra"
	"github.com/xuzhang3/alicloud-v2-check/internal/report"
	"github.com/xuzhang3/alicloud-v2-check/internal/scanner"
	"github.com/xuzhang3/alicloud-v2-check/internal/tfversion"
)

// injected via -ldflags at build time
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	// defaultLang, if set to "zh" or "en" at build time, overrides the
	// $LANG-based auto-detection when --lang is not passed.
	defaultLang = ""
)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr))
}

type flags struct {
	format        string
	output        string
	engine        string
	excludes      []string
	failOn        string
	groupBy       string
	lang          string
	ignoreVersion bool
	noColor       bool
}

// execute builds and runs the root command, returning the process exit code.
func execute(args []string, stdout, stderr io.Writer) int {
	exitCode := 0
	cmd := newRootCmd(&exitCode, stdout)
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	if err := cmd.Execute(); err != nil {
		if exitCode == 0 {
			exitCode = 2
		}
	}
	return exitCode
}

func newRootCmd(exitCode *int, stdout io.Writer) *cobra.Command {
	f := &flags{}
	cmd := &cobra.Command{
		Use:   "alicloud-v2-check [flags] [path...]",
		Short: "Scan Terraform HCL for aliyun/alicloud provider v2 breaking changes",
		Long: "扫描 Terraform HCL，找出升级 aliyun/alicloud provider 1.x → 2.0.0 的 breaking change，并定位到 文件:行号。\n" +
			"Scan Terraform HCL for aliyun/alicloud provider 1.x → 2.0.0 breaking changes, located by file:line.\n\n" +
			"只检查、只报告，不修改文件 / Read-only: reports problems, never edits files.",
		Args:          cobra.ArbitraryArgs,
		Version:       fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		SilenceUsage:  false,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(f, args, stdout, exitCode)
		},
	}

	fl := cmd.Flags()
	fl.StringVar(&f.format, "format", "markdown", "output format: text|markdown")
	fl.StringVarP(&f.output, "output", "o", "", "write report to a file instead of stdout")
	fl.StringVar(&f.engine, "engine", "auto", "parser engine: auto|hcl|regex")
	fl.StringArrayVar(&f.excludes, "exclude", nil, "exclude path glob (repeatable)")
	fl.StringVar(&f.failOn, "fail-on", "any", "exit-code policy: none|module|ref|arg|any")
	fl.StringVar(&f.groupBy, "group-by", "resource", "group findings by: category|resource")
	fl.StringVar(&f.lang, "lang", "", "language: zh|en (default: auto from $LANG)")
	fl.BoolVar(&f.ignoreVersion, "ignore-version", false, "scan even if the provider constraint targets v3+")
	fl.BoolVar(&f.noColor, "no-color", false, "disable colored output")

	cmd.SetVersionTemplate("alicloud-v2-check {{.Version}}\n")
	return cmd
}

func runScan(f *flags, paths []string, stdout io.Writer, exitCode *int) error {
	// resolve & validate flags
	format := f.format
	switch format {
	case "text", "markdown", "md":
		if format == "md" {
			format = "markdown"
		}
	default:
		return fmt.Errorf("--format must be text|markdown (got %q)", format)
	}

	eng := scanner.Engine(f.engine)
	switch eng {
	case scanner.EngineAuto, scanner.EngineHCL, scanner.EngineRegex:
	default:
		return fmt.Errorf("--engine must be auto|hcl|regex (got %q)", f.engine)
	}

	failOn := report.FailOn(f.failOn)
	switch failOn {
	case report.FailNone, report.FailModule, report.FailRef, report.FailArg, report.FailAny:
	default:
		return fmt.Errorf("--fail-on must be none|module|ref|arg|any (got %q)", f.failOn)
	}

	groupBy := report.GroupBy(f.groupBy)
	switch groupBy {
	case report.GroupByCategory, report.GroupByResource:
	default:
		return fmt.Errorf("--group-by must be category|resource (got %q)", f.groupBy)
	}

	var lang report.Lang
	switch f.lang {
	case "":
		if defaultLang != "" {
			lang = report.Lang(defaultLang)
		} else {
			lang = report.AutoLang(os.Getenv("LANG") + os.Getenv("LC_ALL"))
		}
	case string(report.LangZH), string(report.LangEN):
		lang = report.Lang(f.lang)
	default:
		return fmt.Errorf("--lang must be zh|en (got %q)", f.lang)
	}

	if len(paths) == 0 {
		paths = []string{"."}
	}
	opts := scanner.Options{Excludes: f.excludes, Engine: eng}

	files, err := scanner.CollectFiles(paths, opts)
	if err != nil {
		return err
	}

	verdict := tfversion.Detect(files)
	note := report.VersionNote(verdict, lang, f.ignoreVersion)
	skip := verdict.OnlyV3Plus && !f.ignoreVersion

	var findings []scanner.Finding
	if !skip {
		findings, err = scanner.ScanFiles(files, eng)
		if err != nil {
			return err
		}
	}

	// resolve output destination
	w := stdout
	if f.output != "" {
		file, err := os.Create(f.output)
		if err != nil {
			return err
		}
		defer file.Close()
		w = file
	}

	ropts := report.Options{
		Roots:       paths,
		Color:       f.output == "" && !f.noColor && isTerminalWriter(stdout),
		Lang:        lang,
		GroupBy:     groupBy,
		VersionNote: note,
	}
	switch format {
	case "markdown":
		fmt.Fprint(w, "```\n")
		report.Tree(w, files, findings, ropts, true)
		fmt.Fprint(w, "```\n\n")
		report.Markdown(w, findings, ropts)
	default:
		report.Tree(w, files, findings, ropts, false)
		fmt.Fprintln(w)
		report.Text(w, findings, ropts)
	}

	if skip {
		*exitCode = 0
	} else {
		*exitCode = report.ExitCode(findings, failOn)
	}
	return nil
}
