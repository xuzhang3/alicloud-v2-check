package report

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aliyun/alicloud-v2-check/internal/scanner"
)

type treeNode struct {
	children map[string]*treeNode
	count    int // actionable findings (file: own; dir: aggregate)
	isFile   bool
}

func newTreeNode() *treeNode { return &treeNode{children: map[string]*treeNode{}} }

// Tree prints the scanned .tf files as an ASCII tree, annotating each file with
// its actionable-finding count (⚠ n) or a check (✓), and each directory with
// its aggregate. A quick visual of "what was scanned and where the issues are".
// When plain is true, only ASCII characters are used (safe for markdown code blocks).
func Tree(w io.Writer, files []string, findings []scanner.Finding, opts Options, plain bool) {
	per := map[string]int{}
	for _, f := range findings {
		if f.Actionable() {
			per[f.File]++
		}
	}
	root := newTreeNode()
	for _, fp := range files {
		cur := root
		parts := strings.Split(filepath.ToSlash(fp), "/")
		for i, p := range parts {
			if p == "" {
				continue
			}
			ch, ok := cur.children[p]
			if !ok {
				ch = newTreeNode()
				cur.children[p] = ch
			}
			if i == len(parts)-1 {
				ch.isFile = true
				ch.count = per[fp]
			}
			cur = ch
		}
	}
	aggregate(root)
	if plain {
		switch opts.Lang {
		case LangZH:
			fmt.Fprintln(w, "工作空间结构（! n = 待处理项数，ok = 无问题）：")
		default:
			fmt.Fprintln(w, "Workspace structure (! n = items to fix, ok = clean):")
		}
	} else {
		fmt.Fprintln(w, b(opts.Lang).treeTitle)
	}
	printTree(w, root, "", plain)
}

func aggregate(n *treeNode) int {
	if n.isFile {
		return n.count
	}
	t := 0
	for _, c := range n.children {
		t += aggregate(c)
	}
	n.count = t
	return t
}

func printTree(w io.Writer, n *treeNode, prefix string, plain bool) {
	names := make([]string, 0, len(n.children))
	for name := range n.children {
		names = append(names, name)
	}
	sort.Strings(names)
	for i, name := range names {
		ch := n.children[name]
		last := i == len(names)-1
		var branch, next string
		if plain {
			if last {
				branch, next = "+-- ", "    "
			} else {
				branch, next = "|-- ", "|   "
			}
		} else {
			if last {
				branch, next = "└── ", "    "
			} else {
				branch, next = "├── ", "│   "
			}
		}
		fmt.Fprintf(w, "%s%s%s%s\n", prefix, branch, name, badge(ch, plain))
		if !ch.isFile {
			printTree(w, ch, prefix+next, plain)
		}
	}
}

func badge(n *treeNode, plain bool) string {
	switch {
	case n.isFile && n.count > 0:
		if plain {
			return fmt.Sprintf("  ! %d", n.count)
		}
		return fmt.Sprintf("  ⚠ %d", n.count)
	case n.isFile:
		if plain {
			return "  ok"
		}
		return "  ✓"
	case n.count > 0:
		return fmt.Sprintf("  (%d)", n.count)
	default:
		return ""
	}
}
