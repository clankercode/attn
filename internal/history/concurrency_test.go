package history

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Concurrent Record + Delete must not corrupt the JSONL file: after the
// storm, every remaining line must parse and Load must succeed.
func TestConcurrentRecordAndDelete(t *testing.T) {
	tmp := setupEnv(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				p := filepath.Join(tmp, ".tts-output", fmt.Sprintf("c%d-%d.mp3", i, j))
				e := Entry{Time: time.Now(), Text: "x", Provider: "grok", Voice: "eve", Path: p}
				if err := Record(e); err != nil {
					t.Errorf("Record: %v", err)
					return
				}
				if j%3 == 0 {
					if err := Delete(e); err != nil {
						t.Errorf("Delete: %v", err)
						return
					}
				}
			}
		}(i)
	}
	wg.Wait()

	entries, err := Load()
	if err != nil {
		t.Fatalf("Load after concurrency: %v", err)
	}
	// 80 records, 32 deletes (j=0,3,6,9 of each 10, x8) -> 48 expected.
	if len(entries) != 48 {
		t.Fatalf("expected 48 surviving entries, got %d", len(entries))
	}
}
