package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"golang.org/x/sync/errgroup"
)

func StartHttpServer(src *http.Server) error{
	http.HandleFunc("/hello",helloServer)
	fmt.Println("start")
	return src.ListenAndServe()
}

func helloServer(w http.ResponseWriter,req *http.Request){
	io.WriteString(w,"hello Go")
}

func main(){
	//fmt.Println("q")
	ctx := context.Background()
	//定义WithCancel,企业选下游的Context
	ctx,cancel := context.WithCancel(ctx)
	//使用errgroup进行goroutine取消
	group, errCtx := errgroup.WithContext(ctx)
	srv := &http.Server{addr:":8080"}

	group.Go(func()error{
		return StartHttpServer(srv)
	})

	group.Go(func() {
		<-errCtx.Done()
		fmt.Println("stop")
		return srv.Shutdown(errCtx)
	})

	chanel := make(chan os.Signal,1)
	signal.Notify(chanel)
	group.Go(func() {
		for{
			select {
			case <-errCtx.Done():
				return errCtx.Err()
				case <-chanel:
					cancel()
			}
		}
		return nil
	})
	err := group.Wait()
	if err != nil{
		fmt.Println("group error: ",err)
	}
	fmt.Println("all group done")
}
