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

* 支持多轮对话或单轮对话，自动记录对话内容，且支持启动时通过 --last参数恢复上次会话
* 精细的上下文消息管理，可在 Token 超限时自动缩减待提交的消息列表，可查看、删除或手动缩减消息

* 支持管道重定向输入输出，可轻松用脚本进行二次封装
* 完善的命令自动补全，炫酷的输出动画效果，Markdown 渲染 ChatGPT 响应

* 支持添加提示词仓库，支持扮演任意提示词仓库预置的角色，内置 Cheatsheet 和 Committer，默认支持 [awesome-chatgpt-prompts](https://github.com/f/awesome-chatgpt-prompts), [awesome-chatgpt-prompts-zh](https://github.com/PlexPt/awesome-chatgpt-prompts-zh)（首次使用需同步），可添加自己的提示词仓库

* 会话恢复，会话新建，会话切换等常规会话管理
* 强大的会话栈机制，可逐层进入或退出子会话。当然，也具备会话新建、会话切换等基础能力

* 支持查看和动态修改内部参数，实时修改 API 提交参数

* 支持执行系统命令，并将命令输出提交给 ChatGPT
* 灵活的执行器机制，可直接执行 ChatGPT 返回的命令，并可选择将执行结果反哺给 ChatGPT


## 快速开始

### 安装

```
go install github.com/shafreeck/guru@latest
```

### 获取 OpenAI API Key

Guru 使用 OpenAI 的开放接口与 ChatGPT 交互，因此不受 Web 端体验产品屏蔽的影响，被官方的 Web 应用屏蔽了没有关系，登陆其开发平台，获取对应的 API Key 即可。

https://platform.openai.com/account/api-keys

需要注意的是，调用 OpenAI 接口是收费的，新注册的账户默认有 5 美元可用，由于 ChatGPT 的接口非常便宜，5 美元可以用很长时间了（在我频繁调试的情况下，目前一共用了 2.4 美元）。如果你的账户注册的稍微早一点，默认就有 18 美元可用。大家可以登陆自行查看一下自己的账户余额 ：

https://platform.openai.com/account/usage

PS：赠送的账户余额是有期限的，能用就赶紧用哈。

### 配置 Guru 

为了便于大家使用，特定添加了交互式配置的功能，Guru 会默认从配置文件和命令行参数来获取需要的参数，如果没有配置的话，也可以通过命令行参数直接指定。由于 API Key 的私密性，还是建议将其写到配置文件。

![guru-config-cropped](https://user-images.githubusercontent.com/418483/230640993-2c50e9e5-f015-4520-95b6-ee3cdb92936e.gif)

也可以跳过配置，直接命令行参数指定相关选项。

## 使用指南

### 多轮对话

```
> guru [text]
```

执行 `guru` 直接进入多轮对话模式，`guru` 实际是 `guru chat` 命令的别名。`--oneshot` 参数可以进入单轮对话模式，单轮对话模式下，上下文自动丢弃。`--last` 参数可恢复上次对话。会话记录自动存储（默认为 ~/.guru/session/ 目录下）。

![chat](https://user-images.githubusercontent.com/418483/230428335-5e52561c-efb8-4425-a015-2a737491f83e.gif)

### 内置提示词

#### 使用 Cheatsheet

```
> guru cheat
```

`guru cheat` 本质上是 `guru chat -p Cheatsheet` 命令的别名，用来简化用户输入。

![cheat](https://user-images.githubusercontent.com/418483/230428209-0fb10754-a501-4cc1-b807-d3c6e0502c37.gif)

#### 使用 Committer

```
> git diff | guru commit
```
`guru commit` 是 `guru chat -p Committer` 的别名，用来简化用户输入。


![640](https://user-images.githubusercontent.com/418483/231173798-4d0d4f37-9343-407e-8cf5-c43f3ead52db.gif)

### 单轮对话
`--oneshot` 参数可以进入单轮对话，单轮对话模式下，上下文消息会被自动丢弃。但是，如果 `--prompt, -p` 指定了提示词，则提示词的内容会被固定，跟随每次对话提交。

> 使用 :message pin 命令可将任意消息固定

```
> guru --oneshot
```


### 重定向输入输出

```
echo list files in this dir in detail | guru cheat | sh
```

![guru-cheat-output](https://user-images.githubusercontent.com/418483/230641057-68721bad-1ee6-4d9b-a614-07034a504dc3.gif)


### 消息管理

ChatGPT 并不会在服务端保存对话的上下文，其上下文感知能力，是通过客户端每次提交对话记录来实现的。在 ChatGPT 接口的定义中，一个发起的问题，和一个回答的内容，都叫做一个消息（Message）。由于 ChatGPT 接口对消息内容的总大小限制为 4096 个 Token，因此，持续的对话会导致消息大小超过限制。

Guru 支持自动清理旧的消息，以滚动窗口的方式实现持续的对话。但是，有时我们期望更精细的控制提交到 ChatGPT 的内容，从而精确控制上下文，这时候，可使用消息管理的内部命令对消息记录进行手动缩减，删除，或追加等。

对于不想删除，或不想被滚动窗口清理的消息，我们可以通过 :message pin命令，将消息钉住。单轮对话的oneshot机制，即使用这个功能将提示词消息固定，实现每次对话都会提交提示词的功能。

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

### 会话管理

每次启动 `guru`，都会自动创建一个会话，会话内容默认保存在 `~/.guru/session/` 目录下。启动时，可通过 `--session-id, -s` 指定某个会话 id，或者通过 `--last` 恢复上次对话。指定的会话 id 如果不存在，则自动创建。

会话管理的功能非常丰富。我们可以在同一个 guru交互过程中新建、切换会话。其中最具实用特色的功能是会话栈，实现在不中断当前会话的前提下，逐层进入子会话。会话的连续性非常重要，比如，在对一篇论文的多轮对话中，我期望对话内容都被记录，并在以后回顾的时候，可以看到清晰的对话脉络。guru后面会支持会话导出的功能。

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

#### 会话栈的应用

* `>` 是一个特殊的命令，本质上是 `:session stack push` 命令的别名，当执行 `>` 时，进入到新的会话，并将会话入栈，这时命令提示符会追加 ">" 符号，比如变为 `guru >> `

* `<` 是 `:session stack pop` 的别名，执行后，栈顶的会话弹出，命令提示符会撤销追加的 ">" 符号

> 注意：目前只有 `>` `<` 命令才能对命令提示符产生效果。直接执行 `:session stack push/pop` 并没有这样的效果，后续将会完善这个机制。

### 提示词管理

`prompt repo` 相关命令，可以添加或同步提示词仓库，目前默认支持  awesome-chatgpt-prompts, awesome-chatgpt-prompts-zh 两个比较优质的仓库，用户可自行添加自己喜欢的仓库。

需要注意，初次使用时，除了内置的 `Cheatsheet`, `Committer` 提示词外，其他远程仓库的提示词，都需要执行 `:prompt repo sync` 进行同步才能使用。同步到本地的提示词文件默认存储在 `~/.guru/prompt/` 目录下。

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

```
guru > :act as Linux Terminal
```

![actas](https://user-images.githubusercontent.com/418483/230641109-a27a339a-3c98-49c8-a298-e6cc78ad5dc6.gif)


### 执行系统命令
`$` 后可以接系统命令，命令的输出结果会在下轮对话中提交到 ChatGPT，这在我们需要载入文件的时候，非常有用。

![640](https://user-images.githubusercontent.com/418483/231177107-61b8bc2e-08b4-4dfc-87be-5d2219d20e97.png)

`$` 不接任何命令时，会进入 `shell` 模式，此时命令提示符变为 `guru $ `，这个模式下输入的任何命令，都跟在 `shell` 中一样执行，最终所有的输出，会在下轮对话中，提交到 ChatGPT。

`shell` 模式下，输入 `>` 则重新回到对话模式。

### 查看或设置参数

* :info 查看内部信息
* :set 设置内部参数 

```
dir                           /Users/shafreeck/.guru
filename
api-key                sk-************************************************
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

执行器是 `guru` 最强大的和最独特的功能，启动 `guru` 时，通过 `--executor, -e` 参数指定执行器。`guru` 在每次对话后，都会将 ChatGPT 的输出内容，通过标准输入，交由执行器处理。如果指定了 `--feedback` 参数，则执行器的执行结果，也会反哺回 ChatGPT。

执行器跟上面提交的执行系统命令不同，执行系统命令只是通过 `shell` 的方式，丰富数据输入的手段。而执行器则是用来处理 ChatGPT 的输出。实现了 `输入`-> `输出`-> `再输入`的完整闭环。这意味着我们可以在对话的流程中，通过命令的方式，对沟通消息进行任意处理。

为了安全起见，每次执行器的调用，都需要用户确认。

#### shell 作为执行器

最简单的使用场景，将 ChatGPT 返回的命令，交由 `shell` 执行

```
> guru cheat -e sh
```

![executor](https://user-images.githubusercontent.com/418483/230641197-21ca91d6-e2f1-44c4-987e-1b5f72813f60.gif)

#### 进阶玩法：实现 ChatGPT 自己与自己对话

可通过一下命令实现 ChatGPT 之间的对话。

```
> guru -e "guru --dir ./conversation --last" --feedback Hi
```
自我对话的原理，是将另一个 `guru` 作为执行器，第二个 `guru` 的启动参数，设置 `--dir` 为自己独立的目录，以避免跟第一个 `guru` 混淆，设置 `--last` 每次启动时回复会话，从而维持对话的上下文。

![640](https://user-images.githubusercontent.com/418483/231178718-1be2b35b-5d9f-44e2-8bac-1b274269eb89.gif)
