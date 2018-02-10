calc: str.peg.go main.go
	go build

str.peg.go: str.peg
	peg -switch -inline str.peg

clean:
	rm -f str str.peg.go