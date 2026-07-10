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
func Tree(w io.Writer, files []string, findings []scanner.Finding, opts Options) {
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
	fmt.Fprintln(w, b(opts.Lang).treeTitle)
	printTree(w, root, "")
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

func printTree(w io.Writer, n *treeNode, prefix string) {
	names := make([]string, 0, len(n.children))
	for name := range n.children {
		names = append(names, name)
	}
	sort.Strings(names)
	for i, name := range names {
		ch := n.children[name]
		last := i == len(names)-1
		branch, next := "├── ", "│   "
		if last {
			branch, next = "└── ", "    "
		}
		fmt.Fprintf(w, "%s%s%s%s\n", prefix, branch, name, badge(ch))
		if !ch.isFile {
			printTree(w, ch, prefix+next)
		}
	}
}

func badge(n *treeNode) string {
	switch {
	case n.isFile && n.count > 0:
		return fmt.Sprintf("  ⚠ %d", n.count)
	case n.isFile:
		return "  ✓"
	case n.count > 0:
		return fmt.Sprintf("  (%d)", n.count)
	default:
		return ""
	}
}
