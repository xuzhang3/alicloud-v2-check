// Package scanner walks Terraform HCL files and reports alicloud provider v2
// breaking-change usages. It is a line-based regex "locator", not a full HCL
// parser — final source of truth is `terraform plan`.
package scanner

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xuzhang3/alicloud-v2-check/internal/rules"
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

// Finding is one detected item. Human-readable text (Message) is filled in by
// the report layer so it can be localized; the scanner only produces the
// structured fields below.
type Finding struct {
	File       string     `json:"file"`
	Line       int        `json:"line"`
	Category   Category   `json:"category"`
	Target     string     `json:"target"` // resource/data type, or module source
	Attr       string     `json:"attr"`   // attr name (ARG/REF) or comma-joined fields (PRESENT)
	Key        string     `json:"key,omitempty"`
	Confidence Confidence `json:"confidence"`
	Message    string     `json:"message,omitempty"` // localized by report layer
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

// Engine selects the parsing backend.
type Engine string

const (
	// EngineAuto uses the HCL AST parser, falling back to regex per-file when
	// a file cannot be parsed (broken/partial HCL). This is the default.
	EngineAuto Engine = "auto"
	// EngineHCL uses only the HCL AST parser; unparseable files are skipped.
	EngineHCL Engine = "hcl"
	// EngineRegex uses only the line-based regex scanner.
	EngineRegex Engine = "regex"
)

// Options controls a scan.
type Options struct {
	// Excludes are filepath.Match-style patterns (matched against each path
	// and its base name); matching paths are skipped.
	Excludes []string
	// Engine selects the backend (default EngineAuto).
	Engine Engine
}

// CollectFiles returns the de-duplicated list of .tf files under paths,
// honoring excludes and skip-dirs.
func CollectFiles(paths []string, opts Options) ([]string, error) {
	var files []string
	seen := map[string]bool{}
	for _, root := range paths {
		fs, err := collectTFFiles(root, opts)
		if err != nil {
			return nil, err
		}
		for _, f := range fs {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	return files, nil
}

// ScanFiles scans an explicit list of files with the given engine.
func ScanFiles(files []string, engine Engine) ([]Finding, error) {
	if engine == "" {
		engine = EngineAuto
	}
	var findings []Finding
	for _, f := range files {
		fs, err := scanOne(f, engine)
		if err != nil {
			return nil, err
		}
		findings = append(findings, fs...)
	}
	return findings, nil
}

// ScanPaths scans every path (file or dir), de-duplicating files.
func ScanPaths(paths []string, opts Options) ([]Finding, int, error) {
	files, err := CollectFiles(paths, opts)
	if err != nil {
		return nil, 0, err
	}
	findings, err := ScanFiles(files, opts.Engine)
	return findings, len(files), err
}

// scanOne dispatches a single file to the selected engine.
func scanOne(path string, engine Engine) ([]Finding, error) {
	switch engine {
	case EngineRegex:
		return ScanFile(path)
	case EngineHCL:
		fs, err := ScanFileHCL(path)
		if errors.Is(err, ErrParse) {
			return nil, nil // skip unparseable file
		}
		return fs, err
	default: // auto
		fs, err := ScanFileHCL(path)
		if errors.Is(err, ErrParse) {
			return ScanFile(path) // fall back to regex
		}
		return fs, err
	}
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
	absRoot, _ := filepath.Abs(root)
	var files []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Never skip the root path itself, even if its name matches SkipDirs.
			absPath, _ := filepath.Abs(path)
			isRoot := absPath == absRoot
			if !isRoot && (rules.SkipDirs[d.Name()] || excluded(path, opts)) {
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

		// Track enclosing affected block type (shallow) and emit PRESENT.
		// Reset currentType on ANY resource/data decl so non-affected blocks
		// don't inherit stale type from a previous affected block.
		if m := reResourceDecl.FindStringSubmatch(line); m != nil {
			if rules.IsAffectedType(m[1]) {
				currentType = m[1]
				findings = append(findings, newPresent(path, lineNo, m[1], raw))
			} else {
				currentType = ""
			}
		}
		if m := reDataDecl.FindStringSubmatch(line); m != nil {
			if rules.IsAffectedType(m[1]) {
				currentType = m[1]
				findings = append(findings, newPresent(path, lineNo, m[1], raw))
			} else {
				currentType = ""
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
				Attr: attr, Confidence: conf, Code: raw,
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
				Attr: attr, Key: key, Confidence: conf, Code: raw,
			})
		}

		// [MODULE] known-affected module
		if m := reSource.FindStringSubmatch(line); m != nil {
			base := strings.TrimRight(strings.SplitN(m[1], "//", 2)[0], "/")
			if rules.IsAffectedModule(base) {
				findings = append(findings, Finding{
					File: path, Line: lineNo, Category: MODULE, Target: base,
					Confidence: High, Code: raw,
				})
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return findings, nil
}

// newPresent builds a PRESENT finding, looking up the affected field list for
// the given resource/data type from the rules catalog. Shared by both engines.
func newPresent(path string, line int, typ, code string) Finding {
	attrs := rules.AffectedResources[typ]
	if attrs == nil {
		attrs = rules.AffectedDataSources[typ]
	}
	return Finding{
		File: path, Line: line, Category: PRESENT, Target: typ,
		Attr: strings.Join(attrs, ", "), Confidence: High, Code: code,
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
// Properly handles backslash-escaped quotes inside strings.
func stripInlineComment(line string) string {
	inStr := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '\\' && inStr {
			i++ // skip escaped char
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		if c == '#' {
			return line[:i]
		}
		if c == '/' && i+1 < len(line) && line[i+1] == '/' {
			return line[:i]
		}
	}
	return line
}
