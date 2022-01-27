package main

import "fmt"

func main(){
	//var a = 1
	//var b = 3
	////a = a+b
	////b = a-b
	////a = a-b
	//a,b = b,a
	//fmt.Printf("a= %d,b=%d",a,b)


	var a *int
	var b int
	b= *a

	fmt.Println(b)
	//fmt.Println(a)

}
