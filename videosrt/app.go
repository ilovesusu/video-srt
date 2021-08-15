package videosrt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/ilovesusu/video-srt/config/ini"
	"github.com/ilovesusu/video-srt/videosrt/aliyun/cloud"
	"github.com/ilovesusu/video-srt/videosrt/aliyun/oss"
	"github.com/ilovesusu/video-srt/videosrt/ffmpeg"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

//主应用
type VideoSrt struct {
	Ffmpeg       ffmpeg.Ffmpeg
	AliyunOss    oss.AliyunOss      //oss
	AliyunClound cloud.AliyunClound //语音识别引擎

	IntelligentBlock bool   //智能分段处理
	TempDir          string //临时文件目录
	AppDir           string //应用根目录
}

//获取应用
func NewApp(cfg string) *VideoSrt {
	app := ReadConfig(cfg)

	return app
}

//读取配置
func ReadConfig(appDir string) *VideoSrt {
	appconfig := &VideoSrt{}

	//AliyunOss
	appconfig.AliyunOss.Endpoint = ini.SuAliOSS.Endpoint
	appconfig.AliyunOss.EndpointInternal = ini.SuAliOSS.EndpointInternal
	appconfig.AliyunOss.AccessKeyId = ini.SuAliOSS.AccessKeyId
	appconfig.AliyunOss.AccessKeySecret = ini.SuAliOSS.AccessKeySecret
	appconfig.AliyunOss.BucketName = ini.SuAliOSS.BucketName
	appconfig.AliyunOss.BucketDomain = ini.SuAliOSS.BucketDomain

	//AliyunISI
	appconfig.AliyunClound.AccessKeyId = ini.SuAliISI.AccessKeyId
	appconfig.AliyunClound.AccessKeySecret = ini.SuAliISI.AccessKeySecret
	appconfig.AliyunClound.AppKey = ini.SuAliISI.AppKey

	appconfig.IntelligentBlock = ini.SuAliISI.IntelligentBlock

	appconfig.TempDir = "temp/audio"
	appconfig.AppDir = appDir

	return appconfig
}

//应用运行
func (app *VideoSrt) Run2Srt(video string) {
	if video == "" {
		panic("enter a video file waiting to be processed .")
	}

	//校验视频
	if VaildVideo(video) != true {
		panic("the input video file does not exist .")
	}

	tmpAudioDir := app.AppDir + "/" + app.TempDir
	if !DirExists(tmpAudioDir) {
		//创建目录
		if err := CreateDir(tmpAudioDir, false); err != nil {
			panic(err)
		}
	}
	tmpAudioFile := GetRandomCodeString(15) + ".mp3"
	tmpAudio := tmpAudioDir + "/" + tmpAudioFile

	Log("提取音频文件 ...")

	//分离视频音频
	ExtractVideoAudio(video, tmpAudio)

	Log("上传音频文件 ...")

	//上传音频至OSS
	filelink := UploadAudioToClound(app.AliyunOss, tmpAudio)
	//获取完整链接
	filelink = app.AliyunOss.GetObjectFileUrl(filelink)

	Log("上传文件成功 , 识别中 ...")

	//阿里云录音文件识别
	AudioResult := AliyunAudioRecognition(app.AliyunClound, filelink, app.IntelligentBlock)

	Log("文件识别成功 , 字幕处理中 ...")

	//输出字幕文件
	AliyunAudioResultMakeSubtitleFile(video, AudioResult)

	Log("完成")

	//删除临时文件
	if remove := os.Remove(tmpAudio); remove != nil {
		panic(remove)
	}
}

//提取视频音频文件
func ExtractVideoAudio(video string, tmpAudio string) {
	if err := ffmpeg.ExtractAudio(video, tmpAudio); err != nil {
		panic(err)
	}
}

//上传音频至oss
func UploadAudioToClound(target oss.AliyunOss, audioFile string) string {
	name := ""
	//提取文件名称
	if fileInfo, e := os.Stat(audioFile); e != nil {
		panic(e)
	} else {
		name = fileInfo.Name()
	}

	//上传
	if file, e := target.UploadFile(audioFile, name); e != nil {
		panic(e)
	} else {
		return file
	}
}

//阿里云录音文件识别
func AliyunAudioRecognition(engine cloud.AliyunClound, filelink string, intelligent_block bool) (AudioResult map[int64][]*cloud.AliyunAudioRecognitionResult) {
	//创建识别请求
	taskid, client, e := engine.NewAudioFile(filelink)
	if e != nil {
		panic(e)
	}

	AudioResult = make(map[int64][]*cloud.AliyunAudioRecognitionResult)

	//遍历获取识别结果
	engine.GetAudioFileResult(taskid, client, func(result []byte) {
		//mylog.WriteLog( string( result ) )

		//结果处理
		statusText, _ := jsonparser.GetString(result, "StatusText") //结果状态
		if statusText == cloud.STATUS_SUCCESS {

			//智能分段
			if intelligent_block {
				cloud.AliyunAudioResultWordHandle(result, func(vresult *cloud.AliyunAudioRecognitionResult) {
					channelId := vresult.ChannelId

					_, isPresent := AudioResult[channelId]
					if isPresent {
						//追加
						AudioResult[channelId] = append(AudioResult[channelId], vresult)
					} else {
						//初始
						AudioResult[channelId] = []*cloud.AliyunAudioRecognitionResult{}
						AudioResult[channelId] = append(AudioResult[channelId], vresult)
					}
				})
				return
			}

			_, err := jsonparser.ArrayEach(result, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
				text, _ := jsonparser.GetString(value, "Text")
				channelId, _ := jsonparser.GetInt(value, "ChannelId")
				beginTime, _ := jsonparser.GetInt(value, "BeginTime")
				endTime, _ := jsonparser.GetInt(value, "EndTime")
				silenceDuration, _ := jsonparser.GetInt(value, "SilenceDuration")
				speechRate, _ := jsonparser.GetInt(value, "SpeechRate")
				emotionValue, _ := jsonparser.GetInt(value, "EmotionValue")

				vresult := &cloud.AliyunAudioRecognitionResult{
					Text:            text,
					ChannelId:       channelId,
					BeginTime:       beginTime,
					EndTime:         endTime,
					SilenceDuration: silenceDuration,
					SpeechRate:      speechRate,
					EmotionValue:    emotionValue,
				}

				_, isPresent := AudioResult[channelId]
				if isPresent {
					//追加
					AudioResult[channelId] = append(AudioResult[channelId], vresult)
				} else {
					//初始
					AudioResult[channelId] = []*cloud.AliyunAudioRecognitionResult{}
					AudioResult[channelId] = append(AudioResult[channelId], vresult)
				}
			}, "Result", "Sentences")
			if err != nil {
				panic(err)
			}
		}
	})

	return
}

//阿里云录音识别结果集生成字幕文件
func AliyunAudioResultMakeSubtitleFile(video string, AudioResult map[int64][]*cloud.AliyunAudioRecognitionResult) {
	subfileDir := path.Dir(video)
	subfile := GetFileBaseName(video)

	for channel, result := range AudioResult {
		thisfile := subfileDir + "/" + subfile + "_channel_" + strconv.FormatInt(channel, 10) + ".srt"
		//输出字幕文件
		println(thisfile)

		file, e := os.Create(thisfile)
		if e != nil {
			panic(e)
		}

		defer file.Close() //defer

		index := 0
		for _, data := range result {
			linestr := MakeSubtitleText(index, data.BeginTime, data.EndTime, data.Text)

			file.WriteString(linestr)

			index++
		}
	}
}

//拼接字幕字符串
func MakeSubtitleText(index int, startTime int64, endTime int64, text string) string {
	var content bytes.Buffer
	content.WriteString(strconv.Itoa(index))
	content.WriteString("\n")
	content.WriteString(SubtitleTimeMillisecond(startTime))
	content.WriteString(" --> ")
	content.WriteString(SubtitleTimeMillisecond(endTime))
	content.WriteString("\n")
	content.WriteString(text)
	content.WriteString("\n")
	content.WriteString("\n")
	return content.String()
}

//todo 合成语音
func (app *VideoSrt) Run2Wav(text string) {
	filedata, _ := ioutil.ReadFile(text)
	var appkey string = "dDtODT7zmodosF0m"
	var token string = "4c4eeeaed2c644c98b346d7ac04001b6"
	//var text string = "今天是周一，天气挺好的。"
	requestLongTts4Post(string(filedata), appkey, token)
}

// 长文本语音合成restful接口，支持post调用，不支持get请求。发出请求后，可以轮询状态或者等待服务端合成后自动回调（如果设置了回调参数）。
func requestLongTts4Post(text string, appkey string, token string) {

	var url string = "https://nls-gateway.cn-shanghai.aliyuncs.com/rest/v1/tts/async"

	// 拼接HTTP Post请求的消息体内容。
	context := make(map[string]interface{})
	context["device_id"] = "test-device"
	header := make(map[string]interface{})
	header["appkey"] = appkey
	header["token"] = token
	tts_request := make(map[string]interface{})
	tts_request["text"] = text
	tts_request["format"] = "wav"
	tts_request["voice"] = "ailun"
	tts_request["sample_rate"] = 16000
	tts_request["speech_rate"] = 10
	tts_request["pitch_rate"] = 40
	tts_request["enable_subtitle"] = false
	payload := make(map[string]interface{})
	payload["enable_notify"] = false
	//payload["notify_url"] = "http://123.com"
	payload["tts_request"] = tts_request
	ttsBody := make(map[string]interface{})
	ttsBody["context"] = context
	ttsBody["header"] = header
	ttsBody["payload"] = payload
	ttsBodyJson, err := json.Marshal(ttsBody)
	if err != nil {
		panic(nil)
	}
	fmt.Println(string(ttsBodyJson))
	// 发送HTTPS POST请求，处理服务端的响应。
	response, err := http.Post(url, "application/json;charset=utf-8", bytes.NewBuffer([]byte(ttsBodyJson)))
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	contentType := response.Header.Get("Content-Type")
	body, _ := ioutil.ReadAll(response.Body)
	fmt.Println(string(contentType))
	fmt.Println(string(body))
	statusCode := response.StatusCode
	if statusCode == 200 {
		fmt.Println("The POST request succeed!")
		var f interface{}
		err := json.Unmarshal(body, &f)
		if err != nil {
			fmt.Println(err)
		}
		// 从消息体中解析出来task_id（重要）和request_id。
		var taskId string = ""
		var requestId string = ""
		m := f.(map[string]interface{})
		for k, v := range m {
			if k == "error_code" {
				fmt.Println("error_code = ", v)
			} else if k == "request_id" {
				fmt.Println("request_id = ", v)
				requestId = v.(string)

			} else if k == "data" {
				fmt.Println("data = ", v)
				data := v.(map[string]interface{})
				fmt.Println(data)
				taskId = data["task_id"].(string)

			}
		}

		// 说明：轮询检查服务端的合成状态，轮询操作非必须，如果设置了回调url，则服务端会在合成完成后主动回调。
		waitLoop4Complete(url, appkey, token, taskId, requestId)

	} else {
		fmt.Println("The POST request failed: " + string(body))
		fmt.Println("The HTTP statusCode: " + strconv.Itoa(statusCode))
	}

}

// 根据特定信息轮询检查某个请求在服务端的合成状态，每隔10秒钟轮询一次状态.轮询操作非必须，如果设置了回调url，则服务端会在合成完成后主动回调。
func waitLoop4Complete(url string, appkey string, token string, task_id string, request_id string) {
	var fullUrl string = url + "?appkey=" + appkey + "&task_id=" + task_id + "&token=" + token + "&request_id=" + request_id
	fmt.Println("fullUrl=" + fullUrl)
	for {
		response, err := http.Get(fullUrl)
		if err != nil {
			fmt.Println("The GET request failed!")
			panic(err)
		}
		defer response.Body.Close()
		body, _ := ioutil.ReadAll(response.Body)
		fmt.Println("waitLoop4Complete = ", string(body))
		var f interface{}
		json.Unmarshal(body, &f)
		if err != nil {
			fmt.Println(err)
		}
		// 从消息体中解析出来task_id（重要）和request_id。
		var code float64 = 0
		var taskId string = ""
		var audioAddress string = ""
		m := f.(map[string]interface{})
		for k, v := range m {
			if k == "error_code" {
				code = v.(float64)
			} else if k == "request_id" {
			} else if k == "data" {
				if v != nil {
					data := v.(map[string]interface{})
					taskId = data["task_id"].(string)
					if data["audio_address"] == nil {
						fmt.Println("Tts Queuing...,please wait...")

					} else {
						audioAddress = data["audio_address"].(string)
					}
				}
			}
		}
		if code == 20000000 && audioAddress != "" {
			fmt.Println("Tts Finished! task_id = " + taskId)
			fmt.Println("Tts Finished! audio_address = " + audioAddress)
			break
		} else {
			// 每隔10秒钟轮询一次状态
			time.Sleep(time.Duration(10) * time.Second)
		}
	}
}
