# config 模块说明（配置管理）

> 所属目录：`config/`
> 架构总览见 [架构设计](../架构设计.md) | 使用见 [使用指南](../使用指南.md)

## 职责

从 YAML 加载配置，支持**文件合并、环境变量覆盖、目录热重载**，并提供类型安全的取值访问。

## 核心功能

### 1. `Load(configFile)` —— 静态加载

1. 解析入口文件，支持 `include` 指令**递归合并**多个 YAML 文件（include 本身不参与业务配置）
2. 合并结果反序列化为 `Config` 结构体，**默认值兜底**（server `:8080` / db sqlite / log info）
3. 环境变量覆盖（见下表）

**优先级**：默认值 < 文件 < 环境变量。

### 2. 配置结构

| 节 | 字段 | 环境变量覆盖 |
|---|---|---|
| `server` | addr / read_timeout / write_timeout | `APP_ADDR` / `APP_READ_TIMEOUT` / `APP_WRITE_TIMEOUT` |
| `database` | driver / dsn | `APP_DB_DRIVER` / `APP_DB_DSN` |
| `log` | level / format / console / dir / filename / max_age / caller | `APP_LOG_*` |

### 3. 冲突显式失败

合并时遇到**重复 key 直接 panic**：配置冲突宁可启动失败，不静默覆盖（设计原则：失败要显式）。

### 4. `Watcher` —— 热重载

- 基于 fsnotify 监听整个配置目录，任一 `.yaml`/`.yml` 写入或创建触发重载
- `Watch("server.addr")` 按**点路径订阅**，值变化时向 channel 推送新值
- `Get(path)` 同步读取当前值，返回 `Value` 类型

### 5. `Value` —— 类型安全取值

链式 API：`Get("params.timeout").Int(30)`、`.String()`、`.Float64()`、`.Bool()`，均支持默认值；`Exists()` 判断路径是否存在。

## 与其他模块的关系

- **零依赖**：只依赖第三方 `gopkg.in/yaml.v3` 与 `github.com/fsnotify/fsnotify`
- 独立于 logger / framework，日志配置只是 `Config.Log` 的载体，由业务代码自行把配置传给 `logger.New`

## 边界 / 刻意不做

- 注释宣称"命令行 flag 覆盖"，**当前版本尚未实现**（实际优先级为 文件 < 环境变量）
- 热重载**不处理 include** 指令（静态加载已检查 include，热加载按目录平铺合并）

## 相关文档

- [架构设计](../架构设计.md)：§七 设计原则（失败要显式）
- [使用指南](../使用指南.md)：八、配置
