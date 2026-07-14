package report

import (
	"fmt"
	"strings"

	"github.com/xuzhang3/alicloud-v2-check/internal/rules"
	"github.com/xuzhang3/alicloud-v2-check/internal/scanner"
)

// Lang selects the output language.
type Lang string

const (
	LangZH Lang = "zh"
	LangEN Lang = "en"
)

// strings bundle for one language.
type bundle struct {
	reportTitle string
	scanPath    string
	clean       string
	legendHead  string
	legend      map[scanner.Category]string
	heuristic   string
	heurTag     string
	catTitle    map[scanner.Category]string
	lblFile     string
	lblResource string
	lblModule   string
	lblField    string
	lblAdvice   string
	lblCode     string
	unknownType string
	summary     string // has two %d
	refLine     string
	treeTitle   string
	// version gating
	versionDetected string // "%s" = constraint
	versionSkip     string // v3+ notice
	versionRelevant string
	versionOverride string
	kindResource    string
	kindDataSource  string
	// finding message templates
	argMsg     string // %[1]s = attr
	refMsg     string // %[1]s = attr, %[2]s = key
	moduleMsg  string
	presentMsg string // %[1]s = kind, %[2]s = fields
}

var bundles = map[Lang]bundle{
	LangZH: {
		reportTitle: "Alicloud Provider v2 Breaking Change 扫描报告",
		scanPath:    "扫描路径: ",
		clean:       "未发现受影响的资源、写法或模块。可放心升级到 v2（仍建议先跑 terraform plan 复核）。",
		legendHead:  "【类别说明】所有 v2 breaking change 本质都是属性从 TypeMap 变为 TypeList：",
		legend: map[scanner.Category]string{
			scanner.ARG:     "map 赋值参数 → 改成 block 写法。  例: `runtime = { ... }`  →  `runtime { ... }`",
			scanner.REF:     "map 下标引用 → 改成 list 下标。  例: `x.connections[\"key\"]`  →  `x.connections[0].key`",
			scanner.MODULE:  "引用了已知受影响的模块。  需升级模块版本并核对其 output 引用写法。",
			scanner.PRESENT: "（信息）出现受影响的资源/数据源。  未必有错，升级后请核对其 map→list 属性。",
		},
		heuristic: "  标注 [启发式/需人工确认] = 仅凭属性名匹配、无法确定所属资源类型，需人工判断。",
		heurTag:   "[启发式/需人工确认]",
		catTitle: map[scanner.Category]string{
			scanner.ARG:     "map 赋值参数需改为 block 写法",
			scanner.REF:     "map 下标引用需改为 list 下标",
			scanner.MODULE:  "引用了已知受影响的模块",
			scanner.PRESENT: "出现受影响的资源 / 数据源（信息）",
		},
		lblFile: "文件", lblResource: "资源", lblModule: "模块", lblField: "字段",
		lblAdvice: "建议", lblCode: "代码",
		unknownType:     "(无法确定,需人工确认)",
		summary:         "汇总: 需处理 %d 处（ARG/REF/MODULE），信息提示 %d 处。",
		refLine:         "参考: 官方 version-2-upgrade 升级指南",
		treeTitle:       "工作空间结构（⚠ n = 待处理项数，✓ = 无问题）：",
		versionDetected: "检测到 aliyun/alicloud provider 版本约束: %s",
		versionSkip:     "该约束已指向 v3 及以上；本工具仅检查 1.x → 2.0 升级，判定为不适用，已跳过。可用 --ignore-version 强制扫描。",
		versionRelevant: "该约束覆盖 v1/v2，适用本次 v2 升级检查。",
		versionOverride: "（--ignore-version 已开启，忽略版本判定）",
		kindResource:    "resource",
		kindDataSource:  "data source",
		argMsg:          "map 赋值 `%[1]s = {` 需改为 block 写法 `%[1]s { ... }`",
		refMsg:          "`.%[1]s[\"%[2]s\"]` 需改为 `.%[1]s[0].%[2]s`",
		moduleMsg:       "该模块内部使用受影响资源，请升级到兼容 v2 的模块版本并核对其 output 引用",
		presentMsg:      "受影响 %[1]s，升级后请核对其 map->list 属性: %[2]s",
	},
	LangEN: {
		reportTitle: "Alicloud Provider v2 Breaking Change Report",
		scanPath:    "Scanned: ",
		clean:       "No affected resources, syntax, or modules found. Safe to upgrade to v2 (still run `terraform plan` to confirm).",
		legendHead:  "[Legend] Every v2 breaking change is an attribute changing from TypeMap to TypeList:",
		legend: map[scanner.Category]string{
			scanner.ARG:     "map-assign argument → block syntax.  e.g. `runtime = { ... }`  →  `runtime { ... }`",
			scanner.REF:     "map-index reference → list index.  e.g. `x.connections[\"key\"]`  →  `x.connections[0].key`",
			scanner.MODULE:  "uses a known-affected module.  Upgrade the module version and review its output references.",
			scanner.PRESENT: "(info) an affected resource/data source is present.  Not necessarily wrong; review its map→list attributes after upgrade.",
		},
		heuristic: "  [heuristic/verify] = matched by attribute name only; the owning resource type could not be determined.",
		heurTag:   "[heuristic/verify]",
		catTitle: map[scanner.Category]string{
			scanner.ARG:     "map-assign argument must become block syntax",
			scanner.REF:     "map-index reference must become list index",
			scanner.MODULE:  "uses a known-affected module",
			scanner.PRESENT: "affected resource / data source present (info)",
		},
		lblFile: "File", lblResource: "Resource", lblModule: "Module", lblField: "Field",
		lblAdvice: "Fix", lblCode: "Code",
		unknownType:     "(unknown, verify manually)",
		summary:         "Summary: %d to fix (ARG/REF/MODULE), %d info.",
		refLine:         "Reference: official version-2-upgrade guide",
		treeTitle:       "Workspace structure (⚠ n = items to fix, ✓ = clean):",
		versionDetected: "Detected aliyun/alicloud provider version constraint: %s",
		versionSkip:     "This constraint targets v3+; this tool only checks the 1.x → 2.0 upgrade, so it does not apply and was skipped. Use --ignore-version to force a scan.",
		versionRelevant: "This constraint covers v1/v2 and is in scope for the v2 upgrade check.",
		versionOverride: "(--ignore-version set; version gating skipped)",
		kindResource:    "resource",
		kindDataSource:  "data source",
		argMsg:          "map assignment `%[1]s = {` must become block syntax `%[1]s { ... }`",
		refMsg:          "`.%[1]s[\"%[2]s\"]` must become `.%[1]s[0].%[2]s`",
		moduleMsg:       "this module internally uses affected resources; upgrade to a v2-compatible version and review its output references",
		presentMsg:      "affected %[1]s; after upgrade review its map→list attributes: %[2]s",
	},
}

func b(lang Lang) bundle {
	if bd, ok := bundles[lang]; ok {
		return bd
	}
	return bundles[LangZH]
}

// localize renders the human-readable message for a finding in the given lang.
func localize(f scanner.Finding, lang Lang) string {
	bd := b(lang)
	switch f.Category {
	case scanner.ARG:
		return fmt.Sprintf(bd.argMsg, f.Attr)
	case scanner.REF:
		return fmt.Sprintf(bd.refMsg, f.Attr, f.Key)
	case scanner.MODULE:
		return bd.moduleMsg
	case scanner.PRESENT:
		kind := bd.kindResource
		if _, ok := rules.AffectedDataSources[f.Target]; ok {
			kind = bd.kindDataSource
		}
		return fmt.Sprintf(bd.presentMsg, kind, f.Attr)
	}
	return ""
}

// AutoLang picks a language from an environment value (LANG/LC_ALL). Defaults to en.
func AutoLang(env string) Lang {
	if strings.HasPrefix(strings.ToLower(env), "zh") {
		return LangZH
	}
	return LangEN
}
