package main

import (
	"fmt"
)

func printHello(ch chan int){
	fmt.Printf("hello from 1\n")
	ch<-2
}

func main() {
	ch := make(chan int)
	go func() {
		fmt.Printf("hello inline\n")
		ch<-1
	}()
	go printHello(ch)
	fmt.Printf("hello from main\n")
	i:= <-ch
	fmt.Println("Recieved ",i)
}
