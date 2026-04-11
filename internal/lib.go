package internal

import (
	"attn-tool/internal/cli"
)

func Run(args []string) {
	cfg := cli.Parse(args)
	_ = cfg
}
