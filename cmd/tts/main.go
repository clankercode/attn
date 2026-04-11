package main

import (
	"attn-tool/internal"
	"os"
)

func main() {
	internal.Run(os.Args[1:])
}
