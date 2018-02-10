package main

import (
	"fmt"
)

func main() {
	str := &Str{Buffer: "key = val, key1=val1"}
	str.Init()

	if err := str.Parse(); err != nil {
		panic(err)
	}

	//str.PrettyPrintSyntaxTree(str.Buffer)

	toks := str.Tokens()
	for _, t := range toks {
		if t.pegRule == rulestring {
			fmt.Println(t.String(), " - ", str.Buffer[t.begin:t.end])
		}
	}
}
