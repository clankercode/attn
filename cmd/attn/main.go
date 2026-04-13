package main

import (
	"github.com/clankercode/attn/internal"
	"os"
)

func main() {
	internal.Run(os.Args[1:])
}
