package main

//go:generate peg -switch -inline str.peg

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const endSymbol rune = 1114112

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	ruleline
	ruleexpr
	rulekv
	rulestring
	rulews
)

var rul3s = [...]string{
	"Unknown",
	"line",
	"expr",
	"kv",
	"string",
	"ws",
}

type token32 struct {
	pegRule
	begin, end uint32
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v", rul3s[t.pegRule], t.begin, t.end)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(pretty bool, buffer string) {
	var print func(node *node32, depth int)
	print = func(node *node32, depth int) {
		for node != nil {
			for c := 0; c < depth; c++ {
				fmt.Printf(" ")
			}
			rule := rul3s[node.pegRule]
			quote := strconv.Quote(string(([]rune(buffer)[node.begin:node.end])))
			if !pretty {
				fmt.Printf("%v %v\n", rule, quote)
			} else {
				fmt.Printf("\x1B[34m%v\x1B[m %v\n", rule, quote)
			}
			if node.up != nil {
				print(node.up, depth+1)
			}
			node = node.next
		}
	}
	print(node, 0)
}

func (node *node32) Print(buffer string) {
	node.print(false, buffer)
}

func (node *node32) PrettyPrint(buffer string) {
	node.print(true, buffer)
}

type tokens32 struct {
	tree []token32
}

func (t *tokens32) Trim(length uint32) {
	t.tree = t.tree[:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) AST() *node32 {
	type element struct {
		node *node32
		down *element
	}
	tokens := t.Tokens()
	var stack *element
	for _, token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	if stack != nil {
		return stack.node
	}
	return nil
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	t.AST().Print(buffer)
}

func (t *tokens32) PrettyPrintSyntaxTree(buffer string) {
	t.AST().PrettyPrint(buffer)
}

func (t *tokens32) Add(rule pegRule, begin, end, index uint32) {
	if tree := t.tree; int(index) >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	t.tree[index] = token32{
		pegRule: rule,
		begin:   begin,
		end:     end,
	}
}

func (t *tokens32) Tokens() []token32 {
	return t.tree
}

type Str struct {
	Buffer string
	buffer []rune
	rules  [6]func() bool
	parse  func(rule ...int) error
	reset  func()
	Pretty bool
	tokens32
}

func (p *Str) Parse(rule ...int) error {
	return p.parse(rule...)
}

func (p *Str) Reset() {
	p.reset()
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer []rune, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p   *Str
	max token32
}

func (e *parseError) Error() string {
	tokens, error := []token32{e.max}, "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.buffer, positions)
	format := "parse error near %v (line %v symbol %v - line %v symbol %v):\n%v\n"
	if e.p.Pretty {
		format = "parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n"
	}
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf(format,
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			strconv.Quote(string(e.p.buffer[begin:end])))
	}

	return error
}

func (p *Str) PrintSyntaxTree() {
	if p.Pretty {
		p.tokens32.PrettyPrintSyntaxTree(p.Buffer)
	} else {
		p.tokens32.PrintSyntaxTree(p.Buffer)
	}
}

func (p *Str) Init() {
	var (
		max                  token32
		position, tokenIndex uint32
		buffer               []rune
	)
	p.reset = func() {
		max = token32{}
		position, tokenIndex = 0, 0

		p.buffer = []rune(p.Buffer)
		if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != endSymbol {
			p.buffer = append(p.buffer, endSymbol)
		}
		buffer = p.buffer
	}
	p.reset()

	_rules := p.rules
	tree := tokens32{tree: make([]token32, math.MaxInt16)}
	p.parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokens32 = tree
		if matches {
			p.Trim(tokenIndex)
			return nil
		}
		return &parseError{p, max}
	}

	add := func(rule pegRule, begin uint32) {
		tree.Add(rule, begin, position, tokenIndex)
		tokenIndex++
		if begin != position && position > max.end {
			max = token32{rule, begin, position}
		}
	}

	matchDot := func() bool {
		if buffer[position] != endSymbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	_rules = [...]func() bool{
		nil,
		/* 0 line <- <(expr !.)> */
		func() bool {
			position0, tokenIndex0 := position, tokenIndex
			{
				position1 := position
				{
					position2 := position
				l3:
					{
						position4, tokenIndex4 := position, tokenIndex
						{
							position5, tokenIndex5 := position, tokenIndex
							if !_rules[rulekv]() {
								goto l6
							}
							goto l5
						l6:
							position, tokenIndex = position5, tokenIndex5
							{
								position7, tokenIndex7 := position, tokenIndex
								if !_rules[rulews]() {
									goto l7
								}
								goto l8
							l7:
								position, tokenIndex = position7, tokenIndex7
							}
						l8:
							if buffer[position] != rune(',') {
								goto l4
							}
							position++
							{
								position9, tokenIndex9 := position, tokenIndex
								if !_rules[rulews]() {
									goto l9
								}
								goto l10
							l9:
								position, tokenIndex = position9, tokenIndex9
							}
						l10:
							if !_rules[rulekv]() {
								goto l4
							}
						}
					l5:
						goto l3
					l4:
						position, tokenIndex = position4, tokenIndex4
					}
					add(ruleexpr, position2)
				}
				{
					position11, tokenIndex11 := position, tokenIndex
					if !matchDot() {
						goto l11
					}
					goto l0
				l11:
					position, tokenIndex = position11, tokenIndex11
				}
				add(ruleline, position1)
			}
			return true
		l0:
			position, tokenIndex = position0, tokenIndex0
			return false
		},
		/* 1 expr <- <(kv / (ws? ',' ws? kv))*> */
		nil,
		/* 2 kv <- <(string ws? '=' ws? string)> */
		func() bool {
			position13, tokenIndex13 := position, tokenIndex
			{
				position14 := position
				if !_rules[rulestring]() {
					goto l13
				}
				{
					position15, tokenIndex15 := position, tokenIndex
					if !_rules[rulews]() {
						goto l15
					}
					goto l16
				l15:
					position, tokenIndex = position15, tokenIndex15
				}
			l16:
				if buffer[position] != rune('=') {
					goto l13
				}
				position++
				{
					position17, tokenIndex17 := position, tokenIndex
					if !_rules[rulews]() {
						goto l17
					}
					goto l18
				l17:
					position, tokenIndex = position17, tokenIndex17
				}
			l18:
				if !_rules[rulestring]() {
					goto l13
				}
				add(rulekv, position14)
			}
			return true
		l13:
			position, tokenIndex = position13, tokenIndex13
			return false
		},
		/* 3 string <- <([a-z] / [0-9])+> */
		func() bool {
			position19, tokenIndex19 := position, tokenIndex
			{
				position20 := position
				{
					position23, tokenIndex23 := position, tokenIndex
					if c := buffer[position]; c < rune('a') || c > rune('z') {
						goto l24
					}
					position++
					goto l23
				l24:
					position, tokenIndex = position23, tokenIndex23
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l19
					}
					position++
				}
			l23:
			l21:
				{
					position22, tokenIndex22 := position, tokenIndex
					{
						position25, tokenIndex25 := position, tokenIndex
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l26
						}
						position++
						goto l25
					l26:
						position, tokenIndex = position25, tokenIndex25
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l22
						}
						position++
					}
				l25:
					goto l21
				l22:
					position, tokenIndex = position22, tokenIndex22
				}
				add(rulestring, position20)
			}
			return true
		l19:
			position, tokenIndex = position19, tokenIndex19
			return false
		},
		/* 4 ws <- <(' ' / '\t')*> */
		func() bool {
			{
				position28 := position
			l29:
				{
					position30, tokenIndex30 := position, tokenIndex
					{
						position31, tokenIndex31 := position, tokenIndex
						if buffer[position] != rune(' ') {
							goto l32
						}
						position++
						goto l31
					l32:
						position, tokenIndex = position31, tokenIndex31
						if buffer[position] != rune('\t') {
							goto l30
						}
						position++
					}
				l31:
					goto l29
				l30:
					position, tokenIndex = position30, tokenIndex30
				}
				add(rulews, position28)
			}
			return true
		},
	}
	p.rules = _rules
}
