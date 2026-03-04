# iaws (Interactive AWS CLI)

基于 Go 的交互式 AWS CLI：无需拼接长 flag 与复杂参数，通过 Terminal UI 选择与过滤完成常用查看与少量写操作。

## 安装

```bash
git clone <repo>
cd iaws
go mod tidy
go build -o iaws .
# 或将 iaws 放到 PATH
```

## 依赖与配置

- **AWS 认证**：使用 `~/.aws/config` 与 `~/.aws/credentials`。运行 `iaws` 后先选择 **profile**（含 default），再选择 **region**，之后所有操作共用该 profile/region。
- **Assume Role + MFA**：若所选 profile 启用了 MFA（如 `role_arn` + `mfa_serial`），需在运行前设置环境变量 **`AWS_MFA_CODE`** 为当前 MFA 码，否则会报错。例如：`AWS_MFA_CODE=123456 ./iaws`。**凭证缓存在 `~/.aws/cli/cache/`**（与 AWS CLI 同目录），在有效期内再次运行 iaws 或 SSM 登录无需再次输入 MFA。
- **与 AWS CLI / kubectl 共享缓存**：iaws 使用 **`~/.aws/cli/cache/`** 存放 assume-role 缓存（文件名带 `iaws_` 前缀，不与 CLI 冲突）。若该目录下已有其他有效缓存（例如你先用 `aws` 输过 MFA），iaws 会优先复用；iaws 写入的缓存也可被同一目录下的其他工具共享。kubectl 访问 EKS 时通过 `aws eks get-token` 使用默认 AWS 凭证链，与 CLI/iaws 共用同一套凭证与缓存目录。
- **SSM 登录 EC2**：需在本地安装 [Session Manager Plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)，并确保实例已配置 SSM 与相应 IAM 权限。若出现 **「Plugin with name Standard_Stream not found」**，多为 **EC2 实例端** 资源问题：磁盘满、inotify 耗尽或打开文件数超限。可在实例上检查 `df -h`、`/proc/sys/fs/inotify/max_user_watches`，必要时扩容磁盘或提高 `fs.inotify.max_user_watches` / `fs.file-max` 后重启 SSM Agent 或实例；本地请将 Session Manager Plugin 更新至最新版。

## 使用示例

```bash
./iaws
# 1) 选择 profile（如 default）
# 2) 选择 region（如 us-east-1）
# 3) 主菜单选择 EC2 / SSM / Secrets Manager / S3
# 4) 在列表中可用键盘输入关键词过滤，↑/k ↓/j 移动，Enter 确认，Esc/q 返回
# 5) 危险操作（如 Stop 实例、Put Secret）会二次确认 (y/n)
```

## 主要特性

1. **全交互**：直接执行 `iaws`，所有参数与选项通过选择完成，无需手写长命令。
2. **Terminal UI**：通过选择或输入关键词过滤选项，交互式执行常用命令，例如：
   - 查看 EC2 信息（实例、VPC、子网、安全组等）
   - SSM 登录 EC2
   - 查看 Secrets Manager、Get/Put 单条 value
   - S3 桶与对象列表、下载/上传
3. **覆盖范围**：以最常用 AWS 服务的常用查看操作为主，以及少量输入很少的写操作（如 EC2 启停、Secret put、S3 单文件上传）。完整枚举见需求文档。
4. **认证**：自动读取 AWS config，支持选择 profile（含 default）。
5. **测试**：使用 LocalStack 做功能测试，不依赖真实 AWS 账号即可验证主流程。

## 文档

- **[需求说明（完善版）](docs/REQUIREMENTS.md)**：结构化需求、常用查看/写操作枚举、技术选型建议。
- **[实现提示词](docs/PROMPT.md)**：可直接用于开发或 AI 实现的完整提示词（目标、功能、交互、技术约束、交付与验收）。

## 第一期范围

EC2、SSM、Secrets Manager、S3 的常用查看与上述轻量写操作；后续可扩展 RDS、Lambda、ECS、EKS、IAM、CloudWatch 等。

## 使用 LocalStack 做功能测试

不依赖真实 AWS 账号即可验证 EC2 list、SSM list/get、Secrets Manager list/get/put、S3 list/get 等核心路径。

1. **启动 LocalStack**（例如用 Docker）：

```bash
docker run --rm -d -p 4566:4566 -p 4510-4559:4510-4559 localstack/localstack
```

2. **设置环境变量并运行 iaws**：

```bash
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_ENDPOINT_URL=http://localhost:4566
./iaws
```

3. 选择 profile（如 default）、region（如 `us-east-1`），即可在 LocalStack 上执行列表/获取等操作。如需预先写入测试数据，可使用 `awslocal` 或 AWS CLI 指向 `http://localhost:4566` 创建 EC2、SSM 参数、Secrets、S3 桶等后再在 iaws 中验证。
