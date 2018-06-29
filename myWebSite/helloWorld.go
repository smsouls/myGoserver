package main

import (
	. "./abc"
	"fmt"
)

func main() {
	Abc("hell")

	book1 := &Book{
		ID:            110,
		Title:         "小朋友",
		Author:        "小朋友",
		PublishedDate: "2018-06-28",
		ImageURL:      "http://www.baidu.com",
		Description:   "哈哈啊哈哈哈哈啊",
		CreatedBy:     "tiny",
		CreatedByID:   "110",
	}

	fmt.Println(book1.Title)

}
