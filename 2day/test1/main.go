package main

import (
	"fmt"
	"time"
)

func hello(ch chan int){
	ch<-99
	fmt.Println("hello")
}

func main(){
	var a = make(chan int)

	for i:=0;i<3;i++{
		go hello(a)
		fmt.Println("i=",i)
		fmt.Println("main")
		if i==2{
			fmt.Println(<-a)
			time.Sleep(time.Second)
			fmt.Println(<-a)
			//time.Sleep(time.Second)
			fmt.Println(<-a)
			//time.Sleep(time.Second)

		}
	}
	//time.Sleep(time.Second)


}
