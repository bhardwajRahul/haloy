package haloy

import (
	"fmt"
	"time"

	"github.com/haloydev/haloy/internal/ui"
	"github.com/spf13/cobra"
)

func ProgressDemoCmd() *cobra.Command {
	var (
		totalBytes int64 = 26_906_214
		stepBytes  int64 = 32 * 1024
		items            = 1
		width            = 0
		delay            = 25 * time.Millisecond
	)

	cmd := &cobra.Command{
		Use:    "__progress-demo",
		Short:  "Run the hidden progress renderer demo",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if totalBytes <= 0 {
				return fmt.Errorf("--total-bytes must be greater than 0")
			}
			if stepBytes <= 0 {
				return fmt.Errorf("--step-bytes must be greater than 0")
			}
			if items <= 0 {
				return fmt.Errorf("--items must be greater than 0")
			}
			if width < 0 {
				return fmt.Errorf("--width must be 0 or greater")
			}

			progress := ui.NewProgressBar(ui.ProgressBarConfig{
				Description: "Uploading layers",
				TotalBytes:  totalBytes,
				TotalItems:  items,
				ShowBytes:   true,
				TermWidth:   width,
			})
			defer progress.Finish()

			var uploaded int64
			for i := 0; i < items; i++ {
				target := totalBytes * int64(i+1) / int64(items)
				for uploaded < target {
					next := min(stepBytes, target-uploaded)
					progress.Add(next)
					uploaded += next
					if delay > 0 {
						time.Sleep(delay)
					}
				}
				progress.CompleteItem()
			}

			return nil
		},
	}

	cmd.Flags().Int64Var(&totalBytes, "total-bytes", totalBytes, "total bytes to simulate")
	cmd.Flags().Int64Var(&stepBytes, "step-bytes", stepBytes, "bytes to add per update")
	cmd.Flags().IntVar(&items, "items", items, "number of upload items to simulate")
	cmd.Flags().IntVar(&width, "width", width, "terminal width to simulate; 0 auto-detects")
	cmd.Flags().DurationVar(&delay, "delay", delay, "delay between updates")

	return cmd
}
