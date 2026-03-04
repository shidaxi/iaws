# iaws 需求说明（完善版）

## 1. 目标与约束

- **形态**：Go 编写的交互式 AWS CLI，通过 Terminal UI 完成所有参数选择，无需手写长 flag。
- **交互**：选择 + 关键词过滤（filter），支持常用命令的交互式执行。
- **认证**：自动读取 `~/.aws/config`，支持选择 profile（含 default）。
- **测试**：使用 LocalStack 做功能测试，不依赖真实 AWS 账号即可验证主流程。

## 2. 常用「查看类」操作枚举（按服务）

在「最常用服务的常用查看操作」前提下，建议覆盖：

| 服务 | 常用查看操作 | 说明 |
|------|--------------|------|
| **EC2** | 实例列表、VPC/子网/安全组/密钥对、卷、AMI 列表 | 已有「查看 EC2 信息」 |
| **SSM** | 参数列表、获取参数值；会话列表/历史 | 配合「SSM 登录 EC2」 |
| **Secrets Manager** | 密钥列表、GetSecretValue | 已有 |
| **S3** | 桶列表、对象列表、下载对象（GetObject） | 高频查看 |
| **RDS** | 实例列表、集群列表、快照列表 | 运维常用 |
| **Lambda** | 函数列表、函数详情/配置、别名与版本 | 无服务器常用 |
| **ECS** | 集群列表、服务列表、任务列表、任务定义 | 容器常用 |
| **EKS** | 集群列表、节点组、集群详情 | K8s 常用 |
| **IAM** | 用户/角色/策略列表、用户/角色详情 | 权限排查 |
| **CloudWatch** | 日志组/流列表、指标列表、告警列表 | 排障与监控 |

**第一期建议范围**：EC2 + SSM + Secrets Manager + S3。后续可按需扩展 RDS、Lambda、ECS、EKS、IAM、CloudWatch。

## 3. 「输入量很少的修改/写操作」枚举

在保持「交互为主、少打字」的前提下，建议支持的轻量写操作：

- **Secrets Manager**：PutSecretValue（单条 value，key 可选或固定）。
- **EC2**：实例 Start/Stop/Reboot（选择实例后一键）。
- **SSM**：SendCommand（可选常用文档如 `AWS-RunShellScript`，参数通过少量输入或选择）。
- **S3**：上传单个文件（选桶 + 选/输入 key + 选本地文件）。

## 4. 功能与非功能补充

- **Profile 选择**：启动时或全局设置中选择 profile，后续命令共用；支持 default 与具名 profile。
- **Region**：可从 profile 继承，或交互中选择 region（列出 config 中的 region、或常用 region 列表）。
- **SSM 登录 EC2**：先选 region/profile → 列实例（可 filter）→ 选实例 → 调 `aws ssm start-session`（或等价 API），本地需安装 Session Manager Plugin。
- **错误与权限**：命令失败时给出清晰错误信息；权限不足时提示可能缺失的 IAM 动作（如列出 min 权限建议）。
- **LocalStack**：核心路径（如 EC2 list、SSM list/get、Secrets Manager list/get/put、S3 list/get）在 LocalStack 可测；需在 README 或 docs 中说明如何用 LocalStack 跑测试。

## 5. 技术选型建议

- **Terminal UI**：Bubble Tea（bubbletea）+ lipgloss，支持列表、多选、输入框、filter。
- **AWS SDK**：aws-sdk-go-v2，与现有 `aws configure`/config 兼容。
- **配置**：读取 `~/.aws/config` 与 `~/.aws/credentials`（或环境变量），不自行实现 credential chain，使用 SDK 默认链即可。
