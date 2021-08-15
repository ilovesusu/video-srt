package main

import (
	"flag"
	"fmt"
	"github.com/ilovesusu/video-srt/videosrt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	video string
	text  string
)

func main() {

	//致命错误捕获
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("")
			log.Printf("错误:\n%v", err)

			time.Sleep(time.Second * 5)
		}
	}()

	//无参数则显示帮助
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "-h")
	}

	//设置命令行参数
	flag.StringVar(&text, "text", "", "请输入准备合成语音的文本文件")
	flag.StringVar(&video, "video", "", "请输入准备导出字幕的视频文件")

	flag.Parse()

	//获取应用
	appDir, err := filepath.Abs(filepath.Dir(os.Args[0])) //应用执行根目录
	if err != nil {
		panic(err)
	}

	app := videosrt.NewApp(appDir)

	appDir = videosrt.WinDir(appDir)

	//根据参数调起应用
	if text != "" {
		fmt.Println("**合成语音**")
		app.Run2Wav(videosrt.WinDir(text))
	}

	if video != "" {
		fmt.Println("**导出字幕**")
		app.Run2Srt(videosrt.WinDir(video))
	}

	//延迟退出
	time.Sleep(time.Second * 1)
}
