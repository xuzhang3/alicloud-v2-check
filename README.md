# alicloud-v2-check

扫描 Terraform HCL，找出升级 `aliyun/alicloud` provider `1.x → 2.0.0` 后会失效的写法，
并定位到 `文件:行号`。**纯 CLI、单静态二进制、零运行时依赖**，可按 `os/arch` 分发到各平台。

> 所有 v2 breaking change 本质都是属性从 **TypeMap → TypeList**。
> 本工具**只检查、只报告，绝不修改任何文件**。

## 安装

### 下载预编译二进制（推荐）

从 [Releases](../../releases) 下载对应平台的压缩包，解压后把 `alicloud-v2-check` 放进 `PATH`。

支持平台：`linux/amd64`、`linux/arm64`、`darwin/amd64`、`darwin/arm64`、`windows/amd64`、`windows/arm64`。

校验：
```bash
sha256sum -c checksums.txt
```

### 用 Go 安装
```bash
go install github.com/aliyun/alicloud-v2-check@latest
```

### 从源码构建
```bash
make build          # 产出 bin/alicloud-v2-check
make build-all      # 交叉编译所有平台到 dist/
```

## 用法

```bash
# 默认扫描当前工作空间（无需任何参数）
alicloud-v2-check

# 扫描指定目录 / 文件（可多个）
alicloud-v2-check ./infra ./modules/rds/main.tf

# JSON 输出（供 CI / 脚本消费）
alicloud-v2-check --json ./infra

# 排除路径（可重复；支持 **/dir/** 段匹配）
alicloud-v2-check --exclude '**/vendor/**' --exclude examples .

# 控制退出码策略
alicloud-v2-check --fail-on module .
```

不带任何路径参数时，**默认扫描当前目录（整个工作空间）**，并自动排除 `**/.claude/**`。

### 选项

| 选项 | 说明 |
|------|------|
| `--format text\|json` | 输出格式（默认 text） |
| `--json` | 等价于 `--format json` |
| `--exclude <glob>` | 排除路径，可重复；默认已内置 `**/.claude/**` |
| `--fail-on none\|module\|ref\|arg\|any` | 退出码策略（默认 `any`） |
| `--no-color` | 关闭彩色（非 TTY 自动关闭） |
| `--quiet` | 省略顶部类别说明图例 |
| `--version`, `-v` | 打印版本 |
| `--help`, `-h` | 帮助 |

### 退出码

| 码 | 含义 |
|----|------|
| 0 | 未发现需处理项（依据 `--fail-on`） |
| 1 | 发现需处理项 |
| 2 | 运行错误（路径不存在、参数非法等） |

CI 门禁示例：
```bash
alicloud-v2-check . || { echo "存在 alicloud v2 breaking change"; exit 1; }
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

## 说明

- 基于行级正则的「提醒 + 定位」工具，不是完整 HCL 解析器；最终以 `terraform plan` 为准。
- registry 模块内部代码不在工作空间，无法扫描，只能通过 `source` 识别已知受影响模块。
- 升级建议：先升到 `alicloud ~> 1.282.0` 且 `terraform plan` 无非预期变更，再升 `~> 2.0.0`，改完再跑 `terraform plan` 复核。
