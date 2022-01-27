package main

//我们在数据库操作的时候，比如 dao 层中当遇到一个 sql.ErrNoRows 的时候，是否应该 Wrap 这个 error，抛给上层。
//为什么，应该怎么做请写出代码？
import (
	"fmt"
	"time"
)

func main(){
	message := make(chan int, 10)
	done := make(chan bool)

	defer close(message)

	go func(){
		ticker := time.NewTicker(time.Second)
		for _ = range ticker.C{
			select {
			case <-done:
				fmt.Println("child")
				return
			default:
				fmt.Println("send")
			}
		}
	}()
	for i:=0;i<10;i++{
		message<-i
	}
	time.Sleep(time.Second*5)
	close(done)
	time.Sleep(time.Second)
	fmt.Println("end")

}
