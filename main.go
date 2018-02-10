package main

import (
	"log"
	"fmt"
)

func main() {
	str := &Str{Buffer: "aaa bbb"}
	str.Init()


	if err := str.Parse(); err != nil {
		log.Fatal(err)
	}

	//str.PrettyPrintSyntaxTree(str.Buffer)

	toks := str.Tokens()
	for _, t := range toks {
		if t.pegRule == ruleexpr {
			fmt.Println(t.String(), " - ", str.Buffer[t.begin:t.end])
		}
	}
}