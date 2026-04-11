package internal

import (
	"os"
	"os/exec"
	"testing"
)

func TestRunDryRunDoesNotRequireAPIKey(t *testing.T) {
	if os.Getenv("ATTN_DRY_RUN_CHILD") == "1" {
		os.Unsetenv("GROQ_API_KEY")
		os.Unsetenv("MINIMAX_API_KEY")
		Run([]string{"--dry-run", "-o", "/tmp/attn-dry-run.mp3", "hello"})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestRunDryRunDoesNotRequireAPIKey")
	cmd.Env = append(os.Environ(), "ATTN_DRY_RUN_CHILD=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry run should not require provider credentials: %v\n%s", err, output)
	}
}
