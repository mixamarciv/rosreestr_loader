package main

import (
	"fmt"
	"os"
	s "strings"

	mf "github.com/mixamarciv/gofncstd3000"
)

var log_file string

func init() {
	path, _ := mf.AppPath()
	timestr := mf.CurTimeStrShort()
	path = s.Replace(path, "\\", "/", -1) + "/log/" + timestr[0:8]
	mf.MkdirAll(path)
	log_file = path + "/" + mf.CurTimeStrShort() + ".log"
	WriteLogln("start log")
}

func WriteLog(data string) {
	mf.FileAppendStr(log_file, data)
}

func WriteLogln(data string) {
	mf.FileAppendStr(log_file, mf.CurTimeStr()+" "+s.TrimRight(data, "\n\r\t ")+"\n")
}

func WriteLogErr(info string, err error) {
	mf.FileAppendStr(log_file, mf.CurTimeStr()+" "+info+"\n"+mf.ErrStr(err))
}

func WriteLogErrAndExit(info string, err error) {
	if err == nil {
		return
	}
	mf.FileAppendStr(log_file, mf.CurTimeStr()+" "+info+"\n"+mf.ErrStr(err))
	panic(err)
	os.Exit(1)
}

func LogPrint(data string) {
	fmt.Println(data)
	WriteLogln(data)
}

func LogPrintErrAndExit(info string, err error) {
	if err == nil {
		return
	}
	fmt.Println(info)
	fmt.Printf("%+v", err)
	WriteLogErrAndExit(info, err)
}

func LogPrintAndExit(info string) {
	fmt.Println(info)
	WriteLogln(info)
	os.Exit(1)
}