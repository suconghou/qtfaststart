package main

import (
	"fmt"
	"io"
	"os"

	"github.com/suconghou/qtfaststart"
)

func main1() {
	f, err := os.Open("/tmp/demo3.mp4")
	if err == nil {
		stat, err := f.Stat()
		if err == nil {
			fs := qtfaststart.New(f, stat.Size())
			ret, err := fs.Parse(f)
			if err == nil {
				for _, v := range ret {
					fmt.Println(v.Type, v.Offset, v.Size, v.Hsize)
				}
			} else {
				fmt.Println(err)
			}
		}
	}
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	f, err := os.Open("/tmp/demo3.mp4")
	if err == nil {
		stat, err := f.Stat()
		if err == nil {
			fs := qtfaststart.New(f, stat.Size())
			ret, err := fs.Convert(f)
			f, err := os.OpenFile("/tmp/1", os.O_WRONLY|os.O_CREATE, 0666)
			n, err := io.Copy(f, ret)
			fmt.Println(err, ret, n)
		}
	}
	if err != nil {
		fmt.Println(err)
	}
}

func main3() {
	url := "http://vpn.suconghou.cn/demo3.mp4"
	resp, err := qtfaststart.URLResp(url, nil, 60) //should check http1.1 and range support
	if err == nil {
		fs := qtfaststart.New(resp.Body, resp.ContentLength)
		ret, err := fs.ParseURL(url)
		if err == nil {
			for _, v := range ret {
				fmt.Println(v.Type, v.Offset, v.Size, v.Hsize)
			}
		} else {
			fmt.Println(err)
		}
	}
	if err != nil {
		fmt.Println(err)
	}
}

func main4() {
	url := "http://127.0.0.1:6060/demo3.mp4"
	resp, err := qtfaststart.URLResp(url, nil, 60) //should check http1.1 and range support
	if err == nil {
		fs := qtfaststart.New(resp.Body, resp.ContentLength)
		ret, err := fs.ConvertURL(url)
		if err == nil {
			f, err := os.OpenFile("/tmp/1", os.O_WRONLY|os.O_CREATE, 0666)
			n, err := io.Copy(f, ret)
			os.Stderr.WriteString(fmt.Sprintf("%d %s", n, err))
		} else {
			fmt.Println(err)
		}
	}
	if err != nil {
		fmt.Println(err)
	}
}
