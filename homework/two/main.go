package main

import (
	"database/sql"
	"fmt"
	"log"
)
//我们在数据库操作的时候，比如 dao 层中当遇到一个 sql.ErrNoRows 的时候，是否应该 Wrap 这个 error，抛给上层。 为什么，应该怎么做请写出代码？
//应该。
//sql.go中定义var ErrNoRows = errors.New("sql: no rows in result set")。 按照条件查询的数据不存在，是一个正常的错误。
//上层应该对该特殊情况进行单独处理，代码如下（）
func main() {
	db,err := sql.Open("mysql","")
	if err != nil{
		fmt.Println(err)
	}

	res,err :=db.Query("select name from user where id=1")

	if err != nil {
		if err == sql.ErrNoRows {
			//wrap这个error，抛给上层
		} else {
			log.Fatal(err)
		}
	}
	fmt.Println(*res)
}
