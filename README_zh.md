# Guru

在终端体验 ChatGPT ！[介绍文章](https://mp.weixin.qq.com/s/3dFncTQlF0QLJVhXa31oIQ)

```
Guru 的含义是一个受人尊敬的导师、顾问或精神领袖，通常在某个领域具有专业的知
识、技能和经验，并能够传授、启发、引领其弟子或追随者去追求他们自己的目标和
理想，帮助他人实现成长和成功。这个词源于印度教和佛教传统中的做法，受到广泛
的尊敬和崇拜。在现代社会中，guru通常用于描述领袖、思想家、企业家、教育家、艺
术导师等具有影响力和指导作用的人士。  --- 由 ChatGPT 生成
```

## 功能

* 连续对话或单次问答两种模式，默认为连续对话模式
* 精细控制提交给 ChatGPT 的上下文信息，可查看、缩减、删除等

* 支持管道重定向标准输入、标准输出
* Markdown 渲染 ChatGPT 响应内容
* 完善的自动补全和动画效果

* 可扮演任意提示词指定的角色，内置 Cheatsheet 和 Committer
* 提示词仓库管理，默认支持 [awesome-chatgpt-prompts](https://github.com/f/awesome-chatgpt-prompts), 
[awesome-chatgpt-prompts-zh](https://github.com/PlexPt/awesome-chatgpt-prompts-zh)，可添加自己的提示词仓库

* 会话恢复，会话新建，会话切换等常规会话管理
* 特有的会话栈机制，中断当前会话并进入新的会话，可随时回到当前会话

* 动态修改 Guru 内部变量，ChatGPT 接口参数等

* 支持执行系统命令，并将命令输出提交给 ChatGPT
* 特有的执行器机制，可将 ChatGPT 返回的命令交给执行器执行，比如 sh


## 快速开始

### 安装

```
go install github.com/shafreeck/guru@latest
```

### 获取 OpenAI API Key

登陆自己的 OpenAI 账号，并生成 API Key：https://platform.openai.com/account/api-keys

### 配置 Guru 
![guru-config-cropped](https://user-images.githubusercontent.com/418483/230640993-2c50e9e5-f015-4520-95b6-ee3cdb92936e.gif)

也可以跳过配置，直接命令行参数指定相关选项。

## 使用方法

### 连续对话

```
guru [text]
```

执行 `guru` 进入连续对话模式，对话内容会自动记录（默认为 ~/.guru/session/ 目录下）。

![chat](https://user-images.githubusercontent.com/418483/230428335-5e52561c-efb8-4425-a015-2a737491f83e.gif)

### 使用内置的 Cheatsheet

```
guru cheat
```

`guru cheat` 本质上是 `guru chat -p Cheatsheet` 命令的别名。即使用 Cheatsheet 这个 guru 内置的提示词。

![cheat](https://user-images.githubusercontent.com/418483/230428209-0fb10754-a501-4cc1-b807-d3c6e0502c37.gif)

### 单次问答

```
guru --oneshot
```

指定 `--oneshot` 选项进入单次问答模式，这个模式下，每次只提交当前输入的内容，自动丢弃上下文。但是如果指定了 `-p` 提示词，则提示词消息会在每次问答时自动提交。

> 内部命令 `:ls` 可查看当前提交的消息。


### 输入输出重定向

```
echo list files in this dir in detail | guru cheat | sh
```

![guru-cheat-output](https://user-images.githubusercontent.com/418483/230641057-68721bad-1ee6-4d9b-a614-07034a504dc3.gif)


### 消息管理

向 ChatGPT 的内容，以及其返回的内容，称为一条消息(Message)，通过内部 `:message` 命令可以管理当前要提交给 ChatGPT 的消息。

```
guru > :message
Available commands:

:message list                 list messages
:message delete               delete messages
:message shrink               shrink messages
:message show                 show certain messages
:message append               append a message
:message pin                  pin messages
:message unpin                unpin messages
```

* `:message list` 列出当前所有的消息，也可以用简短的别名 `:ls`
* `:message delete [id...]` 删除消息，参数为消息 id，可同时删除多条消息
* `:message shrink [expr]` 缩减消息，`expr` 为范围表达式，其与 Go 语言中 Slice 的表达是相同: `begin:end`。其中，begin 或 end 可省略，比如 `5:`，代表保留 id 大于等于 5 的所有消息。
* `message show [id]` 显示某条消息，并使用 Markdown 渲染，默认显示最后一条消息
* `message append` 追加一条消息，也可使用简短的别名 `:append`
* `message pin [id]` 钉住某条消息，钉的的消息不回被消息自动收缩机制删除，也无法被 `:message delete` 命令删除。
* `message unpin [id]` 解除消息的钉住状态

### 使用 Prompt

```
guru > :prompt
Available commands:

:prompt act as                act as a role
:prompt list                  list all prompts
:prompt repo sync             sync prompts with remote repos
:prompt repo add              add a remote repo
:prompt repo list             list remote repos


Alias commands:

:prompts                      alias :prompts = :prompt list
```

`:prompt` 相关命令可以使用 awesome-chatgpt-prompts 设置的提示词，可以添加并同步自定义的提示词仓库。

* `prompt act as` 充当提示词设定的角色，可使用简短命令别名 `:act as`
* `prompt list` 列出所有载入的提示词信息，可使用简短命令别名 `:prompts`
* `prompt repo add/sync/list` 添加、同步、列出提示词仓库

#### 充当 Linux Terminal

![actas](https://user-images.githubusercontent.com/418483/230641109-a27a339a-3c98-49c8-a298-e6cc78ad5dc6.gif)

### 会话管理

`guru` 启动时默认开启新的会话，可通过 `--last` 参数恢复上次会话，也可以直接 `--session-id <id>` 指定会话 id，打开某个会话。

```
guru > :session
Available commands:

:session new                  create a new session
:session remove               delete a session
:session shrink               shrink sessions
:session list                 list sessions
:session switch               switch a session
:session history              print history of current session
:session stack                show the session stack
:session stack push           create a new session, and stash the current
:session stack pop            pop out current session
```

* `:session new` 新建会话，也可使用简短命令别名 `:new`
* `:session remove [sid]` 删除会话
* `:session shrink [expr]` 缩减会话，`expr` 是范围表达式，使用方法跟 `:message shrink` 命令相同
* `:session list` 列出所有会话，其中当前会话会通过 `*` 标出
* `:session switch [sid]` 切换会话
* `:session history` 显示会话历史
* `:session stack` 显示会话栈状态，可使用简短命令别名 `:stack`
* `:session stack push` 新建会话并入栈，可使用简短命令别名 `>`
* `:session stack pop` 当前会话出栈，可使用简短命令别名 `<`

当使用 `>` 新建会话时，会自动在命令提示符追加 `>` 符号，比如 由`guru> ` 变为 `guru >> `，使用出栈命令 `<` 可取消追加的符号。


### 查看或设置参数

* :info 查看内部信息
* :set 设置内部参数 

```
dir                           /Users/shafreeck/.guru
filename
openai-api-key                sk-************************************************
pin                           false
prompt
session-id                    chat-1680879639912-1ec4e509-af5b-4abb-9f4b-bebde2276d96
socks5                        localhost:8804
stdin                         false
timeout                       3m0s
------------------------------
chatgpt.frequency_penalty     0
chatgpt.max_tokens            0
chatgpt.model                 gpt-3.5-turbo
chatgpt.n                     1
chatgpt.presence_penalty      0
chatgpt.stop
chatgpt.stream                true
chatgpt.temperature           1
chatgpt.top_p                 1
chatgpt.user
disable-auto-shrink           false
executor
feedback                      false
non-interactive               false
oneshot                       false
system
verbose                       false
```

```
:set chatgpt.temperature 1.5
```

### 执行器

执行器是任意支持标准输入的命令，guru 将 ChatGPT 返回的内容，通过标准输入提交到执行器，并在此之前询问是否执行

`--executor` 参数用于指定执行器，另外 `--feedback` 参数用于指定是否将执行器的执行结果重新提交到 ChatGPT。

![executor](https://user-images.githubusercontent.com/418483/230641197-21ca91d6-e2f1-44c4-987e-1b5f72813f60.gif)

#### 进阶玩法

可通过一下命令实现 ChatGPT 之间的对话。

`guru -e "guru --dir ./conversation --last" --feedback`

`-e` 参数指定执行器为 `guru --dir ./chat --last`, 第一个 guru 返回的内容，
会通过标准输入给到第二个 guru，第二个 guru 指定了一个独立的工作目录，将会话数据跟第一个 guru 区分。
`--last` 指定每次启动时恢复会话，从而实现具备上下文对话的能力。`--feedback` 则表示将第二个 guru 
的输出内容重新提交回第一个 guru。这样便可以实现连续对话。

![chat-with-self](https://user-images.githubusercontent.com/418483/230642986-03c73ed9-2cc5-4f06-bc70-d8beb8437a6b.jpg)
