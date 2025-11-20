import argparse
import os
import random
import asyncio
from pathlib import Path
from faster_whisper import WhisperModel
from googletrans import Translator


def format_srt_time(seconds):
    """将秒数转换为 SRT 时间格式 (HH:MM:SS,mmm)"""
    hours = int(seconds // 3600)
    minutes = int((seconds % 3600) // 60)
    secs = int(seconds % 60)
    millis = int((seconds % 1) * 1000)
    return f"{hours:02d}:{minutes:02d}:{secs:02d},{millis:03d}"


def get_unique_filename(base_path):
    """如果文件已存在，添加随机数生成唯一文件名"""
    if not os.path.exists(base_path):
        return base_path

    # 文件已存在，添加随机数
    path_obj = Path(base_path)
    stem = path_obj.stem
    suffix = path_obj.suffix
    directory = path_obj.parent

    while True:
        random_num = random.randint(1000, 9999)
        new_filename = f"{stem}_{random_num}{suffix}"
        new_path = directory / new_filename
        if not os.path.exists(new_path):
            return str(new_path)


async def transcribe_and_translate(input_file, model_size="large-v3", language="ja"):
    """
    转录音频/视频文件，翻译成中文，并保存为 SRT 文件

    Args:
        input_file: 输入文件路径
        model_size: Whisper 模型大小
        language: 源语言代码
    """
    print(f"加载模型: {model_size}")
    model = WhisperModel(model_size)

    print(f"开始转录文件: {input_file}")
    segments, info = model.transcribe(
        input_file,
        language=language,
        log_progress=True,
        word_timestamps=True,
        vad_filter=True,
        vad_parameters={
            "threshold": 0.35,
            "speech_pad_ms": 400,
            "max_speech_duration_s": float("inf"),
            "min_speech_duration_ms": 200,
            "min_silence_duration_ms": 500,
        }
    )

    # 收集所有片段
    print("收集转录片段...")
    segment_list = []
    for segment in segments:
        segment_list.append({
            'start': segment.start,
            'end': segment.end,
            'text': segment.text
        })
        print(f"[{segment.start:.2f}s -> {segment.end:.2f}s] {segment.text}")

    # 翻译成中文（并发翻译，限制并发数为 CPU 核心数）
    print("\n开始翻译成中文...")
    translator = Translator()

    # 获取 CPU 核心数
    max_concurrent = os.cpu_count() or 10
    semaphore = asyncio.Semaphore(max_concurrent)
    print(f"使用 {max_concurrent} 个并发任务进行翻译")

    async def translate_segment(segment, index):
        """翻译单个片段，使用信号量控制并发"""
        async with semaphore:
            try:
                translation = await translator.translate(segment['text'], src=language, dest='zh-cn')
                segment['translation'] = translation.text
                print(f"[{index}/{len(segment_list)}] 翻译: {segment['text']} -> {segment['translation']}")
            except Exception as e:
                print(f"翻译失败 ({segment['text']}): {e}")
                segment['translation'] = segment['text']  # 翻译失败时使用原文

    # 并发翻译所有片段
    tasks = [translate_segment(segment, i) for i, segment in enumerate(segment_list, 1)]
    await asyncio.gather(*tasks)

    # 生成 SRT 文件
    input_path = Path(input_file)
    output_dir = input_path.parent
    base_srt_path = output_dir / f"{input_path.stem}.srt"
    srt_path = get_unique_filename(str(base_srt_path))

    print(f"\n生成 SRT 文件: {srt_path}")
    with open(srt_path, 'w', encoding='utf-8') as f:
        for i, segment in enumerate(segment_list, 1):
            # SRT 格式:
            # 1
            # 00:00:00,000 --> 00:00:02,000
            # 翻译文本
            f.write(f"{i}\n")
            f.write(f"{format_srt_time(segment['start'])} --> {format_srt_time(segment['end'])}\n")
            f.write(f"{segment['translation']}\n\n")

    print(f"✓ 完成！SRT 文件已保存到: {srt_path}")
    return srt_path


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='转录音频/视频文件并翻译成中文 SRT 字幕')
    parser.add_argument('input_file', help='输入音频/视频文件路径')
    parser.add_argument('-m', '--model', default='large-v3',
                        help='Whisper 模型大小 (默认: large-v3)')
    parser.add_argument('-l', '--language', default='ja',
                        help='源语言代码 (默认: ja 日语)')

    args = parser.parse_args()

    # 检查输入文件是否存在
    if not os.path.exists(args.input_file):
        print(f"错误: 文件不存在 - {args.input_file}")
        exit(1)

    asyncio.run(transcribe_and_translate(args.input_file, args.model, args.language))
