# 实现计划 / 现状：alicloud-v2-check（跨平台 CLI）

> 本文档已更新为**当前实现状态**（features as built）。

## 1. 目标

纯 CLI 工具：单静态二进制、可按 `os/arch` 分发。扫描 Terraform HCL，检测升级
`aliyun/alicloud` provider `1.x → 2.0.0` 的 breaking change（本质都是属性
`TypeMap → TypeList`），定位到 `文件:行号`。**只检查、只报告，绝不修改用户文件。**

## 2. 技术栈（已实现）

- **语言**：Go（`CGO_ENABLED=0` 纯静态，`GOOS/GOARCH` 交叉编译）。
- **CLI 框架**：`spf13/cobra`（真实 flag、自动 help/version、子命令预留、补全能力）。
- **HCL 解析**：`hashicorp/hcl/v2`（`hclsyntax`）—— AST 引擎。
- **版本约束**：`hashicorp/go-version` —— 解析 provider version constraint。
- **分发**：GoReleaser + GitHub Actions（打 tag 自动出全平台包 + checksums + Release）。
- 依赖已从「零第三方」演进为「少量官方/生态标准库」，但仍是纯 Go、静态、交叉编译无障碍。

## 3. 目标平台矩阵（已验证）

linux/amd64、linux/arm64、darwin/amd64、darwin/arm64、windows/amd64、windows/arm64。
`make build-all` 与 `goreleaser build --snapshot` 均已产出并用 `file` 校验架构。

## 4. 功能（已实现）

### 4.1 检测四类（file:line 定位）
- `[ARG]`  map 赋值参数（`runtime`、`to_connect_vpc_ip_block`）→ 改 block 写法。
- `[REF]`  `.attr["key"]` map 下标引用 → 改 `.attr[0].key`。
- `[MODULE]` 引用已知受影响的 `terraform-alicloud-modules`（rds / rds-mysql / rds-postgres / multi-zone-infrastructure-with-ots）。
- `[PRESENT]`（信息）出现受影响的 resource/data source。
- 置信度：无法回溯资源类型的引用标 `[启发式/需人工确认]`（HIGH/MEDIUM）。
- 报告顶部先打印**类别说明图例**；底部给出**官方升级指南链接**。

### 4.2 双解析引擎（--engine auto|hcl|regex）
- `hcl`：官方 HCL AST，精确区分 `attr = {}` vs `attr {}`；只把**真正的变量引用**当作
  `.attr["k"]`，字符串/heredoc 字面量不会误报，多行正确。
- `regex`：逐行正则，对破碎/不完整 HCL 更宽容。
- `auto`（默认）：优先 HCL，**对解析失败的文件逐个回退**到 regex。
- 两引擎在合法 testdata 上结果一致（parity 测试保证）。

### 4.3 Provider 版本感知（v1/v2 适用，v3+ 跳过）
- 从 `terraform.required_providers.alicloud.version`（及 legacy `provider "alicloud"`）
  读取版本约束。
- 约束覆盖 v1/v2 → 适用，正常扫描；纯 v3+ → 判定不适用，**跳过并给出提示**（退出码 0）。
- `--ignore-version` 强制忽略该判定照常扫描。
- 检测到的约束会在报告头部列出（含 file:line）。

### 4.4 国际化 i18n（--lang zh|en）
- 报告全文（标题、图例、类别标题、字段标签、建议、汇总、版本提示、链接说明）均有
  中/英两套；findings 结构与逻辑语言无关，文本在 report 层本地化。
- 缺省从 `$LANG`/`$LC_ALL` 自动判定（zh* → 中文，否则英文）。
- JSON 输出的 `message` 字段同样按 `--lang` 本地化。

### 4.5 其它 flag（cobra）
`--format text|json`、`--json`、`--exclude <glob>`（可重复，默认内置 `**/.claude/**`）、
`--fail-on none|module|ref|arg|any`、`--no-color`（非 TTY 自动关闭）、`--quiet`、
`--version`、`--help`。缺省无路径参数时扫描当前工作空间。

### 4.6 退出码
- 0：无需处理项（依据 `--fail-on`）或版本判定为不适用而跳过。
- 1：发现需处理项。
- 2：参数非法 / 运行错误（路径不存在等）。

## 5. 项目结构（现状）

```
alicloud-v2-check/
├── go.mod / go.sum
├── main.go                      # cobra 根命令、flag、退出码；execute() 可测
├── tty.go                       # TTY 检测（彩色自动开关）
├── internal/
│   ├── rules/                   # 受影响资源/数据源/模块/属性清单 + 判定
│   ├── scanner/
│   │   ├── scanner.go           # 引擎选择、文件遍历、regex 引擎、Finding 结构
│   │   ├── hcl.go               # HCL AST 引擎
│   │   ├── scanner_test.go / hcl_test.go / integration_test.go
│   ├── report/
│   │   ├── report.go            # 文本/JSON 渲染、退出码策略、版本提示
│   │   ├── i18n.go              # zh/en 文案与本地化
│   │   └── report_test.go
│   └── tfversion/               # provider 版本约束检测 + 适用性判定 + 测试
├── testdata/                    # resources/datasources/modules/clean fixtures
├── .goreleaser.yaml / .github/workflows/{ci,release}.yml
├── Makefile / README.md / LICENSE
└── docs/PLAN.md
```

## 6. 构建与分发

- `make build` / `make build-all`（6 平台交叉编译到 dist/）。
- `.goreleaser.yaml`：`CGO_ENABLED=0`，goos×goarch 矩阵，tar.gz/zip，sha256 checksums，ldflags 注入版本。
- `.github/workflows/release.yml`：push `v*` tag → GoReleaser 自动发布；`ci.yml`：vet+test+gofmt 门禁。
- 版本 tag：当前 `v0.0.1`。

## 7. 测试（现状：全绿）

- `rules`：清单计数与判定。
- `scanner`：四类检测、负向用例、格式变体、exclude/dedup。
- `hcl`：heredoc 不误报、插值命中、解析错误 sentinel、auto 回退、hcl 跳过、**双引擎 parity**。
- `report`：图例/文件行、quiet、JSON 结构、退出码/`--fail-on`。
- `tfversion`：约束分类（v1/v2/v3）、required_providers 检测、缺省无约束。
- `main`（CLI）：help/version、JSON 退出码、fail-on、zh/en 输出、quiet、**版本 gating（v3 跳过 / --ignore-version 扫描）**。

## 8. 验收标准（已满足）

- `go build`/`go vet`/`gofmt` 干净；`go test ./...` 全绿。
- testdata 双引擎结果一致：ARG=3 / REF=9 / MODULE=4 / PRESENT=8。
- 6 平台交叉编译产物架构正确。
- `--engine` / `--lang` / `--fail-on` / `--ignore-version` / `--exclude` 行为符合文档。
- 全程只读，不写用户 `.tf`。

## 9. 后续可选项（未做）

- Homebrew tap / `install.sh` 一键安装。
- 子命令（如 `fix` 预览 diff —— 但需保持只读默认）。
- 更多受影响资源随官方指南更新。
- SARIF 输出以接入代码扫描平台。
