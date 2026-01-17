package loadtest

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"go.uber.org/zap"
)

func StartMultiLoadTest(logger *zap.SugaredLogger, host string, maxPads int) {
	if maxPads <= 0 {
		maxPads = 10
	}

	fmt.Printf("Starting multi-pad load test: %d pads for 30 seconds each\n", maxPads)

	executable, err := os.Executable()
	if err != nil {
		logger.Errorf("Failed to get executable path: %v", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	messageCount := 0

	for i := 0; i < maxPads; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cmd := exec.Command(executable, "loadtest", "-host", host, "-authors", "3", "-duration", "30")
			cmd.Env = append(os.Environ(), "SILENT_METRICS=true")

			output, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("Child process %d exited with error: %v\n", id, err)
				fmt.Printf("Output: %s\n", string(output))
				fmt.Println("total pads made:", id) // Approximation
				fmt.Println("total messages", messageCount)
				os.Exit(1)
			}
		}(i)

		time.Sleep(100 * time.Millisecond)
	}

	wg.Wait()
	fmt.Println("Multi-pad load test completed successfully")
}
