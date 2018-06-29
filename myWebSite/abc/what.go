package abc

import "fmt"

func Abc(str string) {
	fmt.Println(str)
}

type Book struct {
	ID            int64
	Title         string
	Author        string
	PublishedDate string
	ImageURL      string
	Description   string
	CreatedBy     string
	CreatedByID   string
}
