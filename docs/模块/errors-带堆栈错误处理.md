# errors 模块说明（带堆栈错误处理）

> 所属目录：`errors/`
> 架构总览见 [架构设计](../架构设计.md) | 使用见 [使用指南](../使用指南.md)

## 职责

把"错误发生在哪"变成一等公民：业务代码**直接替换标准 errors 包导入**（`import "github.com/memory198/go-gear/errors"`）即可获得调用堆栈能力，同时保持与标准库的 `errors.Is` / `errors.As` / `errors.Unwrap` 完全兼容。

## 核心设计

- **`withStack{msg, cause, stack}`**：包装错误时记录调用堆栈；`Unwrap()` 透传 cause，因此标准错误链的查找、比较逻辑沿链穿透，不破坏兼容性
- **帧过滤**：捕获堆栈时过滤 `runtime` 与 errors 包自身的帧，只保留业务调用点；深度限 8，避免噪音
- **`fmt.Sprintf("%+v", err)`**：打印完整错误链，最底层原因先出，每层带 `at file:line (func)`；`%s` / `%v` 只打印消息
- **防重复包裹**：`From` 检测到已是项目 error 时直接返回原值，避免堆栈叠加

## API 速览

| 函数 | 作用 |
|---|---|
| `Errorf(format, args...)` | 创建新错误并记录当前堆栈（无 cause） |
| `Wrap(err, msg)` / `Wrapf` | 包裹错误 + 附加本层描述 + 记录堆栈 |
| `WithStack(err)` | 只记录堆栈，不加描述（标记透传点） |
| `From(err)` | 外部错误转项目错误，堆栈记在转换处；已是项目错误则原样返回 |
| `FromMsg(err, msg)` | From + 附加描述 |
| `Stack(err)` | 获取完整堆栈字符串（等价 `%+v`） |
| `Is` / `As` / `Unwrap` / `New` | 标准库透传，业务代码无需改导入习惯 |

## 与其他模块的关系

- **零依赖**：不依赖任何项目包，可单独抽取使用
- **与 framework 的 BizError**：两者互不感知，但 `framework.WriteError` 用 `errors.As` 查找 `BizError`，带堆栈的包装不阻碍穿透——业务代码 `errors.Wrap(bizErr, "...")` 后仍能被正确识别

## 边界 / 刻意不做

- 不定义业务错误码（那是 framework `BizError` 的职责），只负责堆栈与错误链
- 堆栈帧深度限制为 8 层，深调用链只保留外层关键帧

## 相关文档

- [架构设计](../架构设计.md)：§五 trace 链路、§七 设计原则
- [使用指南](../使用指南.md)：五、Errors 用法
