package main

import (
	"bytes"
	"crypto/md5"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chai2010/webp"
	"github.com/cilidm/toolbox/logging"
	"golang.org/x/image/bmp"
)

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		os.MkdirAll(path, os.ModePerm)
		return false, nil
	}
	return false, err
}

// 判断字符串是否在数组里
func IsContain(items []string, item string) bool {
	for _, eachItem := range items {
		if eachItem == item {
			return true
		}
	}
	return false
}

func GetMd5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	md5hash := md5.New()
	if _, err := io.Copy(md5hash, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5hash.Sum(nil)), nil
}

func walkDir(dir, dst string) {
	PathExists(dst)
	var num uint64 = 1
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if info.IsDir() == false {
			ext := path.Ext(info.Name())
			if IsContain([]string{".jpg", ".jpeg", ".png", ".bmp", ".gif"}, ext) {
				maxChan <- struct{}{}
				atomic.AddUint64(&num, 1)
				md, err := GetMd5(p)
				if err != nil {
					logging.Error(err.Error())
					return nil
				}
				webpFile := dst + "/" + md + ".webp"
				if CheckFileIsExist(webpFile) == false {
					go change2webp(p, webpFile, 90, true)
				}
			}
		}
		return nil
	})
	close(maxChan)
	searchDone <- struct{}{}
}

/**
 * 判断文件是否存在  存在返回 true 不存在返回false
 */
func CheckFileIsExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}

func change2webp(p1, p2 string, quality float32, Log bool) {
	fmt.Println("begin", p1)
	err := WebpEncoder(p1, p2, quality, Log)
	if err != nil {
		logging.Error(err.Error())
	}
	<-maxChan
}

func WebpEncoder(p1, p2 string, quality float32, Log bool) (err error) {
	logging.Debug("img path :", path.Base(p1), ",quality is :", quality)
	var buf bytes.Buffer
	var img image.Image
	data, err := ioutil.ReadFile(p1)
	if err != nil {
		return err
	}
	contentType := GetFileContentType(data[:512])
	if strings.Contains(contentType, "jpeg") {
		img, _ = jpeg.Decode(bytes.NewReader(data))
	} else if strings.Contains(contentType, "png") {
		img, _ = png.Decode(bytes.NewReader(data))
	} else if strings.Contains(contentType, "bmp") {
		img, _ = bmp.Decode(bytes.NewReader(data))
	} else if strings.Contains(contentType, "gif") {
		logging.Warn("Gif support is not perfect!")
		img, _ = gif.Decode(bytes.NewReader(data))
	}
	if img == nil {
		msg := "image file " + path.Base(p1) + " is corrupted or not supported"
		err = errors.New(msg)
		return err
	}
	if err = webp.Encode(&buf, img, &webp.Options{Lossless: false, Quality: quality}); err != nil {
		return err
	}
	if err = ioutil.WriteFile(p2, buf.Bytes(), 0644); err != nil {
		return err
	}
	if Log {
		logging.Info("Save to " + p2 + " ok!")
	}
	return nil
}

func GetFileContentType(buffer []byte) string {
	contentType := http.DetectContentType(buffer)
	return contentType
}

var (
	maxChan    = make(chan struct{}, 10)
	done       = make(chan bool)
	searchDone = make(chan struct{})
	source     string
	dst        string
)

func monitorDone() {
	select {
	case <-searchDone:
		for {
			if len(maxChan) == 0 {
				done <- true
				break
			}
		}
	}
}

func init() {
	go monitorDone()
	flag.StringVar(&source, "s", "", "源地址")
	flag.StringVar(&dst, "d", "", "目标地址")
	flag.Parse()
}

func main() {
	if source == "" || dst == "" {
		log.Fatal("请输入源地址及目标地址","-s 源地址","-d 目标地址")
	}

	start := time.Now()

	walkDir(source, dst)

	select {
	case <-done:
		fmt.Println("文件更新完毕,耗时", time.Since(start))
	}
}