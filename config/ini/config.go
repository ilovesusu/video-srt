package ini

import "gopkg.in/ini.v1"

var SuAliISI aliyunISI
var SuAliOSS aliOSS

type aliyunISI struct {
	AppKey           string `ini:"AppKey"`
	AccessKeyId      string `ini:"AccessKeyId"`
	AccessKeySecret  string `ini:"AccessKeySecret"`
	IntelligentBlock bool   `ini:"IntelligentBlock"`
}

type aliOSS struct {
	Endpoint         string `ini:"Endpoint"`
	EndpointInternal string `ini:"EndpointInternal"`
	BucketName       string `ini:"BucketName"`
	BucketDomain     string `ini:"BucketDomain"`
	AccessKeyId      string `ini:"AccessKeyId"`
	AccessKeySecret  string `ini:"AccessKeySecret"`
}

func init() {
	cfg, err := ini.ShadowLoad("config.ini")
	if err != nil {
		panic(err)
	}

	err = cfg.Section("aliyunISI").MapTo(&SuAliISI)
	if err != nil {
		panic(err)
	}

	if err != nil {
		panic(err)
	}
}
