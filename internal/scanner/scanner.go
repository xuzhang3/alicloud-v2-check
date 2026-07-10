// Package scanner walks Terraform HCL files and reports alicloud provider v2
// breaking-change usages. It is a line-based regex "locator", not a full HCL
// parser — final source of truth is `terraform plan`.
package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/aliyun/alicloud-v2-check/internal/rules"
)

// Category classifies a finding.
type Category string

const (
	ARG     Category = "ARG"     // map-assign arg -> block syntax
	REF     Category = "REF"     // map-index reference -> list index
	MODULE  Category = "MODULE"  // known-affected registry module
	PRESENT Category = "PRESENT" // affected resource/data source present (info)
)

// Confidence marks whether a finding is certain or heuristic.
type Confidence string

const (
	High   Confidence = "HIGH"
	Medium Confidence = "MEDIUM"
)

// Finding is one detected item.
type Finding struct {
	File       string     `json:"file"`
	Line       int        `json:"line"`
	Category   Category   `json:"category"`
	Target     string     `json:"target"` // resource/data type, or module source
	Attr       string     `json:"attr"`
	Confidence Confidence `json:"confidence"`
	Message    string     `json:"message"`
	Code       string     `json:"code"`
}

// Actionable reports whether the finding requires a change (not PRESENT).
func (f Finding) Actionable() bool { return f.Category != PRESENT }

var (
	reBlockArg     = regexp.MustCompile(`^\s*(` + strings.Join(rules.BlockArgAttrs, "|") + `)\s*=\s*\{`)
	reMapIndex     = regexp.MustCompile(`\.(` + strings.Join(rules.MapIndexAttrs, "|") + `)\s*\[\s*"([^"]+)"\s*\]`)
	reResourceDecl = regexp.MustCompile(`^\s*resource\s+"([a-z0-9_]+)"`)
	reDataDecl     = regexp.MustCompile(`^\s*data\s+"([a-z0-9_]+)"`)
	reSource       = regexp.MustCompile(`source\s*=\s*"([^"]+)"`)
	reTypeToken    = regexp.MustCompile(`(data\.)?(alicloud_[a-z0-9_]+)`)
)

// Options controls a scan.
type Options struct {
	// Excludes are filepath.Match-style patterns (matched against each path
	// and its base name); matching paths are skipped.
	Excludes []string
}

// ScanPaths scans every path (file or dir), de-duplicating files.
func ScanPaths(paths []string, opts Options) ([]Finding, int, error) {
	var findings []Finding
	seen := map[string]bool{}
	count := 0
	for _, root := range paths {
		files, err := collectTFFiles(root, opts)
		if err != nil {
			return nil, count, err
		}
		for _, f := range files {
			if seen[f] {
				continue
			}
			seen[f] = true
			count++
			fs, err := ScanFile(f)
			if err != nil {
				return nil, count, err
			}
			findings = append(findings, fs...)
		}
	}
	return findings, count, nil
}

func collectTFFiles(root string, opts Options) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if strings.HasSuffix(root, ".tf") && !excluded(root, opts) {
			return []string{root}, nil
		}
		return nil, nil
	}
	var files []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if rules.SkipDirs[d.Name()] || excluded(path, opts) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".tf") && !excluded(path, opts) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func excluded(path string, opts Options) bool {
	for _, pat := range opts.Excludes {
		if ok, _ := filepath.Match(pat, path); ok {
			return true
		}
		if ok, _ := filepath.Match(pat, filepath.Base(path)); ok {
			return true
		}
		// Also treat a bare substring like ".claude" or a glob like **/.claude/**
		// as a path-segment match for convenience.
		if matchPathGlob(pat, path) {
			return true
		}
	}
	return false
}

// matchPathGlob supports simple `**` segment globs (e.g. **/.claude/**).
func matchPathGlob(pat, path string) bool {
	trimmed := strings.Trim(pat, "*/")
	if trimmed == "" {
		return false
	}
	if !strings.ContainsAny(pat, "*?[") {
		return false
	}
	return slices.Contains(strings.Split(filepath.ToSlash(path), "/"), trimmed)
}

// ScanFile scans a single .tf file.
func ScanFile(path string) ([]Finding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var findings []Finding
	currentType := "" // affected resource/data type of the enclosing block, if any

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := sc.Text()
		line := stripInlineComment(raw)

		// Track enclosing affected block type (shallow).
		if m := reResourceDecl.FindStringSubmatch(line); m != nil {
			if rules.IsAffectedType(m[1]) {
				currentType = m[1]
			}
			if attrs, ok := rules.AffectedResources[m[1]]; ok {
				findings = append(findings, present(path, lineNo, m[1], raw, attrs, "resource"))
			}
		}
		if m := reDataDecl.FindStringSubmatch(line); m != nil {
			if rules.IsAffectedType(m[1]) {
				currentType = m[1]
			}
			if attrs, ok := rules.AffectedDataSources[m[1]]; ok {
				findings = append(findings, present(path, lineNo, m[1], raw, attrs, "data source"))
			}
		}

		// [ARG] map assign -> block
		if m := reBlockArg.FindStringSubmatch(line); m != nil {
			attr := m[1]
			conf := Medium
			if rules.IsAffectedType(currentType) {
				conf = High
			}
			findings = append(findings, Finding{
				File: path, Line: lineNo, Category: ARG, Target: currentType,
				Attr: attr, Confidence: conf,
				Message: "map 赋值 `" + attr + " = {` 需改为 block 写法 `" + attr + " { ... }`",
				Code:    raw,
			})
		}

		// [REF] map index -> list index
		for _, m := range reMapIndex.FindAllStringSubmatchIndex(line, -1) {
			attr := line[m[2]:m[3]]
			key := line[m[4]:m[5]]
			prefix := line[:m[0]]
			refType := lastTypeToken(prefix)
			conf := Medium
			if rules.IsAffectedType(refType) {
				conf = High
			}
			findings = append(findings, Finding{
				File: path, Line: lineNo, Category: REF, Target: refType,
				Attr: attr, Confidence: conf,
				Message: "`." + attr + `["` + key + `"]` + "` 需改为 `." + attr + "[0]." + key + "`",
				Code:    raw,
			})
		}

		// [MODULE] known-affected module
		if m := reSource.FindStringSubmatch(line); m != nil {
			base := strings.TrimRight(strings.SplitN(m[1], "//", 2)[0], "/")
			if rules.IsAffectedModule(base) {
				findings = append(findings, Finding{
					File: path, Line: lineNo, Category: MODULE, Target: base,
					Confidence: High,
					Message:    "该模块内部使用受影响资源，请升级到兼容 v2 的模块版本并核对其 output 引用",
					Code:       raw,
				})
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return findings, nil
}

func present(path string, line int, typ, raw string, attrs []string, kind string) Finding {
	fields := strings.Join(attrs, ", ")
	return Finding{
		File: path, Line: line, Category: PRESENT, Target: typ,
		Attr: fields, Confidence: High,
		Message: "受影响 " + kind + "，升级后请核对其 map->list 属性: " + fields,
		Code:    raw,
	}
}

func lastTypeToken(prefix string) string {
	ms := reTypeToken.FindAllStringSubmatch(prefix, -1)
	if len(ms) == 0 {
		return ""
	}
	return ms[len(ms)-1][2]
}

// stripInlineComment removes # and // comments outside of double-quoted strings.
func stripInlineComment(line string) string {
	for _, marker := range []string{"#", "//"} {
		idx := strings.Index(line, marker)
		for idx != -1 {
			if strings.Count(line[:idx], `"`)%2 == 0 {
				line = line[:idx]
				break
			}
			next := strings.Index(line[idx+1:], marker)
			if next == -1 {
				idx = -1
			} else {
				idx = idx + 1 + next
			}
		}
	}
	return line
}
