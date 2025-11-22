# goTranscriber

GoTranscriber启发于pyTranscriber

由于pyTranscriber极其低效的音频转换效率和高额内存占用。

我使用Golang重塑了pyTranscriber

goTranscriber在处理一个两小时的视频的时候，仅需要100MB内存的占用，相比于pyTranscriber超过1GB内存占用，有了较大的提升。

同时，更换pyTranscriber中的使用计算音频RMS(Root Mean Sqrt)获取声强方式，goTranscriber默认采用16000Hz的WebRTC VAD算法进行检测声音区域，更加精准。

当然，经过一个星期的测试，goTranscriber拥有极佳的稳定性，并不会像pyTranscriber一样随便崩溃。

# 基本设计

`视频` -> `FFmpeg预处理音频` -> `WebRTC VAD (保守模式)识别人声区域` -> `切片` -> `多线程上传Google Speech-To-Text API` -> `翻译` -> `生成SRT文件`

# 问题

1. `WebRTC VAD` 识别正确性很高，但是切片会原本是同一句话硬切成两份，这是因为`Google Speech-To-Text API`最多容许10秒（实测，官方说12秒）音频片段。
   目前补救手段是，通过分析停顿来优化切分，避免一句话被切成碎片。尽管听起来可靠，但遇到有BGM人声区域，还是会乱切。
3. `BGM`会一定程度上影响识别

# 后续开发
上述问题，其实很难解决。因为，即使把vad切片准确性优化到极致，依然无法避免10秒切片带来的上下文丢失

后续，这个项目就此终止，其替代品我推荐：[faster-whisper](https://github.com/SYSTRAN/faster-whisper)

faster-whisper对whisper进行了优化，而且引入基于机器学习的Silero Vad进行人声区域切片避免上下文过长和引入一些无关片段。

我试着用了下，除了速度比goTranscriber慢很多，其效果是相当好的。

我想，goTranscriber其实已经可以就此停止了。如果追求速度，可以用goTranscriber，其速度是相当快的，一个2小时的视频大概需要30-45分钟就可以完成，而且对于使用者算力并无要求。

但如果你有一定算力，想要更好的质量，那我推荐faster-whisper。

# 安装教程

依赖要求：`ffmpeg`, `go`, `gcc / clang`(编译WebRTC VAD，TODO：使用Go重写，这样就不需要编译器了)

## Linux(Debian为例)

```
apt install gcc g++ ffmpeg -y
wget https://go.dev/dl/go1.19.3.linux-amd64.tar.gz && rm -rf /usr/local/go && tar -C /usr/local -xzf go1.19.3.linux-amd64.tar.gz
git clone https://github.com/MeteorsLiu/goTranscriber.git
cd goTranscriber
/usr/local/go/bin/go build 
./goSRT -file xxx -lang xx
```

# goTranscriber使用
`-translate`，是否翻译成中文，默认true，也就是默认翻译

`-vad`, 切片人声区域识别引擎参数，默认：WebRTC VAD保守模式，可选：energy(基于声音能量比例分析), webrtcpause(WebRTC VAD激进模式+停顿分析)

`-concurrency`，听识并发数量，默认10

`-lang`, 视频源语言，关于如何填写可以参考[源代码](https://github.com/MeteorsLiu/goTranscriber/blob/60df26a27ab35e71b01f68f4311f326effee9396/transcribe/transcribe.go#L37)

`-file`，视频文件地址

## 输出
会输出SRT文件到视频地址目录

假设视频文件叫: `xxx.mp4`
格式如下：`xxx_webrtc.srt`，如果已经存在，goTranscriber默认不对文件进行覆盖，而是会在后面加一些随机数 `xxx_webrtc_123456.srt`

`webrtc`后缀代表识别引擎

## 关于不同人声识别引擎选用
默认WebRTC VAD保守模式适合多数场景，但如果背景有嘈杂BGM，效果较差，此类场景推荐energy或者webrtcpause。

实测webrtcpause效果要优于energy（多数情况下）

## 为什么会默认输出
因为`goTranscriber`使用场景就是批量化自动输出

# Whisper使用

推荐版本：`Python 3.8/3.9`

安装依赖: `pip3 install -r requirements.txt`

假设还是输入`xxx.mp4`:

开始识别：`python3 whisper.py xxx.mp4`

参数：
`-l`，视频源语言

`-m`，Whisper模型，默认`large-v3`，质量最好

