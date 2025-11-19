package voice

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestVoice(t *testing.T) {
	f, err := New("test_test.mp3")
	if err != nil {
		t.Error("cannot read file", err)
		return
	}
	region := f.Vad()

	t.Log(f.To(region))
}

func TestVoiceWithAll(t *testing.T) {
	f, err := New("/Users/haolan/Downloads/SPSD-02.mp4")
	if err != nil {
		t.Error("cannot read file", err)
		return
	}
	regionsWithSpectral := f.Vad()

	regions := f.To(regionsWithSpectral)

	new, err := os.Create("new.pcm")
	if err != nil {
		t.Error("cannot read file", err)
		return
	}
	defer new.Close()

	duration := 0.0
	for i, f := range regions {
		exec.Command("ffplay", "-autoexit", "-ar", "16000", "-f", "s16le", "-acodec", "pcm_s16le", f).Run()
		fmt.Println(i, duration)
		duration += regionsWithSpectral[i].End - regionsWithSpectral[i].Start
	}

}

func TestVoiceWithReduce(t *testing.T) {
	f, err := New("/Users/haolan/Downloads/SPSD-02.mp4")
	if err != nil {
		t.Error("cannot read file", err)
		return
	}

	fmt.Println("\n========== WITHOUT SPECTRAL ANALYSIS ==========")
	regionsWithoutSpectral := f.Vad()
	fmt.Printf("Total regions: %d\n", len(regionsWithoutSpectral))

	// 统计对比
	fmt.Println("\n========== COMPARISON ==========")
	fmt.Printf("Without spectral: %d regions\n", len(regionsWithoutSpectral))

	// 抽取10分钟样本进行对比播放
	sampleDuration := 600.0 // 10分钟
	fmt.Printf("\n========== EXTRACTING %.0f MINUTE SAMPLE ==========\n", sampleDuration/60)

	// 计算总时长
	var totalDuration float64
	if len(regionsWithoutSpectral) > 0 {
		totalDuration = regionsWithoutSpectral[len(regionsWithoutSpectral)-1].End
	}
	fmt.Printf("Total video duration: %s\n", formatTimestamp(totalDuration))

	// 计算从哪个时间点开始抽样（从中间开始）
	startTime := (totalDuration - sampleDuration) / 2
	if startTime < 0 {
		startTime = 0
	}
	endTime := startTime + sampleDuration

	fmt.Printf("Sample range: %s - %s\n", formatTimestamp(startTime), formatTimestamp(endTime))

	// 过滤出样本时间范围内的区域
	sampleWithout := filterRegionsByTime(regionsWithoutSpectral, startTime, endTime)

	fmt.Printf("\nSample without spectral: %d regions\n", len(sampleWithout))

	// 对比播放
	fmt.Println("\n========== SIDE-BY-SIDE COMPARISON ==========")
	fmt.Println("For each timestamp, playing WITHOUT spectral first, then WITH spectral")
	fmt.Println("Press Ctrl+C to skip or stop")

	// 合并两个列表的时间戳，按时间排序
	timeStamps := make(map[float64]bool)
	for _, r := range sampleWithout {
		timeStamps[r.Start] = true
	}

	// 转换为排序列表
	var sortedTimes []float64
	for ts := range timeStamps {
		sortedTimes = append(sortedTimes, ts)
	}
	// 简单排序
	for i := 0; i < len(sortedTimes); i++ {
		for j := i + 1; j < len(sortedTimes); j++ {
			if sortedTimes[i] > sortedTimes[j] {
				sortedTimes[i], sortedTimes[j] = sortedTimes[j], sortedTimes[i]
			}
		}
	}

	// 遍历时间戳，播放对应区域
	for _, ts := range sortedTimes {
		// 查找该时间戳的区域
		var withoutRegion, withRegion *Region

		for i := range sampleWithout {
			if sampleWithout[i].Start == ts {
				withoutRegion = &sampleWithout[i]
				break
			}
		}
		fmt.Printf("\n========== Timestamp: %s ==========\n", formatTimestamp(ts))

		// 播放无频谱分析版本
		if withoutRegion != nil {
			fmt.Printf("▶ WITHOUT spectral: %s - %s (%.2fs)\n",
				formatTimestamp(withoutRegion.Start),
				formatTimestamp(withoutRegion.End),
				withoutRegion.End-withoutRegion.Start)
			files := f.To([]Region{*withoutRegion})
			if len(files) > 0 {
				cmd := exec.Command("ffplay", "-autoexit", "-ar", "16000", "-f", "s16le", "-acodec", "pcm_s16le", files[0])
				cmd.Run()
			}
		} else {
			fmt.Println("▶ WITHOUT spectral: [FILTERED OUT by VAD]")
		}

		// 播放有频谱分析版本
		if withRegion != nil {
			fmt.Printf("▶ WITH spectral:    %s - %s (%.2fs)\n",
				formatTimestamp(withRegion.Start),
				formatTimestamp(withRegion.End),
				withRegion.End-withRegion.Start)
			files := f.To([]Region{*withRegion})
			if len(files) > 0 {
				cmd := exec.Command("ffplay", "-autoexit", "-ar", "16000", "-f", "s16le", "-acodec", "pcm_s16le", files[0])
				cmd.Run()
			}
		} else {
			fmt.Println("▶ WITH spectral:    [FILTERED OUT by spectral analysis - likely BGM]")
		}
	}

	fmt.Println("\n========== PLAYBACK COMPLETE ==========")
}

// filterRegionsByTime 过滤出指定时间范围内的区域
func filterRegionsByTime(regions []Region, startTime, endTime float64) []Region {
	var filtered []Region
	for _, r := range regions {
		// 区域与时间范围有交集
		if r.End > startTime && r.Start < endTime {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// formatTimestamp 格式化时间戳为 MM:SS.mmm 格式
func formatTimestamp(seconds float64) string {
	minutes := int(seconds) / 60
	secs := int(seconds) % 60
	millis := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%02d:%02d.%03d", minutes, secs, millis)
}
