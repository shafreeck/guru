
## Guru命令行工具使用说明

> 本文档由 ChatGPT 生成

### 命令

`guru`

### 参数

#### ChatGPTOptions

- `--chatgpt.model`: (可选, 默认值: `gpt-3.5-turbo`) 模型ID。有关详细信息，请参阅模型终结点兼容性表。
- `--chatgpt.temperature`: (可选, 默认值: `1`) 指代温度，范围在0到2之间。较高的值（例如0.8）会使输出更加随机，而较低的值（例如0.2）将使其更加集中和确定性。
- `--chatgpt.top_p`: (可选, 默认值: `1`) 在没有温度抽样的情况下，模型将考虑排名前几个的结果。因此，0.1意味着只考虑组成10％概率质量的标记。
- `--chatgpt.n`: (可选, 默认值: `1`) 为每个输入消息生成的聊天完成选择的数量。
- `--chatgpt.stop`: (可选) 最多有4个序列，其中API将停止生成更多标记。
- `--chatgpt.max_tokens`: (可选, 默认值: `0`) 聊天完成中要生成的最大标记数。
- `--chatgpt.presence_penalty`: (可选, 默认值: `0`) -2.0到2.0之间的数字。正值根据新标记在文本中是否出现惩罚新标记，从而增加模型谈论新话题的可能性。
- `--chatgpt.frequency_penalty`: (可选, 默认值: `0`) -2.0到2.0之间的数字。正值会根据其在文本中的现有频率惩罚新标记，从而减少模型重复相同行的可能性。

#### 总体选项

- `--openai-api-key`: OpenAI API密钥
- `--socks5`: (可选) Socks5代理
- `--timeout`: (可选, 默认值: `180s`) 联网超时
- `--interactive`: (可选, 默认值: `false`) 是否为命令行交互模式
- `--system`: (可选) 系统提示，用于初始化chatgpt
- `--file`: (可选) 发送完text后发送文件内容
- `--verbose`: (可选, 默认值: `false`) 输出详细信息
- `text`: (可选) 发送要问的文本

### 命令别名

可以使用以下命令别名替换完整命令:

- `review`: `chat --system 帮我Review以下代码,并给出优化意见,用Markdown渲染你的回应`
- `translate`: `chat --system 帮我翻译以下文本到中文,用Markdown渲染你的回应`
- `unittest`: `chat --system 为我指定的函数编写一个单元测试,用Markdown渲染你的回应`

### 交互模式

在交互模式下，您可以输入要发送的文本，并且ChatGPT将输出它的回复。执行命令启动交互模式。

### 非交互模式

您可以通过提供带文本的--text参数、文件内容的--file参数，或者将文本复制并粘贴到命令行中来开始ChatGPT交互。命令在完成对文本的分析后，将输出答案，并退出。

### 渲染Markdown

如果ChatGPT的响应包含Markdown，则应该在大多数情况下使用终端支持它的彩色终端。如果您的终端不方便字体，可以通过使用半图形字符将Markdown呈现为终端输出。

如果您想要可定制的Markdown呈现，您可能需要指定区域渲染。

### 例子

在交互式模式下启动guru的命令行聊天:

```
guru chat
```

使用api key设置:

```
guru chat --openai-api-key=<openai-api-key>
```

将text作为参数传递:

```
guru chat "What is the answer to the universe?"
```


获取ChatGPT关于用户输入文本的回答，并渲染Markdown：

```
guru chat "I need to improve my report based on the feedback I got. Can you give me some suggestions?" | my_markdown_render
```
