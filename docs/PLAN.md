# 实现计划：alicloud-v2-check（跨平台 CLI）

## 1. 目标

把现有的 Python 扫描器（`.claude/skills/alicloud-provider-v2-breaking-change-check/scripts/scan.py`）
重写为一个 **纯 CLI 工具**：单个静态二进制、零运行时依赖，可按 `os/arch` 分发到各平台。

用于扫描 Terraform HCL，检测升级 `aliyun/alicloud` provider `1.x → 2.0.0` 的 breaking change
（本质都是属性 `TypeMap → TypeList`），并定位到 `文件:行号`。**只检查、只报告，不修改用户文件。**

## 2. 技术选型（已确认）

- **语言**：Go —— 单静态二进制，`GOOS`/`GOARCH` 一条命令交叉编译所有平台，零依赖。
- **分发**：GoReleaser + GitHub Actions —— 打 tag 自动为所有 os/arch 产出压缩包 + `checksums.txt` + GitHub Release。
- **功能范围**：对齐现有 Python 扫描器 **并增强**。
- 仅用 Go 标准库（`regexp`/`flag`/`os`/`path/filepath`/`encoding/json`），不引入第三方依赖，
  保证交叉编译干净、二进制体积小。

## 3. 目标平台矩阵

| GOOS | GOARCH | 备注 |
|------|--------|------|
| linux | amd64 | |
| linux | arm64 | |
| darwin | amd64 | Intel Mac |
| darwin | arm64 | Apple Silicon |
| windows | amd64 | 产物 `.exe` |
| windows | arm64 | 可选 |

（CGO 关闭：`CGO_ENABLED=0`，纯静态。）

## 4. 功能：对齐 + 增强

### 4.1 对齐现有 Python 扫描器
- 检测四类，语义与标签完全一致：
  - `[ARG]`  —— `runtime = {` / `to_connect_vpc_ip_block = {` 这类 map 赋值 → 改 block 写法。
  - `[REF]`  —— `.attr["key"]` map 下标引用 → 改 `.attr[0].key`。
  - `[MODULE]` —— 引用已知受影响的 `terraform-alicloud-modules` 模块（rds / rds-mysql / rds-postgres / multi-zone-infrastructure-with-ots）。
  - `[PRESENT]`（信息）—— 出现受影响的 resource / data source。
- 递归扫描 `.tf`，自动跳过 `.terraform` / `.git` / `.idea` / `.vscode` / `node_modules`。
- 覆盖普通 HCL 与本地 module 子目录。
- 行内注释剥离（`#` / `//`），字符串内引号成对判断，降低误报。
- 属性引用回溯所属资源类型；无法确定时标 `[启发式/需人工确认]`（HIGH / MEDIUM 置信度）。
- 文本报告：**顶部先打印类别说明图例**，随后逐条 `文件:行号 · 资源/模块 · 字段 · 建议 · 代码`。
- `--json` 机器可读输出（字段：file/line/category/target/attr/confidence/message/code）。
- 退出码：发现需处理项返回 1，干净返回 0。

### 4.2 增强项（新增 flag）
- `--exclude <glob>`（可重复）：排除路径，默认内置忽略 `**/.claude/**`（避免扫到 skill 自带样例）。
- `--json` / `--format text|json`。
- `--no-color` / 自动检测 TTY；文本报告支持彩色分级（ARG/REF/MODULE 不同色）。
- `--fail-on none|module|ref|arg|any`：控制哪些类别影响退出码（默认 any）。
- `--quiet`：只输出汇总与明细，省略图例。
- `--version` / `-v`：打印版本（由 ldflags 注入）。
- `--help` / `-h`。
- 位置参数：一个或多个扫描路径（目录或文件），缺省为当前目录。

## 5. 项目结构

```
alicloud-v2-check/
├── go.mod
├── main.go                      # flag 解析、入口、退出码
├── internal/
│   ├── rules/
│   │   ├── rules.go             # 受影响资源/数据源/模块清单 + 属性映射（对应 Python 常量）
│   │   └── rules_test.go
│   ├── scanner/
│   │   ├── scanner.go           # 遍历文件、逐行匹配、生成 findings
│   │   └── scanner_test.go
│   └── report/
│       ├── report.go            # 文本图例 + 明细 + JSON 输出
│       └── report_test.go
├── testdata/                    # 从现有 examples/ 移植的 fixtures（含 negative/mixed/clean）
│   ├── plain-hcl/...
│   ├── modules/...
│   └── ...
├── .goreleaser.yaml             # 交叉编译矩阵、archive、checksums、release
├── .github/workflows/release.yml# push tag 触发 goreleaser
├── Makefile                     # build / build-all / test / lint / clean
├── README.md                    # 安装、用法、平台矩阵、退出码
└── docs/
    └── PLAN.md                  # 本文件
```

## 6. 核心实现要点（从 Python 移植）

- `rules.go`：
  - `AffectedResources map[string][]string`、`AffectedDataSources map[string][]string`
  - `BlockArgAttrs`（runtime, to_connect_vpc_ip_block）
  - `MapIndexAttrs`（domain_list, certificate_authority, connections, runtime, to_connect_vpc_ip_block, storage_range, gpu, burstable_instance, local_storage）
  - `AffectedModules`（4 个）
- `scanner.go`：预编译正则
  - `reBlockArg = ^\s*(runtime|to_connect_vpc_ip_block)\s*=\s*\{`
  - `reMapIndex = \.(<attrs>)\s*\[\s*"([^"]+)"\s*\]`
  - `reResourceDecl` / `reDataDecl` / `reSource` / `reTypeToken`
  - 维护当前所在 resource/data 块类型以判定 ARG 置信度；REF 回溯行内 `alicloud_*` token 判 HIGH/MEDIUM。
- `report.go`：文本图例常量 + 分类别分组输出 + JSON 序列化；退出码依据 `--fail-on`。
- 版本注入：`-ldflags "-X main.version=$(git describe --tags)"`。

## 7. 构建与分发

### Makefile（本地）
- `make build`：当前平台。
- `make build-all`：遍历平台矩阵，输出到 `dist/alicloud-v2-check_<os>_<arch>[.exe]`。
- `make test` / `make clean`。

### GoReleaser（`.goreleaser.yaml`）
- `builds`：`CGO_ENABLED=0`，goos/goarch 矩阵（排除 windows/arm64 视需要）。
- `archives`：tar.gz（*nix）/ zip（windows），命名 `{{.ProjectName}}_{{.Os}}_{{.Arch}}`。
- `checksum`：`checksums.txt`（sha256）。
- ldflags 注入 version/commit/date。

### GitHub Actions（`.github/workflows/release.yml`）
- 触发：`push` tag `v*`。
- 步骤：checkout → setup-go → `goreleaser release --clean`（用 `GITHUB_TOKEN`）。

## 8. 测试

- `go test ./...`：
  - rules/scanner：对 `testdata/` 断言每类命中数量、file:line、置信度。
  - 负向用例（already-v2 / 注释 / 无关 map 下标）断言 0 actionable。
  - report：JSON 结构、退出码逻辑、`--fail-on` 行为。
- 复用现有 skill 的 `examples/` fixtures 作为 `testdata/` 基线，结果与 Python 版对齐（24 处 for examples 集合）。

## 9. 里程碑

1. `go mod init` + `rules.go`（清单常量）+ 单测。
2. `scanner.go`（正则 + 遍历 + findings）+ 单测（对齐 Python 命中）。
3. `report.go`（文本图例/明细 + JSON + 退出码/`--fail-on`）+ 单测。
4. `main.go`（flag：paths/--json/--exclude/--no-color/--fail-on/--quiet/--version）。
5. `Makefile` + 本地 `build-all` 验证 6 个平台产物可生成。
6. `.goreleaser.yaml` + `release.yml` + `README.md`。
7. `goreleaser build --snapshot --clean` 本地干跑验证矩阵。

## 10. 验收标准

- `go build` 干净，无第三方依赖。
- 对 `workspace/example1..6` 扫描结果与 Python 版一致（ARG3 / REF17 / MODULE4 / PRESENT12）。
- `make build-all` 产出全部目标平台二进制；`file` 校验架构正确。
- `goreleaser build --snapshot` 成功产出归档 + checksums。
- `--help` / `--version` / `--json` / `--exclude` / `--fail-on` 行为符合文档。
- 全程「只读」：工具绝不写用户的 `.tf`。
```
