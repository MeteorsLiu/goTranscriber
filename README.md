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
2. `BGM`会一定程度上影响识别


# 安装教程

## Linux(Debian为例)

```
apt install gcc g++ ffmpeg -y
wget https://go.dev/dl/go1.19.3.linux-amd64.tar.gz && rm -rf /usr/local/go && tar -C /usr/local -xzf go1.19.3.linux-amd64.tar.gz
git clone https://github.com/MeteorsLiu/goTranscriber.git
cd goTranscriber
/usr/local/go/bin/go build 
./goSRT -file xxx -lang xx
```

# English

I am sorry that I don't provide binary file.

It's required to build it by yourself.

Currently, goTranscriber **DON'T** support Windows, for the Gcc compiler reason.

YOU NEED TO Build this project in Linux.

## Linux(Debian/Ubuntu)

```
apt install gcc g++ ffmpeg -y
wget https://go.dev/dl/go1.19.3.linux-amd64.tar.gz && rm -rf /usr/local/go && tar -C /usr/local -xzf go1.19.3.linux-amd64.tar.gz
git clone https://github.com/MeteorsLiu/goTranscriber.git
cd goTranscriber
/usr/local/go/bin/go build 
./goSRT -file xxx -lang xx
```
