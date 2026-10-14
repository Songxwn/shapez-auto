package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"

	"shapez-auto/assets"
	"shapez-auto/internal/server"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:17831", "控制面监听地址")
	flag.Parse()

	svc := server.New(assets.IndexHTML, assets.ModJS)
	url := "http://" + *addr + "/"
	go func() {
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}()
	fmt.Println("shapez-auto 控制面:", url)
	fmt.Println("关闭这个窗口就会停止自动通关。")
	log.Fatal(http.ListenAndServe(*addr, svc.Handler()))
}
