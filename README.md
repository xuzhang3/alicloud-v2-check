# alicloud-v2-check

扫描 Terraform HCL，找出升级 `aliyun/alicloud` provider `1.x → 2.0.0` 后会失效的写法，
并定位到 `文件:行号`。**纯 CLI、单静态二进制、零运行时依赖**，可按 `os/arch` 分发到各平台。

> 所有 v2 breaking change 本质都是属性从 **TypeMap → TypeList**。
> 本工具**只检查、只报告，绝不修改任何文件**。

## 安装

### 下载预编译二进制（推荐）

从 [Releases](../../releases) 下载对应平台的压缩包，解压后把 `alicloud-v2-check` 放进 `PATH`。

支持平台：`linux/amd64`、`linux/arm64`、`linux/386`、`darwin/amd64`、`darwin/arm64`、`windows/amd64`、`windows/arm64`、`windows/386`。

校验：
```bash
sha256sum -c checksums.txt
```

### 用 Go 安装
```bash
go install github.com/xuzhang3/alicloud-v2-check@latest
```

### 从源码构建
```bash
make build          # 产出 bin/alicloud-v2-check
make build-all      # 交叉编译所有平台到 dist/
```

## 用法

```bash
# 默认扫描当前工作空间（无需任何参数），输出 Markdown 格式
alicloud-v2-check

# 扫描指定目录 / 文件（可多个）
alicloud-v2-check ./infra ./modules/rds/main.tf

# 导出 Markdown 报告到文件（贴 PR / wiki / issue）
alicloud-v2-check -o v2-report.md ./infra

# 输出纯文本格式
alicloud-v2-check --format text ./infra

# 排除路径（可重复；支持 **/dir/** 段匹配）
alicloud-v2-check --exclude '**/vendor/**' --exclude examples .

# 控制退出码策略
alicloud-v2-check --fail-on module .
```

不带任何路径参数时，**默认扫描当前目录（整个工作空间）**，并自动排除 `.terraform`、`.git`、`.idea`、`.vscode`、`node_modules` 等目录。

### 选项

| 选项 | 说明 |
|------|------|
| `--format text\|markdown` | 输出格式（默认 markdown） |
| `--output`, `-o <file>` | 写入文件而非 stdout |
| `--engine auto\|hcl\|regex` | 解析引擎（默认 auto） |
| `--lang zh\|en` | 输出语言（默认按 `$LANG` 自动判定） |
| `--exclude <glob>` | 排除路径，可重复 |
| `--fail-on none\|module\|ref\|arg\|any` | 退出码策略（默认 `any`） |
| `--group-by category\|resource` | 报告分组方式（默认 resource）；resource 按资源名归类 |
| `--ignore-version` | 即使 provider 约束指向 v3+ 也照常扫描 |
| `--no-color` | 关闭彩色（非 TTY 自动关闭） |
| `--version` | 打印版本 |
| `--help`, `-h` | 帮助 |

### 退出码

| 码 | 含义 |
|----|------|
| 0 | 未发现需处理项（依据 `--fail-on`） |
| 1 | 发现需处理项 |
| 2 | 运行错误（路径不存在、参数非法等） |

CI 门禁示例：
```bash
alicloud-v2-check --format text . || { echo "存在 alicloud v2 breaking change"; exit 1; }
```

## 检测类别

| 标签 | 含义 | 改法 |
|------|------|------|
| `ARG` | map 赋值参数（`runtime`、`to_connect_vpc_ip_block`） | `attr = { ... }` → `attr { ... }` |
| `REF` | map 下标引用 | `x.attr["key"]` → `x.attr[0].key` |
| `MODULE` | 引用了已知受影响的 `terraform-alicloud-modules` 模块 | 升级模块版本并核对其 output 引用 |
| `PRESENT` | （信息）出现受影响的资源/数据源 | 升级后核对其 map→list 属性 |

标注 `[启发式/需人工确认]` 的项无法从行内回溯到具体 `alicloud_*` 资源类型（如 module output、locals），需人工判断。

## 覆盖的资源 / 数据源 / 模块

- **Resources**：`alicloud_api_gateway_instance`、`alicloud_cr_repo`、`alicloud_cs_edge_kubernetes`、`alicloud_cs_kubernetes`、`alicloud_cs_managed_kubernetes`
- **Data Sources**：`alicloud_cr_repos`、`alicloud_cs_cluster_credential`、`alicloud_cs_edge_kubernetes_clusters`、`alicloud_cs_kubernetes_clusters`、`alicloud_cs_managed_kubernetes_clusters`、`alicloud_cs_serverless_kubernetes_clusters`、`alicloud_db_instance_classes`、`alicloud_instance_types`
- **Modules**：`terraform-alicloud-modules/{rds, rds-mysql, rds-postgres, multi-zone-infrastructure-with-ots}/alicloud`

## 开发

```bash
make test      # go test ./...
make vet
make snapshot  # goreleaser 本地干跑（不发布）
```

## 解析引擎（--engine）

| 引擎 | 说明 |
|------|------|
| `hcl` | 用官方 HashiCorp HCL 解析器（`hashicorp/hcl/v2`）构建 AST 后过滤。能精确区分 `attr = {}` 与 `attr {}`，并且只把**真正的变量引用**当作 `.attr["k"]` —— 字符串 / heredoc 字面量里的同形文本不会误报，多行也正确。 |
| `regex` | 逐行正则。对**破碎 / 不完整**的 HCL 更宽容（HCL 解析失败的文件它仍能扫）。 |
| `auto`（默认） | 优先用 `hcl`；对无法解析的文件**逐个回退**到 `regex`。兼得精度与容错。 |

示例：一段 heredoc 文档里贴了旧写法代码片段，`regex` 会误报，`hcl` 不会：
```bash
alicloud-v2-check --engine hcl  ./infra   # 干净
alicloud-v2-check --engine regex ./infra  # 可能多报 heredoc 里的示例
```

## Provider 版本感知

工具会读取 `terraform.required_providers.alicloud.version` 约束：
- 约束覆盖 **v1/v2** → 适用，正常扫描。
- 纯 **v3+** → 本次 1.x→2.0 检查不适用，**跳过并提示**（退出码 0）；用 `--ignore-version` 可强制扫描。
- 未声明约束 → 照常扫描。

## 多语言 i18n

`--lang zh|en` 切换中英文报告（缺省按 `$LANG` 自动判定）。

## 说明

- 默认 `auto` 引擎基于官方 HCL AST，精度高；`regex` 引擎是「提醒 + 定位」的兜底。最终仍以 `terraform plan` 为准。
- registry 模块内部代码不在工作空间，无法扫描，只能通过 `source` 识别已知受影响模块。
- 升级建议：先升到 v1 最新版本， 如 `v1.286.0` 且 `terraform plan` 无非预期变更，再升 `~> 2.0.0`，改完再跑 `terraform plan` 复核。
